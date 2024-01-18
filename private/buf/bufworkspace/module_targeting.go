// Copyright 2020-2024 Buf Technologies, Inc.
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

package bufworkspace

import (
	"fmt"

	"github.com/bufbuild/buf/private/pkg/normalpath"
	"github.com/bufbuild/buf/private/pkg/slicesext"
)

// TODO: All the module_bucket_builder_test.go stuff needs to be copied over

type moduleTargeting struct {
	// Whether this module is really a target module.
	//
	// False if this was not specified as a target module by the caller.
	// Also false if there were config.targetPaths or config.protoFileTargetPath, but
	// these paths did not match anything in the module.
	isTargetModule bool
	// relative to the actual moduleDirPath and the roots parsed from the buf.yaml
	moduleTargetPaths []string
	// relative to the actual moduleDirPath and the roots parsed from the buf.yaml
	moduleTargetExcludePaths []string
	// relative to the actual moduleDirPath and the roots parsed from the buf.yaml
	moduleProtoFileTargetPath string
	includePackageFiles       bool
}

func newModuleTargeting(
	moduleDirPath string,
	roots []string,
	config *workspaceBucketConfig,
	isTentativelyTargetModule bool,
) (*moduleTargeting, error) {
	if !isTentativelyTargetModule {
		// If this is not a target Module, we do not want to target anything, as targeting
		// paths for non-target Modules is an error.
		return &moduleTargeting{}, nil
	}
	// If we have no target paths, then we always match the value of isTargetModule.
	// Otherwise, we need to see that at least one path matches the moduleDirPath for us
	// to consider this module a target.
	isTargetModule := len(config.targetPaths) == 0 && config.protoFileTargetPath == ""
	var moduleTargetPaths []string
	var moduleTargetExcludePaths []string
	for _, targetPath := range config.targetPaths {
		if targetPath == moduleDirPath {
			// We're just going to be realists in our error messages here.
			// TODO: Do we error here currently? If so, this error remains. For extra credit in the future,
			// if we were really clever, we'd go back and just add this as a module path.
			return nil, fmt.Errorf("module %q was specified with --path - specify this module path directly as an input", targetPath)
		}
		if normalpath.ContainsPath(moduleDirPath, targetPath, normalpath.Relative) {
			isTargetModule = true
			moduleTargetPath, err := normalpath.Rel(moduleDirPath, targetPath)
			if err != nil {
				return nil, err
			}
			moduleTargetPaths = append(moduleTargetPaths, moduleTargetPath)
		}
	}
	for _, targetExcludePath := range config.targetExcludePaths {
		if targetExcludePath == moduleDirPath {
			// We're just going to be realists in our error messages here.
			// TODO: Do we error here currently? If so, this error remains. For extra credit in the future,
			// if we were really clever, we'd go back and just remove this as a module path if it was specified.
			// This really should be allowed - how else do you exclude from a workspace?
			return nil, fmt.Errorf("module %q was specified with --exclude-path - this flag cannot be used to specify module directories", targetExcludePath)
		}
		if normalpath.ContainsPath(moduleDirPath, targetExcludePath, normalpath.Relative) {
			moduleTargetExcludePath, err := normalpath.Rel(moduleDirPath, targetExcludePath)
			if err != nil {
				return nil, err
			}
			moduleTargetExcludePaths = append(moduleTargetExcludePaths, moduleTargetExcludePath)
		}
	}
	moduleTargetPaths, err := slicesext.MapError(
		moduleTargetPaths,
		func(moduleTargetPath string) (string, error) {
			return applyRootsToTargetPath(roots, moduleTargetPath, normalpath.Relative)
		},
	)
	if err != nil {
		return nil, err
	}
	moduleTargetExcludePaths, err = slicesext.MapError(
		moduleTargetExcludePaths,
		func(moduleTargetExcludePath string) (string, error) {
			return applyRootsToTargetPath(roots, moduleTargetExcludePath, normalpath.Relative)
		},
	)
	if err != nil {
		return nil, err
	}
	var moduleProtoFileTargetPath string
	var includePackageFiles bool
	if config.protoFileTargetPath != "" &&
		normalpath.ContainsPath(moduleDirPath, config.protoFileTargetPath, normalpath.Relative) {
		isTargetModule = true
		moduleProtoFileTargetPath, err = normalpath.Rel(moduleDirPath, config.protoFileTargetPath)
		if err != nil {
			return nil, err
		}
		moduleProtoFileTargetPath, err = applyRootsToTargetPath(roots, moduleProtoFileTargetPath, normalpath.Relative)
		if err != nil {
			return nil, err
		}
		includePackageFiles = config.includePackageFiles
	}
	return &moduleTargeting{
		isTargetModule:            isTargetModule,
		moduleTargetPaths:         moduleTargetPaths,
		moduleTargetExcludePaths:  moduleTargetExcludePaths,
		moduleProtoFileTargetPath: moduleProtoFileTargetPath,
		includePackageFiles:       includePackageFiles,
	}, nil
}
