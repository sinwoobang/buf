// Copyright 2020-2023 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bufimage

import (
	"context"
	"errors"
	"fmt"

	"github.com/bufbuild/buf/private/bufpkg/bufanalysis"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/pkg/normalpath"
	"github.com/bufbuild/buf/private/pkg/syserror"
	"github.com/bufbuild/buf/private/pkg/thread"
	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/protoutil"
	"github.com/bufbuild/protocompile/reporter"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const tracerName = "bufbuild/buf"

func buildImage(
	ctx context.Context,
	moduleReadBucket bufmodule.ModuleReadBucket,
	excludeSourceCodeInfo bool,
	noParallelism bool,
) (_ Image, _ []bufanalysis.FileAnnotation, retErr error) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "build_image")
	defer span.End()
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
	}()
	if !moduleReadBucket.ShouldBeSelfContained() {
		return nil, nil, syserror.New("passed a ModuleReadBucket to BuildImage that was not expected to be self-contained")
	}
	moduleReadBucket = bufmodule.ModuleReadBucketWithOnlyProtoFiles(moduleReadBucket)
	parserAccessorHandler := newParserAccessorHandler(ctx, moduleReadBucket)
	targetFileInfos, err := bufmodule.GetTargetFileInfos(ctx, moduleReadBucket)
	if err != nil {
		return nil, nil, err
	}
	if len(targetFileInfos) == 0 {
		return nil, nil, errors.New("no input files specified")
	}
	paths := bufmodule.FileInfoPaths(targetFileInfos)

	buildResult := getBuildResult(
		ctx,
		parserAccessorHandler,
		paths,
		excludeSourceCodeInfo,
		noParallelism,
	)
	if buildResult.Err != nil {
		return nil, nil, buildResult.Err
	}
	if len(buildResult.FileAnnotations) > 0 {
		return nil, bufanalysis.DeduplicateAndSortFileAnnotations(buildResult.FileAnnotations), nil
	}

	fileDescriptors, err := checkAndSortFileDescriptors(buildResult.FileDescriptors, paths)
	if err != nil {
		return nil, nil, err
	}
	image, err := getImage(
		ctx,
		excludeSourceCodeInfo,
		fileDescriptors,
		parserAccessorHandler,
		buildResult.SyntaxUnspecifiedFilenames,
		buildResult.FilenameToUnusedDependencyFilenames,
	)
	if err != nil {
		return nil, nil, err
	}
	return image, nil, nil
}

