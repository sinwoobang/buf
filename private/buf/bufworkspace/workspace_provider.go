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
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/bufbuild/buf/private/bufpkg/bufconfig"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/pkg/normalpath"
	"github.com/bufbuild/buf/private/pkg/slicesext"
	"github.com/bufbuild/buf/private/pkg/storage"
	"github.com/bufbuild/buf/private/pkg/syserror"
	"github.com/bufbuild/buf/private/pkg/tracing"
	"github.com/gofrs/uuid/v5"
	"go.uber.org/zap"
)

// WorkspaceProvider provides Workspaces and UpdateableWorkspaces.
type WorkspaceProvider interface {
	// GetWorkspaceForBucket returns a new Workspace for the given Bucket.
	//
	// If the underlying bucket has a v2 buf.yaml at the root, this builds a Workspace for this buf.yaml,
	// using TargetSubDirPath for targeting.
	//
	// If the underlying bucket has a buf.work.yaml at the root, this builds a Workspace with all the modules
	// specified in the buf.work.yaml, using TargetSubDirPath for targeting.
	//
	// Otherwise, this builds a Workspace with a single module at the TargetSubDirPath (which may be "."),
	// assuming v1 defaults.
	//
	// If a config override is specified, all buf.work.yamls are ignored. If the config override is v1,
	// this builds a single module at the TargetSubDirPath, if the config override is v2, the builds
	// at the root, using TargetSubDirPath for targeting.
	//
	// All parsing of configuration files is done behind the scenes here.
	GetWorkspaceForBucket(
		ctx context.Context,
		bucket storage.ReadBucket,
		options ...WorkspaceBucketOption,
	) (Workspace, error)

	// GetWorkspaceForModuleKey wraps the ModuleKey into a workspace, returning defaults
	// for config values, and empty ConfiguredDepModuleRefs.
	//
	// This is useful for getting Workspaces for remote modules, but you still need
	// associated configuration.
	GetWorkspaceForModuleKey(
		ctx context.Context,
		moduleKey bufmodule.ModuleKey,
		options ...WorkspaceModuleKeyOption,
	) (Workspace, error)
}

// NewWorkspaceProvider returns a new WorkspaceProvider.
func NewWorkspaceProvider(
	logger *zap.Logger,
	tracer tracing.Tracer,
	graphProvider bufmodule.GraphProvider,
	moduleDataProvider bufmodule.ModuleDataProvider,
	commitProvider bufmodule.CommitProvider,
) WorkspaceProvider {
	return newWorkspaceProvider(
		logger,
		tracer,
		graphProvider,
		moduleDataProvider,
		commitProvider,
	)
}

// *** PRIVATE ***

type workspaceProvider struct {
	logger             *zap.Logger
	tracer             tracing.Tracer
	graphProvider      bufmodule.GraphProvider
	moduleDataProvider bufmodule.ModuleDataProvider
	commitProvider     bufmodule.CommitProvider
}

func newWorkspaceProvider(
	logger *zap.Logger,
	tracer tracing.Tracer,
	graphProvider bufmodule.GraphProvider,
	moduleDataProvider bufmodule.ModuleDataProvider,
	commitProvider bufmodule.CommitProvider,
) *workspaceProvider {
	return &workspaceProvider{
		logger:             logger,
		tracer:             tracer,
		graphProvider:      graphProvider,
		moduleDataProvider: moduleDataProvider,
		commitProvider:     commitProvider,
	}
}

