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

package bufmodule

import (
	"context"
	"io/fs"
	"sort"

	"github.com/bufbuild/buf/private/bufpkg/bufcas"
	"github.com/bufbuild/buf/private/pkg/normalpath"
	"github.com/bufbuild/buf/private/pkg/slicesextended"
	"github.com/bufbuild/buf/private/pkg/storage"
	"go.uber.org/multierr"
)

// ModuleReadBucket is an object analogous to storage.ReadBucket that supplements ObjectInfos
// and Objects with the data on the Module that supplied them.
//
// ModuleReadBuckets talk in terms of Files and FileInfos. They are easily converted into
// storage.ReadBuckets.
//
// The contents of a ModuleReadBucket are specific to its context. In the context of a Module,
// a ModuleReadBucket will return .proto files, documentation file(s), and license file(s). However,
// in the context of converting a Workspace into its corresponding .proto files, a ModuleReadBucket
// will only contain .proto files.
type ModuleReadBucket interface {
	// GetFile gets the File within the Module as specified by the path.
	//
	// Returns an error with fs.ErrNotExist if the path is not part of the Module.
	GetFile(ctx context.Context, path string) (File, error)
	// StatFileInfo gets the FileInfo for the File within the Module as specified by the path.
	//
	// Returns an error with fs.ErrNotExist if the path is not part of the Module.
	StatFileInfo(ctx context.Context, path string) (FileInfo, error)
	// WalkFileInfos walks all Files in the Module, passing the FileInfo to a specified function.
	//
	// This will walk the .proto files, documentation file(s), and license files(s). This package
	// currently exposes functionality to walk just the .proto files, and get the singular
	// documentation and license files, via WalkProtoFileInfos, GetDocFile, and GetLicenseFile.
	//
	// GetDocFile and GetLicenseFile may change in the future if other paths are accepted for
	// documentation or licenses, or if we allow multiple documentation or license files to
	// exist within a Module (currently, only one of each is allowed).
	WalkFileInfos(ctx context.Context, f func(FileInfo) error, options ...WalkFileInfosOption) error

	isModuleReadBucket()
}

// WalkFileInfosOption is an option for WalkFileInfos
type WalkFileInfosOption func(*walkFileInfosOptions)

// WalkFileInfosWithOnlyTargetFiles returns a new WalkFileInfosOption that will result in only
// FileInfos with IsTargetFile() set to true being walked.
//
// Note that no Files from a Module will have IsTargetFile() set to true if
// Module.IsTargetModule() is false.
//
// If specific Files were not targeted but the Module was targeted, all Files will have
// IsTargetFile() set to true, and this function will return all Files that WalkFileInfos does.
func WalkFileInfosWithOnlyTargetFiles() WalkFileInfosOption {
	return func(walkFileInfosOptions *walkFileInfosOptions) {
		walkFileInfosOptions.onlyTargetFiles = true
	}
}

// ModuleReadBucketToStorageReadBucket converts the given ModuleReadBucket to a storage.ReadBucket.
//
// All target and non-target files are added.
func ModuleReadBucketToStorageReadBucket(bucket ModuleReadBucket) storage.ReadBucket {
	return newStorageReadBucket(bucket)
}

// ModuleReadBucketWithOnlyFileTypes returns a new ModuleReadBucket that only contains the given
// FileTypes.
//
// Common use case is to get only the .proto files.
func ModuleReadBucketWithOnlyFileTypes(
	moduleReadBucket ModuleReadBucket,
	fileTypes ...FileType,
) ModuleReadBucket {
	return newFilteredModuleReadBucket(moduleReadBucket, fileTypes)
}

// ModuleReadBucketWithOnlyProtoFiles is a convenience function that returns a new
// ModuleReadBucket that only contains the .proto files.
func ModuleReadBucketWithOnlyProtoFiles(moduleReadBucket ModuleReadBucket) ModuleReadBucket {
	return ModuleReadBucketWithOnlyFileTypes(moduleReadBucket, FileTypeProto)
}

// GetFileInfos is a convenience function that walks the ModuleReadBucket and gets
// all the FileInfos.
//
// Sorted by path.
func GetFileInfos(ctx context.Context, moduleReadBucket ModuleReadBucket) ([]FileInfo, error) {
	var fileInfos []FileInfo
	if err := moduleReadBucket.WalkFileInfos(
		ctx,
		func(fileInfo FileInfo) error {
			fileInfos = append(fileInfos, fileInfo)
			return nil
		},
	); err != nil {
		return nil, err
	}
	sort.Slice(
		fileInfos,
		func(i int, j int) bool {
			return fileInfos[i].Path() < fileInfos[j].Path()
		},
	)
	return fileInfos, nil
}