func getBuildResult(
	ctx context.Context,
	parserAccessorHandler *parserAccessorHandler,
	paths []string,
	excludeSourceCodeInfo bool,
	noParallelism bool,
) *buildResult {
	var errorsWithPos []reporter.ErrorWithPos
	var warningErrorsWithPos []reporter.ErrorWithPos
	sourceInfoMode := protocompile.SourceInfoStandard
	if excludeSourceCodeInfo {
		sourceInfoMode = protocompile.SourceInfoNone
	}
	parallelism := thread.Parallelism()
	if noParallelism {
		parallelism = 1
	}
	compiler := protocompile.Compiler{
		MaxParallelism: parallelism,
		SourceInfoMode: sourceInfoMode,
		Resolver:       &protocompile.SourceResolver{Accessor: parserAccessorHandler.Open},
		Reporter: reporter.NewReporter(
			func(errorWithPos reporter.ErrorWithPos) error {
				errorsWithPos = append(errorsWithPos, errorWithPos)
				return nil
			},
			func(errorWithPos reporter.ErrorWithPos) {
				warningErrorsWithPos = append(warningErrorsWithPos, errorWithPos)
			},
		),
	}
	// fileDescriptors are in the same order as paths per the documentation
	compiledFiles, err := compiler.Compile(ctx, paths...)
	if err != nil {
		if err == reporter.ErrInvalidSource {
			if len(errorsWithPos) == 0 {
				return newBuildResult(
					nil,
					nil,
					nil,
					nil,
					errors.New("got invalid source error from parse but no errors reported"),
				)
			}
			fileAnnotations, err := getFileAnnotations(
				ctx,
				parserAccessorHandler,
				errorsWithPos,
			)
			if err != nil {
				return newBuildResult(nil, nil, nil, nil, err)
			}
			return newBuildResult(nil, nil, nil, fileAnnotations, nil)
		}
		if errorWithPos, ok := err.(reporter.ErrorWithPos); ok {
			fileAnnotations, err := getFileAnnotations(
				ctx,
				parserAccessorHandler,
				[]reporter.ErrorWithPos{errorWithPos},
			)
			if err != nil {
				return newBuildResult(nil, nil, nil, nil, err)
			}
			return newBuildResult(nil, nil, nil, fileAnnotations, nil)
		}
		return newBuildResult(nil, nil, nil, nil, err)
	} else if len(errorsWithPos) > 0 {
		// https://github.com/jhump/protoreflect/pull/331
		return newBuildResult(
			nil,
			nil,
			nil,
			nil,
			errors.New("got no error from parse but errors reported"),
		)
	}
	if len(compiledFiles) != len(paths) {
		return newBuildResult(
			nil,
			nil,
			nil,
			nil,
			fmt.Errorf("expected FileDescriptors to be of length %d but was %d", len(paths), len(compiledFiles)),
		)
	}
	for i, fileDescriptor := range compiledFiles {
		path := paths[i]
		filename := fileDescriptor.Path()
		// doing another rough verification
		// NO LONGER NEED TO DO SUFFIX SINCE WE KNOW THE ROOT FILE NAME
		if path != filename {
			return newBuildResult(
				nil,
				nil,
				nil,
				nil,
				fmt.Errorf("expected fileDescriptor name %s to be a equal to %s", filename, path),
			)
		}
	}
	syntaxUnspecifiedFilenames := make(map[string]struct{})
	filenameToUnusedDependencyFilenames := make(map[string]map[string]struct{})
	for _, warningErrorWithPos := range warningErrorsWithPos {
		maybeAddSyntaxUnspecified(syntaxUnspecifiedFilenames, warningErrorWithPos)
		maybeAddUnusedImport(filenameToUnusedDependencyFilenames, warningErrorWithPos)
	}
	fileDescriptors := make([]protoreflect.FileDescriptor, len(compiledFiles))
	for i := range compiledFiles {
		fileDescriptors[i] = compiledFiles[i]
	}
	return newBuildResult(
		fileDescriptors,
		syntaxUnspecifiedFilenames,
		filenameToUnusedDependencyFilenames,
		nil,
		nil,
	)
}

// We need to sort the FileDescriptors as they may/probably are out of order
// relative to input order after concurrent builds. This mimics the output
// order of protoc.
func checkAndSortFileDescriptors(
	fileDescriptors []protoreflect.FileDescriptor,
	rootRelFilePaths []string,
) ([]protoreflect.FileDescriptor, error) {
	if len(fileDescriptors) != len(rootRelFilePaths) {
		return nil, fmt.Errorf("rootRelFilePath length was %d but FileDescriptor length was %d", len(rootRelFilePaths), len(fileDescriptors))
	}
	nameToFileDescriptor := make(map[string]protoreflect.FileDescriptor, len(fileDescriptors))
	for _, fileDescriptor := range fileDescriptors {
		name := fileDescriptor.Path()
		if name == "" {
			return nil, errors.New("no name on FileDescriptor")
		}
		if _, ok := nameToFileDescriptor[name]; ok {
			return nil, fmt.Errorf("duplicate FileDescriptor: %s", name)
		}
		nameToFileDescriptor[name] = fileDescriptor
	}
	// We now know that all FileDescriptors had unique names and the number of FileDescriptors
	// is equal to the number of rootRelFilePaths. We also verified earlier that rootRelFilePaths
	// has only unique values. Now we can put them in order.
	sortedFileDescriptors := make([]protoreflect.FileDescriptor, 0, len(fileDescriptors))
	for _, rootRelFilePath := range rootRelFilePaths {
		fileDescriptor, ok := nameToFileDescriptor[rootRelFilePath]
		if !ok {
			return nil, fmt.Errorf("no FileDescriptor for rootRelFilePath: %q", rootRelFilePath)
		}
		sortedFileDescriptors = append(sortedFileDescriptors, fileDescriptor)
	}
	return sortedFileDescriptors, nil
}