func (w *workspaceProvider) GetWorkspaceForModuleKey(
	ctx context.Context,
	moduleKey bufmodule.ModuleKey,
	options ...WorkspaceModuleKeyOption,
) (_ Workspace, retErr error) {
	ctx, span := w.tracer.Start(ctx, tracing.WithErr(&retErr))
	defer span.End()

	config, err := newWorkspaceModuleKeyConfig(options)
	if err != nil {
		return nil, err
	}
	// By default, the assocated configuration for a Module gotten by ModuleKey is just
	// the default config. However, if we have a config override, we may have different
	// lint or breaking config. We will only apply this different config for the specific
	// module we are targeting, while the rest will retain the default config - generally,
	// you shouldn't be linting or doing breaking change detection for any module other
	// than the one your are targeting (which matches v1 behavior as well). In v1, we didn't
	// have a "workspace" for modules gotten by module reference, we just had the single
	// module we were building against, and whatever config override we had only applied
	// to that module. In v2, we have a ModuleSet, and we need lint and breaking config
	// for every modules in the ModuleSet, so we attach default lint and breaking config,
	// but given the presence of ignore_only, we don't want to apply configOverride to
	// non-target modules as the config override might have file paths, and we won't
	// lint or breaking change detect against non-target modules anyways.
	targetModuleConfig := bufconfig.DefaultModuleConfig
	if config.configOverride != "" {
		bufYAMLFile, err := bufconfig.GetBufYAMLFileForOverride(config.configOverride)
		if err != nil {
			return nil, err
		}
		moduleConfigs := bufYAMLFile.ModuleConfigs()
		switch len(moduleConfigs) {
		case 0:
			return nil, syserror.New("had BufYAMLFile with 0 ModuleConfigs")
		case 1:
			// If we have a single ModuleConfig, we assume that regardless of whether or not
			// This ModuleConfig has a name, that this is what the user intends to associate
			// with the tqrget module. This also handles the v1 case - v1 buf.yamls will always
			// only have a single ModuleConfig, and it was expected pre-refactor that regardless
			// of if the ModuleConfig had a name associated with it or not, the lint and breaking
			// config that came from it would be associated.
			targetModuleConfig = moduleConfigs[0]
		default:
			// If we have more than one ModuleConfig, find the ModuleConfig that matches the
			// name from the ModuleKey. If none is found, just fall back to the default (ie do nothing here).
			for _, moduleConfig := range moduleConfigs {
				moduleFullName := moduleConfig.ModuleFullName()
				if moduleFullName == nil {
					continue
				}
				if bufmodule.ModuleFullNameEqual(moduleFullName, moduleKey.ModuleFullName()) {
					targetModuleConfig = moduleConfig
					// We know that the ModuleConfigs are unique by ModuleFullName.
					break
				}
			}
		}
	}

	moduleSet, err := bufmodule.NewModuleSetForRemoteModule(
		ctx,
		w.tracer,
		w.graphProvider,
		w.moduleDataProvider,
		w.commitProvider,
		moduleKey,
		bufmodule.RemoteModuleWithTargetPaths(
			config.targetPaths,
			config.targetExcludePaths,
		),
	)
	if err != nil {
		return nil, err
	}

	opaqueIDToLintConfig := make(map[string]bufconfig.LintConfig)
	opaqueIDToBreakingConfig := make(map[string]bufconfig.BreakingConfig)
	for _, module := range moduleSet.Modules() {
		if bufmodule.ModuleFullNameEqual(module.ModuleFullName(), moduleKey.ModuleFullName()) {
			// Set the lint and breaking config for the single targeted Module.
			opaqueIDToLintConfig[module.OpaqueID()] = targetModuleConfig.LintConfig()
			opaqueIDToBreakingConfig[module.OpaqueID()] = targetModuleConfig.BreakingConfig()
		} else {
			// For all non-targets, set the default lint and breaking config.
			opaqueIDToLintConfig[module.OpaqueID()] = bufconfig.DefaultLintConfig
			opaqueIDToBreakingConfig[module.OpaqueID()] = bufconfig.DefaultBreakingConfig
		}
	}
	return newWorkspace(
		moduleSet,
		opaqueIDToLintConfig,
		opaqueIDToBreakingConfig,
		nil,
		false,
	), nil
}