// GetTargetFileInfos is a convenience function that walks the ModuleReadBucket and gets
// all the FileInfos where IsTargetFile() is set to true.
//
// Sorted by path.
func GetTargetFileInfos(ctx context.Context, moduleReadBucket ModuleReadBucket) ([]FileInfo, error) {
	var fileInfos []FileInfo
	if err := moduleReadBucket.WalkFileInfos(
		ctx,
		func(fileInfo FileInfo) error {
			fileInfos = append(fileInfos, fileInfo)
			return nil
		},
		WalkFileInfosWithOnlyTargetFiles(),
	); err != nil {
		return nil, err
	}
	sort.Slice(
		fileInfos,
		func(i int, j int) bool {
			return fileInfos[i].Path() < fileInfos[j].Path()
		},
	)
	return fileInfos, nil
}

// GetDocFile gets the singular documentation File for the Module, if it exists.
//
// When creating a Module from a Bucket, we check the file paths buf.md, README.md, and README.markdown
// to exist, in that order. The first one to exist is chosen as the documentation File that is considered
// part of the Module, and any others are discarded. This function will return that File that was chosen.
//
// Returns an error with fs.ErrNotExist if no documentation file exists.
func GetDocFile(ctx context.Context, moduleReadBucket ModuleReadBucket) (File, error) {
	if docFilePath := getDocFilePathForModuleReadBucket(ctx, moduleReadBucket); docFilePath != "" {
		return moduleReadBucket.GetFile(ctx, docFilePath)
	}
	return nil, fs.ErrNotExist
}

// GetLicenseFile gets the license File for the Module, if it exists.
//
// Returns an error with fs.ErrNotExist if the license File does not exist.
func GetLicenseFile(ctx context.Context, moduleReadBucket ModuleReadBucket) (File, error) {
	return moduleReadBucket.GetFile(ctx, licenseFilePath)
}

// *** PRIVATE ***

// moduleReadBucket

type moduleReadBucket struct {
	delegate             storage.ReadBucket
	module               Module
	targetPathMap        map[string]struct{}
	targetExcludePathMap map[string]struct{}
}

// module cannot be assumed to be functional yet.
// Do not call any functions on module.
func newModuleReadBucket(
	ctx context.Context,
	delegate storage.ReadBucket,
	module Module,
	targetPaths []string,
	targetExcludePaths []string,
) *moduleReadBucket {
	docFilePath := getDocFilePathForStorageReadBucket(ctx, delegate)
	return &moduleReadBucket{
		delegate: storage.MapReadBucket(
			delegate,
			storage.MatchOr(
				storage.MatchPathExt(".proto"),
				storage.MatchPathEqual(licenseFilePath),
				storage.MatchPathEqual(docFilePath),
			),
		),
		module:               module,
		targetPathMap:        slicesextended.ToMap(targetPaths),
		targetExcludePathMap: slicesextended.ToMap(targetExcludePaths),
	}
}

func (f *moduleReadBucket) GetFile(ctx context.Context, path string) (File, error) {
	fileInfo, err := f.StatFileInfo(ctx, path)
	if err != nil {
		return nil, err
	}
	readObjectCloser, err := f.delegate.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	return newFile(fileInfo, readObjectCloser), nil
}

func (f *moduleReadBucket) StatFileInfo(ctx context.Context, path string) (FileInfo, error) {
	objectInfo, err := f.delegate.Stat(ctx, path)
	if err != nil {
		return nil, err
	}
	return f.newFileInfo(objectInfo)
}

func (f *moduleReadBucket) WalkFileInfos(
	ctx context.Context,
	fn func(FileInfo) error,
	options ...WalkFileInfosOption,
) error {
	walkFileInfosOptions := newWalkFileInfosOptions()
	for _, option := range options {
		option(walkFileInfosOptions)
	}
	// TODO: we need to special-case target paths to only walk on target paths
	// if len(f.targetPathMap) > 0, for performance reasons.
	//
	// This may mean we need to change storage.Walk to accept prefixes that are equal to the prefix.
	return f.delegate.Walk(
		ctx,
		"",
		func(objectInfo storage.ObjectInfo) error {
			fileInfo, err := f.newFileInfo(objectInfo)
			if err != nil {
				return err
			}
			if walkFileInfosOptions.onlyTargetFiles && !fileInfo.IsTargetFile() {
				return nil
			}
			return fn(fileInfo)
		},
	)
}

func (*moduleReadBucket) isModuleReadBucket() {}

func (b *moduleReadBucket) newFileInfo(objectInfo storage.ObjectInfo) (FileInfo, error) {
	fileType, err := classifyPathFileType(objectInfo.Path())
	if err != nil {
		// Given our matching in the constructor, all file paths should be classified.
		// A lack of classification is a system error.
		return nil, err
	}
	return newFileInfo(objectInfo, b.module, fileType, b.getIsTargetedFileForPath(objectInfo.Path())), nil
}