// getImage gets the Image for the protoreflect.FileDescriptors.
//
// This mimics protoc's output order.
// This assumes checkAndSortFileDescriptors was called.
func getImage(
	ctx context.Context,
	excludeSourceCodeInfo bool,
	sortedFileDescriptors []protoreflect.FileDescriptor,
	parserAccessorHandler *parserAccessorHandler,
	syntaxUnspecifiedFilenames map[string]struct{},
	filenameToUnusedDependencyFilenames map[string]map[string]struct{},
) (Image, error) {
	// if we aren't including imports, then we need a set of file names that
	// are included so we can create a topologically sorted list w/out
	// including imports that should not be present.
	//
	// if we are including imports, then we need to know what filenames
	// are imports are what filenames are not
	// all input protoreflect.FileDescriptors are not imports, we derive the imports
	// from GetDependencies.
	nonImportFilenames := map[string]struct{}{}
	for _, fileDescriptor := range sortedFileDescriptors {
		nonImportFilenames[fileDescriptor.Path()] = struct{}{}
	}

	var imageFiles []ImageFile
	var err error
	alreadySeen := map[string]struct{}{}
	for _, fileDescriptor := range sortedFileDescriptors {
		imageFiles, err = getImageFilesRec(
			ctx,
			excludeSourceCodeInfo,
			fileDescriptor,
			parserAccessorHandler,
			syntaxUnspecifiedFilenames,
			filenameToUnusedDependencyFilenames,
			alreadySeen,
			nonImportFilenames,
			imageFiles,
		)
		if err != nil {
			return nil, err
		}
	}
	return NewImage(imageFiles)
}

func getImageFilesRec(
	ctx context.Context,
	excludeSourceCodeInfo bool,
	fileDescriptor protoreflect.FileDescriptor,
	parserAccessorHandler *parserAccessorHandler,
	syntaxUnspecifiedFilenames map[string]struct{},
	filenameToUnusedDependencyFilenames map[string]map[string]struct{},
	alreadySeen map[string]struct{},
	nonImportFilenames map[string]struct{},
	imageFiles []ImageFile,
) ([]ImageFile, error) {
	if fileDescriptor == nil {
		return nil, errors.New("nil FileDescriptor")
	}
	path := fileDescriptor.Path()
	if _, ok := alreadySeen[path]; ok {
		return imageFiles, nil
	}
	alreadySeen[path] = struct{}{}

	unusedDependencyFilenames, ok := filenameToUnusedDependencyFilenames[path]
	var unusedDependencyIndexes []int32
	if ok {
		unusedDependencyIndexes = make([]int32, 0, len(unusedDependencyFilenames))
	}
	var err error
	for i := 0; i < fileDescriptor.Imports().Len(); i++ {
		dependency := fileDescriptor.Imports().Get(i).FileDescriptor
		if unusedDependencyFilenames != nil {
			if _, ok := unusedDependencyFilenames[dependency.Path()]; ok {
				unusedDependencyIndexes = append(
					unusedDependencyIndexes,
					int32(i),
				)
			}
		}
		imageFiles, err = getImageFilesRec(
			ctx,
			excludeSourceCodeInfo,
			dependency,
			parserAccessorHandler,
			syntaxUnspecifiedFilenames,
			filenameToUnusedDependencyFilenames,
			alreadySeen,
			nonImportFilenames,
			imageFiles,
		)
		if err != nil {
			return nil, err
		}
	}

	fileDescriptorProto := protoutil.ProtoFromFileDescriptor(fileDescriptor)
	if fileDescriptorProto == nil {
		return nil, errors.New("nil FileDescriptorProto")
	}
	if excludeSourceCodeInfo {
		// need to do this anyways as Parser does not respect this for FileDescriptorProtos
		fileDescriptorProto.SourceCodeInfo = nil
	}
	_, isNotImport := nonImportFilenames[path]
	_, syntaxUnspecified := syntaxUnspecifiedFilenames[path]
	imageFile, err := NewImageFile(
		fileDescriptorProto,
		parserAccessorHandler.ModuleFullName(path),
		parserAccessorHandler.CommitID(path),
		// if empty, defaults to path
		parserAccessorHandler.ExternalPath(path),
		!isNotImport,
		syntaxUnspecified,
		unusedDependencyIndexes,
	)
	if err != nil {
		return nil, err
	}
	return append(imageFiles, imageFile), nil
}

func maybeAddSyntaxUnspecified(
	syntaxUnspecifiedFilenames map[string]struct{},
	errorWithPos reporter.ErrorWithPos,
) {
	if errorWithPos.Unwrap() != parser.ErrNoSyntax {
		return
	}
	syntaxUnspecifiedFilenames[errorWithPos.GetPosition().Filename] = struct{}{}
}