func (w *workspaceProvider) GetWorkspaceForBucket(
	ctx context.Context,
	bucket storage.ReadBucket,
	options ...WorkspaceBucketOption,
) (_ Workspace, retErr error) {
	ctx, span := w.tracer.Start(ctx, tracing.WithErr(&retErr))
	defer span.End()

	config, err := newWorkspaceBucketConfig(options)
	if err != nil {
		return nil, err
	}
	var overrideBufYAMLFile bufconfig.BufYAMLFile
	if config.configOverride != "" {
		overrideBufYAMLFile, err = bufconfig.GetBufYAMLFileForOverride(config.configOverride)
		if err != nil {
			return nil, err
		}
	}
	workspaceTargeting, err := newWorkspaceTargeting(
		ctx,
		w.logger,
		bucket,
		config.targetSubDirPath,
		overrideBufYAMLFile,
		config.ignoreAndDisallowV1BufWorkYAMLs,
	)
	if err != nil {
		return nil, err
	}
	if workspaceTargeting.isV2() {
		return w.getWorkspaceForBucketBufYAMLV2(
			ctx,
			bucket,
			config,
			workspaceTargeting.v2DirPath,
			overrideBufYAMLFile,
		)
	}
	return w.getWorkspaceForBucketAndModuleDirPathsV1Beta1OrV1(
		ctx,
		bucket,
		config,
		workspaceTargeting.v1DirPaths,
		overrideBufYAMLFile,
	)
}