func (f *moduleReadBucket) getIsTargetedFileForPath(path string) bool {
	if !f.module.IsTargetModule() {
		// If the Module is not targeted, the file is automatically not targeted.
		//
		// Note we can change IsTargetModule via setIsTargetModule during ModuleSetBuilder building,
		// so we do not want to cache this value.
		return false
	}
	switch {
	case len(f.targetPathMap) == 0 && len(f.targetExcludePathMap) == 0:
		// If we did not target specific Files, all Files in a targeted Module are targeted.
		return true
	case len(f.targetPathMap) == 0 && len(f.targetExcludePathMap) != 0:
		// We only have exclude paths, no paths.
		return !normalpath.MapHasEqualOrContainingPath(f.targetExcludePathMap, path, normalpath.Relative)
	case len(f.targetPathMap) != 0 && len(f.targetExcludePathMap) == 0:
		// We only have paths, no exclude paths.
		return normalpath.MapHasEqualOrContainingPath(f.targetPathMap, path, normalpath.Relative)
	default:
		// We have both paths and exclude paths.
		return normalpath.MapHasEqualOrContainingPath(f.targetPathMap, path, normalpath.Relative) &&
			!normalpath.MapHasEqualOrContainingPath(f.targetExcludePathMap, path, normalpath.Relative)
	}
}

// filteredModuleReadBucket

type filteredModuleReadBucket struct {
	delegate    ModuleReadBucket
	fileTypeMap map[FileType]struct{}
}

func newFilteredModuleReadBucket(
	delegate ModuleReadBucket,
	fileTypes []FileType,
) *filteredModuleReadBucket {
	return &filteredModuleReadBucket{
		delegate:    delegate,
		fileTypeMap: fileTypeSliceToMap(fileTypes),
	}
}

func (f *filteredModuleReadBucket) GetFile(ctx context.Context, path string) (File, error) {
	// Stat'ing the filtered bucket, not the delegate.
	if _, err := f.StatFileInfo(ctx, path); err != nil {
		return nil, err
	}
	return f.delegate.GetFile(ctx, path)
}

func (f *filteredModuleReadBucket) StatFileInfo(ctx context.Context, path string) (FileInfo, error) {
	fileInfo, err := f.delegate.StatFileInfo(ctx, path)
	if err != nil {
		return nil, err
	}
	if _, ok := f.fileTypeMap[fileInfo.FileType()]; !ok {
		return nil, &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist}
	}
	return fileInfo, nil
}

func (f *filteredModuleReadBucket) WalkFileInfos(
	ctx context.Context,
	fn func(FileInfo) error,
	options ...WalkFileInfosOption,
) error {
	return f.delegate.WalkFileInfos(
		ctx,
		func(fileInfo FileInfo) error {
			if _, ok := f.fileTypeMap[fileInfo.FileType()]; !ok {
				return nil
			}
			return fn(fileInfo)
		},
		options...,
	)
}

func (*filteredModuleReadBucket) isModuleReadBucket() {}

// storageReadBucket

type storageReadBucket struct {
	delegate ModuleReadBucket
}

func newStorageReadBucket(delegate ModuleReadBucket) *storageReadBucket {
	return &storageReadBucket{
		delegate: delegate,
	}
}

func (s *storageReadBucket) Get(ctx context.Context, path string) (storage.ReadObjectCloser, error) {
	return s.delegate.GetFile(ctx, path)
}

func (s *storageReadBucket) Stat(ctx context.Context, path string) (storage.ObjectInfo, error) {
	return s.delegate.StatFileInfo(ctx, path)
}

func (s *storageReadBucket) Walk(ctx context.Context, prefix string, f func(storage.ObjectInfo) error) error {
	return s.delegate.WalkFileInfos(
		ctx,
		func(fileInfo FileInfo) error {
			if !normalpath.EqualsOrContainsPath(prefix, fileInfo.Path(), normalpath.Relative) {
				return nil
			}
			return f(fileInfo)
		},
	)
}

// walkFileInfosOptions

type walkFileInfosOptions struct {
	onlyTargetFiles bool
}

func newWalkFileInfosOptions() *walkFileInfosOptions {
	return &walkFileInfosOptions{}
}

// utils

func moduleReadBucketDigestB5(ctx context.Context, moduleReadBucket ModuleReadBucket) (bufcas.Digest, error) {
	var fileNodes []bufcas.FileNode
	if err := moduleReadBucket.WalkFileInfos(
		ctx,
		func(fileInfo FileInfo) (retErr error) {
			file, err := moduleReadBucket.GetFile(ctx, fileInfo.Path())
			if err != nil {
				return err
			}
			defer func() {
				retErr = multierr.Append(retErr, file.Close())
			}()
			// TODO: what about digest type?
			digest, err := bufcas.NewDigestForContent(file)
			if err != nil {
				return err
			}
			fileNode, err := bufcas.NewFileNode(fileInfo.Path(), digest)
			if err != nil {
				return err
			}
			fileNodes = append(fileNodes, fileNode)
			return nil
		},
	); err != nil {
		return nil, err
	}
	manifest, err := bufcas.NewManifest(fileNodes)
	if err != nil {
		return nil, err
	}
	manifestBlob, err := bufcas.ManifestToBlob(manifest)
	if err != nil {
		return nil, err
	}
	return manifestBlob.Digest(), nil
}