func maybeAddUnusedImport(
	filenameToUnusedImportFilenames map[string]map[string]struct{},
	errorWithPos reporter.ErrorWithPos,
) {
	errorUnusedImport, ok := errorWithPos.Unwrap().(linker.ErrorUnusedImport)
	if !ok {
		return
	}
	pos := errorWithPos.GetPosition()
	unusedImportFilenames, ok := filenameToUnusedImportFilenames[pos.Filename]
	if !ok {
		unusedImportFilenames = make(map[string]struct{})
		filenameToUnusedImportFilenames[pos.Filename] = unusedImportFilenames
	}
	unusedImportFilenames[errorUnusedImport.UnusedImport()] = struct{}{}
}

// getFileAnnotations gets the FileAnnotations for the ErrorWithPos errors.
func getFileAnnotations(
	ctx context.Context,
	parserAccessorHandler *parserAccessorHandler,
	errorsWithPos []reporter.ErrorWithPos,
) ([]bufanalysis.FileAnnotation, error) {
	fileAnnotations := make([]bufanalysis.FileAnnotation, 0, len(errorsWithPos))
	for _, errorWithPos := range errorsWithPos {
		fileAnnotation, err := getFileAnnotation(
			ctx,
			parserAccessorHandler,
			errorWithPos,
		)
		if err != nil {
			return nil, err
		}
		fileAnnotations = append(fileAnnotations, fileAnnotation)
	}
	return fileAnnotations, nil
}

// getFileAnnotation gets the FileAnnotation for the ErrorWithPos error.
func getFileAnnotation(
	ctx context.Context,
	parserAccessorHandler *parserAccessorHandler,
	errorWithPos reporter.ErrorWithPos,
) (bufanalysis.FileAnnotation, error) {
	var fileInfo bufanalysis.FileInfo
	var startLine int
	var startColumn int
	var endLine int
	var endColumn int
	typeString := "COMPILE"
	message := "Compile error."
	// this should never happen
	// maybe we should error
	if errorWithPos.Unwrap() != nil {
		message = errorWithPos.Unwrap().Error()
	}
	sourcePos := errorWithPos.GetPosition()
	if sourcePos.Filename != "" {
		path, err := normalpath.NormalizeAndValidate(sourcePos.Filename)
		if err != nil {
			return nil, err
		}
		fileInfo = newFileInfo(
			path,
			parserAccessorHandler.ExternalPath(path),
		)
	}
	if sourcePos.Line > 0 {
		startLine = sourcePos.Line
		endLine = sourcePos.Line
	}
	if sourcePos.Col > 0 {
		startColumn = sourcePos.Col
		endColumn = sourcePos.Col
	}
	return bufanalysis.NewFileAnnotation(
		fileInfo,
		startLine,
		startColumn,
		endLine,
		endColumn,
		typeString,
		message,
	), nil
}

type buildResult struct {
	FileDescriptors                     []protoreflect.FileDescriptor
	SyntaxUnspecifiedFilenames          map[string]struct{}
	FilenameToUnusedDependencyFilenames map[string]map[string]struct{}
	FileAnnotations                     []bufanalysis.FileAnnotation
	Err                                 error
}

func newBuildResult(
	fileDescriptors []protoreflect.FileDescriptor,
	syntaxUnspecifiedFilenames map[string]struct{},
	filenameToUnusedDependencyFilenames map[string]map[string]struct{},
	fileAnnotations []bufanalysis.FileAnnotation,
	err error,
) *buildResult {
	return &buildResult{
		FileDescriptors:                     fileDescriptors,
		SyntaxUnspecifiedFilenames:          syntaxUnspecifiedFilenames,
		FilenameToUnusedDependencyFilenames: filenameToUnusedDependencyFilenames,
		FileAnnotations:                     fileAnnotations,
		Err:                                 err,
	}
}

type fileInfo struct {
	path         string
	externalPath string
}

func newFileInfo(path string, externalPath string) *fileInfo {
	return &fileInfo{
		path:         path,
		externalPath: externalPath,
	}
}

func (f *fileInfo) Path() string {
	return f.path
}

func (f *fileInfo) ExternalPath() string {
	return f.externalPath
}

type buildImageOptions struct {
	excludeSourceCodeInfo bool
	noParallelism         bool
}

func newBuildImageOptions() *buildImageOptions {
	return &buildImageOptions{}
}