func (w *workspaceProvider) getWorkspaceForBucketAndModuleDirPathsV1Beta1OrV1(
	ctx context.Context,
	bucket storage.ReadBucket,
	config *workspaceBucketConfig,
	moduleDirPaths []string,
	// This can be nil, this is only set if config.configOverride was set, which we
	// deal with outside of this function.
	overrideBufYAMLFile bufconfig.BufYAMLFile,
) (*workspace, error) {
	// config.targetSubDirPath is the input subDirPath. We only want to target modules that are inside
	// this subDirPath. Example: bufWorkYAMLDirPath is "foo", subDirPath is "foo/bar",
	// listed directories are "bar/baz", "bar/bat", "other". We want to include "foo/bar/baz"
	// and "foo/bar/bat".
	//
	// This is new behavior - before, we required that you input an exact match for the module directory path,
	// but now, we will take all the modules underneath this workspace.
	isTargetFunc := func(moduleDirPath string) bool {
		return normalpath.EqualsOrContainsPath(config.targetSubDirPath, moduleDirPath, normalpath.Relative)
	}
	moduleSetBuilder := bufmodule.NewModuleSetBuilder(ctx, w.tracer, w.moduleDataProvider, w.commitProvider)
	bucketIDToModuleConfig := make(map[string]bufconfig.ModuleConfig)
	// We use this to detect different refs across different files.
	moduleFullNameStringToConfiguredDepModuleRefString := make(map[string]string)
	var allConfiguredDepModuleRefs []bufmodule.ModuleRef
	// We keep track of if any module was tentatively targeted, and then actually targeted via
	// the paths flags. We use this pre-building of the ModuleSet to see if the --path and
	// --exclude-path flags resulted in no targeted modules. This condition is represented
	// by hadIsTentativelyTargetModule == true && hadIsTargetModule = false
	//
	// If hadIsTentativelyTargetModule is false, this means that our input subDirPath was not
	// actually representative of any module that we detected in buf.work.yaml or v2 buf.yaml
	// directories, and this is a system error - this should be verified before we reach this function.
	var hadIsTentativelyTargetModule bool
	var hadIsTargetModule bool
	for _, moduleDirPath := range moduleDirPaths {
		moduleConfig, configuredDepModuleRefs, err := getModuleConfigAndConfiguredDepModuleRefsV1Beta1OrV1(
			ctx,
			bucket,
			moduleDirPath,
			overrideBufYAMLFile,
		)
		if err != nil {
			return nil, err
		}
		for _, configuredDepModuleRef := range configuredDepModuleRefs {
			moduleFullNameString := configuredDepModuleRef.ModuleFullName().String()
			configuredDepModuleRefString := configuredDepModuleRef.String()
			existingConfiguredDepModuleRefString, ok := moduleFullNameStringToConfiguredDepModuleRefString[moduleFullNameString]
			if !ok {
				// We haven't encountered a ModuleRef with this ModuleFullName yet, add it.
				allConfiguredDepModuleRefs = append(allConfiguredDepModuleRefs, configuredDepModuleRef)
				moduleFullNameStringToConfiguredDepModuleRefString[moduleFullNameString] = configuredDepModuleRefString
			} else if configuredDepModuleRefString != existingConfiguredDepModuleRefString {
				// We encountered the same ModuleRef by ModuleFullName, but with a different Ref.
				return nil, fmt.Errorf("found different refs for the same module within buf.yaml deps in the workspace: %s %s", configuredDepModuleRefString, existingConfiguredDepModuleRefString)
			}
		}
		bucketIDToModuleConfig[moduleDirPath] = moduleConfig
		bufLockFile, err := bufconfig.GetBufLockFileForPrefix(
			ctx,
			bucket,
			// buf.lock files live at the module root
			moduleDirPath,
			bufconfig.BufLockFileWithDigestResolver(
				func(ctx context.Context, remote string, commitID uuid.UUID) (bufmodule.Digest, error) {
					commitKey, err := bufmodule.NewCommitKey(remote, commitID, bufmodule.DigestTypeB4)
					if err != nil {
						return nil, err
					}
					commits, err := w.commitProvider.GetCommitsForCommitKeys(ctx, []bufmodule.CommitKey{commitKey})
					if err != nil {
						return nil, err
					}
					return commits[0].ModuleKey().Digest()
				},
			),
		)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		} else {
			switch fileVersion := bufLockFile.FileVersion(); fileVersion {
			case bufconfig.FileVersionV1Beta1, bufconfig.FileVersionV1:
			case bufconfig.FileVersionV2:
			// TODO: re-enable once we fix tests
			//return nil, errors.New("got a v2 buf.lock file for a v1 buf.yaml")
			default:
				return nil, syserror.Newf("unknown FileVersion: %v", fileVersion)
			}
			for _, depModuleKey := range bufLockFile.DepModuleKeys() {
				// DepModuleKeys from a BufLockFile is expected to have all transitive dependencies,
				// and we can rely on this property.
				moduleSetBuilder.AddRemoteModule(
					depModuleKey,
					false,
				)
			}
		}
		// We figure out based on the paths if this is really a target module in moduleTargeting.
		isTentativelyTargetModule := isTargetFunc(moduleDirPath)
		if isTentativelyTargetModule {
			hadIsTentativelyTargetModule = true
		}
		mappedModuleBucket, moduleTargeting, err := getMappedModuleBucketAndModuleTargeting(
			ctx,
			bucket,
			config,
			moduleDirPath,
			moduleConfig,
			isTentativelyTargetModule,
		)
		if err != nil {
			return nil, err
		}
		if moduleTargeting.isTargetModule {
			hadIsTargetModule = true
		}
		v1BufYAMLObjectData, err := bufconfig.GetBufYAMLV1Beta1OrV1ObjectDataForPrefix(ctx, bucket, moduleDirPath)
		if err != nil {
			return nil, err
		}
		v1BufLockObjectData, err := bufconfig.GetBufLockV1Beta1OrV1ObjectDataForPrefix(ctx, bucket, moduleDirPath)
		if err != nil {
			return nil, err
		}
		moduleSetBuilder.AddLocalModule(
			mappedModuleBucket,
			moduleDirPath,
			moduleTargeting.isTargetModule,
			bufmodule.LocalModuleWithModuleFullName(moduleConfig.ModuleFullName()),
			bufmodule.LocalModuleWithTargetPaths(
				moduleTargeting.moduleTargetPaths,
				moduleTargeting.moduleTargetExcludePaths,
			),
			bufmodule.LocalModuleWithProtoFileTargetPath(
				moduleTargeting.moduleProtoFileTargetPath,
				moduleTargeting.includePackageFiles,
			),
			bufmodule.LocalModuleWithV1Beta1OrV1BufYAMLObjectData(v1BufYAMLObjectData),
			bufmodule.LocalModuleWithV1Beta1OrV1BufLockObjectData(v1BufLockObjectData),
		)
	}
	if !hadIsTentativelyTargetModule {
		return nil, syserror.Newf("subDirPath %q did not result in any target modules from moduleDirPaths %v", config.targetSubDirPath, moduleDirPaths)
	}
	if !hadIsTargetModule {
		// It would be nice to have a better error message than this in the long term.
		return nil, bufmodule.ErrNoTargetProtoFiles
	}
	moduleSet, err := moduleSetBuilder.Build()
	if err != nil {
		return nil, err
	}
	return w.getWorkspaceForBucketModuleSet(
		moduleSet,
		bucketIDToModuleConfig,
		allConfiguredDepModuleRefs,
		false,
	)
}

func (w *workspaceProvider) getWorkspaceForBucketBufYAMLV2(
	ctx context.Context,
	bucket storage.ReadBucket,
	config *workspaceBucketConfig,
	dirPath string,
	// This can be nil, this is only set if config.configOverride was set, which we
	// deal with outside of this function.
	overrideBufYAMLFile bufconfig.BufYAMLFile,
) (*workspace, error) {
	bucket = storage.MapReadBucket(bucket, storage.MapOnPrefix(dirPath))

	var bufYAMLFile bufconfig.BufYAMLFile
	var err error
	if overrideBufYAMLFile != nil {
		bufYAMLFile = overrideBufYAMLFile
		// We don't want to have ObjectData for a --config override.
		// TODO: What happened when you specified a --config pre-refactor with tamper-proofing? We might
		// have actually still used the buf.yaml for tamper-proofing, if so, we need to attempt to read it
		// regardless of whether override was specified.
	} else {
		bufYAMLFile, err = bufconfig.GetBufYAMLFileForPrefix(ctx, bucket, ".")
		if err != nil {
			// This should be apparent from above functions.
			return nil, syserror.Newf("error getting v2 buf.yaml: %w", err)
		}
		if bufYAMLFile.FileVersion() != bufconfig.FileVersionV2 {
			return nil, syserror.Newf("expected v2 buf.yaml but got %v", bufYAMLFile.FileVersion())
		}
	}

	// config.targetSubDirPath is the input targetSubDirPath. We only want to target modules that are inside
	// this targetSubDirPath. Example: bufWorkYAMLDirPath is "foo", targetSubDirPath is "foo/bar",
	// listed directories are "bar/baz", "bar/bat", "other". We want to include "foo/bar/baz"
	// and "foo/bar/bat".
	//
	// This is new behavior - before, we required that you input an exact match for the module directory path,
	// but now, we will take all the modules underneath this workspace.
	isTargetFunc := func(moduleDirPath string) bool {
		return normalpath.EqualsOrContainsPath(config.targetSubDirPath, moduleDirPath, normalpath.Relative)
	}
	moduleSetBuilder := bufmodule.NewModuleSetBuilder(ctx, w.tracer, w.moduleDataProvider, w.commitProvider)

	bufLockFile, err := bufconfig.GetBufLockFileForPrefix(
		ctx,
		bucket,
		// buf.lock files live next to the buf.yaml
		".",
		// We are not passing BufLockFileWithDigestResolver here because a buf.lock
		// v2 is expected to have digests
	)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	} else {
		switch fileVersion := bufLockFile.FileVersion(); fileVersion {
		case bufconfig.FileVersionV1Beta1, bufconfig.FileVersionV1:
			return nil, fmt.Errorf("got a %s buf.lock file for a v2 buf.yaml", bufLockFile.FileVersion().String())
		case bufconfig.FileVersionV2:
		default:
			return nil, syserror.Newf("unknown FileVersion: %v", fileVersion)
		}
		for _, depModuleKey := range bufLockFile.DepModuleKeys() {
			// DepModuleKeys from a BufLockFile is expected to have all transitive dependencies,
			// and we can rely on this property.
			moduleSetBuilder.AddRemoteModule(
				depModuleKey,
				false,
			)
		}
	}

	bucketIDToModuleConfig := make(map[string]bufconfig.ModuleConfig)
	// We keep track of if any module was tentatively targeted, and then actually targeted via
	// the paths flags. We use this pre-building of the ModuleSet to see if the --path and
	// --exclude-path flags resulted in no targeted modules. This condition is represented
	// by hadIsTentativelyTargetModule == true && hadIsTargetModule = false
	//
	// If hadIsTentativelyTargetModule is false, this means that our input subDirPath was not
	// actually representative of any module that we detected in buf.work.yaml or v2 buf.yaml
	// directories, and this is a system error - this should be verified before we reach this function.
	var hadIsTentativelyTargetModule bool
	var hadIsTargetModule bool
	var moduleDirPaths []string
	for _, moduleConfig := range bufYAMLFile.ModuleConfigs() {
		moduleDirPath := moduleConfig.DirPath()
		moduleDirPaths = append(moduleDirPaths, moduleDirPath)
		bucketIDToModuleConfig[moduleDirPath] = moduleConfig
		// We figure out based on the paths if this is really a target module in moduleTargeting.
		isTentativelyTargetModule := isTargetFunc(moduleDirPath)
		if isTentativelyTargetModule {
			hadIsTentativelyTargetModule = true
		}
		mappedModuleBucket, moduleTargeting, err := getMappedModuleBucketAndModuleTargeting(
			ctx,
			bucket,
			config,
			moduleDirPath,
			moduleConfig,
			isTentativelyTargetModule,
		)
		if err != nil {
			return nil, err
		}
		if moduleTargeting.isTargetModule {
			hadIsTargetModule = true
		}
		moduleSetBuilder.AddLocalModule(
			mappedModuleBucket,
			moduleDirPath,
			moduleTargeting.isTargetModule,
			bufmodule.LocalModuleWithModuleFullName(moduleConfig.ModuleFullName()),
			bufmodule.LocalModuleWithTargetPaths(
				moduleTargeting.moduleTargetPaths,
				moduleTargeting.moduleTargetExcludePaths,
			),
			bufmodule.LocalModuleWithProtoFileTargetPath(
				moduleTargeting.moduleProtoFileTargetPath,
				moduleTargeting.includePackageFiles,
			),
		)
	}
	if !hadIsTentativelyTargetModule {
		return nil, syserror.Newf("targetSubDirPath %q did not result in any target modules from moduleDirPaths %v", config.targetSubDirPath, moduleDirPaths)
	}
	if !hadIsTargetModule {
		// It would be nice to have a better error message than this in the long term.
		return nil, bufmodule.ErrNoTargetProtoFiles
	}
	moduleSet, err := moduleSetBuilder.Build()
	if err != nil {
		return nil, err
	}
	// bufYAMLFile.ConfiguredDepModuleRefs() is unique by ModuleFullName.
	return w.getWorkspaceForBucketModuleSet(
		moduleSet,
		bucketIDToModuleConfig,
		bufYAMLFile.ConfiguredDepModuleRefs(),
		true,
	)
}

// only use for workspaces created from buckets
func (w *workspaceProvider) getWorkspaceForBucketModuleSet(
	moduleSet bufmodule.ModuleSet,
	bucketIDToModuleConfig map[string]bufconfig.ModuleConfig,
	// Expected to already be unique by ModuleFullName.
	configuredDepModuleRefs []bufmodule.ModuleRef,
	isV2 bool,
) (*workspace, error) {
	opaqueIDToLintConfig := make(map[string]bufconfig.LintConfig)
	opaqueIDToBreakingConfig := make(map[string]bufconfig.BreakingConfig)
	for _, module := range moduleSet.Modules() {
		if bucketID := module.BucketID(); bucketID != "" {
			moduleConfig, ok := bucketIDToModuleConfig[bucketID]
			if !ok {
				// This is a system error.
				return nil, syserror.Newf("could not get ModuleConfig for BucketID %q", bucketID)
			}
			opaqueIDToLintConfig[module.OpaqueID()] = moduleConfig.LintConfig()
			opaqueIDToBreakingConfig[module.OpaqueID()] = moduleConfig.BreakingConfig()
		} else {
			opaqueIDToLintConfig[module.OpaqueID()] = bufconfig.DefaultLintConfig
			opaqueIDToBreakingConfig[module.OpaqueID()] = bufconfig.DefaultBreakingConfig
		}
	}
	return newWorkspace(
		moduleSet,
		opaqueIDToLintConfig,
		opaqueIDToBreakingConfig,
		configuredDepModuleRefs,
		isV2,
	), nil
}

func getModuleConfigAndConfiguredDepModuleRefsV1Beta1OrV1(
	ctx context.Context,
	bucket storage.ReadBucket,
	moduleDirPath string,
	overrideBufYAMLFile bufconfig.BufYAMLFile,
) (bufconfig.ModuleConfig, []bufmodule.ModuleRef, error) {
	var bufYAMLFile bufconfig.BufYAMLFile
	var err error
	if overrideBufYAMLFile != nil {
		bufYAMLFile = overrideBufYAMLFile
	} else {
		bufYAMLFile, err = bufconfig.GetBufYAMLFileForPrefix(ctx, bucket, moduleDirPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// If we do not have a buf.yaml, we use the default config.
				// This is a v1 config.
				return bufconfig.DefaultModuleConfig, nil, nil
			}
			return nil, nil, err
		}
	}
	// Just a sanity check. This should have already been validated, but let's make sure.
	if bufYAMLFile.FileVersion() != bufconfig.FileVersionV1Beta1 && bufYAMLFile.FileVersion() != bufconfig.FileVersionV1 {
		return nil, nil, syserror.Newf("buf.yaml at %s did not have version v1beta1 or v1", moduleDirPath)
	}
	moduleConfigs := bufYAMLFile.ModuleConfigs()
	if len(moduleConfigs) != 1 {
		// This is a system error. This should never happen.
		return nil, nil, syserror.Newf("received %d ModuleConfigs from a v1beta1 or v1 BufYAMLFile", len(moduleConfigs))
	}
	return moduleConfigs[0], bufYAMLFile.ConfiguredDepModuleRefs(), nil
}

func getMappedModuleBucketAndModuleTargeting(
	ctx context.Context,
	bucket storage.ReadBucket,
	config *workspaceBucketConfig,
	moduleDirPath string,
	moduleConfig bufconfig.ModuleConfig,
	isTargetModule bool,
) (storage.ReadBucket, *moduleTargeting, error) {
	moduleBucket := storage.MapReadBucket(
		bucket,
		storage.MapOnPrefix(moduleDirPath),
	)
	rootToExcludes := moduleConfig.RootToExcludes()
	var rootBuckets []storage.ReadBucket
	for root, excludes := range rootToExcludes {
		// Roots only applies to .proto files.
		mappers := []storage.Mapper{
			// need to do match extension here
			// https://github.com/bufbuild/buf/issues/113
			storage.MatchPathExt(".proto"),
			storage.MapOnPrefix(root),
		}
		if len(excludes) != 0 {
			var notOrMatchers []storage.Matcher
			for _, exclude := range excludes {
				notOrMatchers = append(
					notOrMatchers,
					storage.MatchPathContained(exclude),
				)
			}
			mappers = append(
				mappers,
				storage.MatchNot(
					storage.MatchOr(
						notOrMatchers...,
					),
				),
			)
		}
		rootBuckets = append(
			rootBuckets,
			storage.MapReadBucket(
				moduleBucket,
				mappers...,
			),
		)
	}
	rootBuckets = append(
		rootBuckets,
		bufmodule.GetDocStorageReadBucket(ctx, moduleBucket),
		bufmodule.GetLicenseStorageReadBucket(moduleBucket),
	)
	mappedModuleBucket := storage.MultiReadBucket(rootBuckets...)
	moduleTargeting, err := newModuleTargeting(
		moduleDirPath,
		slicesext.MapKeysToSlice(rootToExcludes),
		config,
		isTargetModule,
	)
	if err != nil {
		return nil, nil, err
	}
	return mappedModuleBucket, moduleTargeting, nil
}
