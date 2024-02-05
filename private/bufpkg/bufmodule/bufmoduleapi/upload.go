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

package bufmoduleapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	modulev1beta1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1beta1"
	"connectrpc.com/connect"
	"github.com/bufbuild/buf/private/bufpkg/bufapi"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/pkg/slicesext"
	"github.com/bufbuild/buf/private/pkg/syserror"
	"github.com/gofrs/uuid/v5"
)

// Upload uploads the given Modules.
//
// All Modules are expected to be local Modules.
//
// It is expected that if any Module has a dependency on another local Module, that Module is within
// the targetLocalModulesAndTransitiveLocalDeps slice.
//
// Use bufmodule.ModuleSetTargetLocalModulesAndTransitiveLocalDeps to compute the modules list.
//
// Commits will be returned in the order of the input Modules.
func Upload(
	ctx context.Context,
	clientProvider bufapi.UploadServiceClientProvider,
	targetLocalModulesAndTransitiveLocalDeps []bufmodule.Module,
	options ...UploadOption,
) ([]bufmodule.Commit, error) {
	uploadOptions := newUploadOptions()
	for _, option := range options {
		option(uploadOptions)
	}

	registryMap := make(map[string]struct{})
	for _, module := range targetLocalModulesAndTransitiveLocalDeps {
		if !module.IsLocal() {
			return nil, syserror.Newf("non-local module attempted to be uploaded: %q", module.OpaqueID())
		}
		moduleFullName := module.ModuleFullName()
		if moduleFullName == nil {
			// This might actually happen.
			return nil, newRequireModuleFullNameOnUploadError(module)
		}
		registryMap[moduleFullName.Registry()] = struct{}{}
	}
	// Validate we're all within one registry for now.
	registries := slicesext.MapKeysToSortedSlice(registryMap)
	if len(registries) > 1 {
		// TODO: This messes up legacy federation.
		return nil, fmt.Errorf("multiple registries detected: %s", strings.Join(registries, ", "))
	}
	registry := registries[0]

	// While the API allows different labels per reference, we don't have a use case for this
	// in the CLI, so all references will have the same labels. We just pre-compute them now.
	protoScopedLabelRefs := slicesext.Map(
		slicesext.ToUniqueSorted(uploadOptions.labels),
		labelNameToProtoScopedLabelRef,
	)
	uploadedModuleOpaqueIDs := slicesext.ToStructMap(slicesext.Map(targetLocalModulesAndTransitiveLocalDeps, bufmodule.Module.OpaqueID))
	protoUploadRequestContents, err := slicesext.MapError(
		targetLocalModulesAndTransitiveLocalDeps,
		func(module bufmodule.Module) (*modulev1beta1.UploadRequest_Content, error) {
			return getProtoUploadRequestContent(
				ctx,
				protoScopedLabelRefs,
				uploadedModuleOpaqueIDs,
				module,
			)
		},
	)
	if err != nil {
		return nil, err
	}

	response, err := clientProvider.UploadServiceClient(registry).Upload(
		ctx,
		connect.NewRequest(
			&modulev1beta1.UploadRequest{
				Contents: protoUploadRequestContents,
			},
		),
	)
	if err != nil {
		return nil, err
	}
	if len(response.Msg.Commits) != len(protoUploadRequestContents) {
		return nil, fmt.Errorf("expected %d Commits, got %d", len(protoUploadRequestContents), len(response.Msg.Commits))
	}
	commits := make([]bufmodule.Commit, len(response.Msg.Commits))
	for i, protoCommit := range response.Msg.Commits {
		protoCommit := protoCommit
		// This is how we get the ModuleFullName without calling the ModuleService or OwnerService.
		moduleFullName := targetLocalModulesAndTransitiveLocalDeps[i].ModuleFullName()
		commitID, err := uuid.FromString(protoCommit.Id)
		if err != nil {
			return nil, err
		}
		moduleKey, err := bufmodule.NewModuleKey(
			moduleFullName,
			commitID,
			func() (bufmodule.Digest, error) {
				return ProtoToDigest(protoCommit.Digest)
			},
		)
		if err != nil {
			return nil, err
		}
		commits[i] = bufmodule.NewCommit(
			moduleKey,
			func() (time.Time, error) {
				return protoCommit.CreateTime.AsTime(), nil
			},
		)
	}
	return commits, nil
}

// UploadOption is an option for a new Upload.
type UploadOption func(*uploadOptions)

// UploadWithLabels returns a new UploadOption that adds the given labels.
//
// This can be called multiple times. The unique result set of labels will be used.
func UploadWithLabels(labels ...string) UploadOption {
	return func(uploadOptions *uploadOptions) {
		uploadOptions.labels = append(uploadOptions.labels, labels...)
	}
}

// *** PRIVATE ***

// Expects all Modules have ModuleFullNames.
func getProtoModuleRef(module bufmodule.Module) (*modulev1beta1.ModuleRef, error) {
	moduleFullName := module.ModuleFullName()
	if moduleFullName == nil {
		// This should be validated higher up.
		return nil, syserror.Newf("module %q did not have a ModuleFullName in getOpaqueIDToProtoModuleRef", module.OpaqueID())
	}
	return &modulev1beta1.ModuleRef{
		Value: &modulev1beta1.ModuleRef_Name_{
			Name: &modulev1beta1.ModuleRef_Name{
				// Note registry is not used here! See note on NewUploadRequest.
				Owner:  moduleFullName.Owner(),
				Module: moduleFullName.Name(),
			},
		},
	}, nil
}

func getProtoUploadRequestContent(
	ctx context.Context,
	// This slice is already populated.
	protoScopedLabelRefs []*modulev1beta1.ScopedLabelRef,
	// This map is already populated.
	uploadedModuleOpaqueIDs map[string]struct{},
	module bufmodule.Module,
) (*modulev1beta1.UploadRequest_Content, error) {
	if !module.IsLocal() {
		return nil, syserror.New("expected local Module in getProtoUploadRequestContent")
	}
	if module.ModuleFullName() == nil {
		return nil, syserror.Wrap(newRequireModuleFullNameOnUploadError(module))
	}
	protoModuleRef, err := getProtoModuleRef(module)
	if err != nil {
		return nil, err
	}

	// Includes transitive dependencies.
	// Sorted by OpaqueID.
	moduleDeps, err := module.ModuleDeps()
	if err != nil {
		return nil, err
	}
	protoDepRefs := make([]*modulev1beta1.UploadRequest_DepRef, 0, len(moduleDeps))
	for _, moduleDep := range moduleDeps {
		if moduleDep.ModuleFullName() == nil {
			// All modules that will be deps need a ModuleFullName.
			return nil, newRequireModuleFullNameOnUploadError(moduleDep)
		}
		depProtoModuleRef, err := getProtoModuleRef(moduleDep)
		if err != nil {
			return nil, err
		}
		if moduleDep.IsLocal() {
			if _, ok := uploadedModuleOpaqueIDs[moduleDep.OpaqueID()]; !ok {
				return nil, syserror.Newf("attempted to add local module dep %q when it was not scheduled to be uploaded", moduleDep.OpaqueID())
			}
			protoDepRefs = append(
				protoDepRefs,
				&modulev1beta1.UploadRequest_DepRef{
					ModuleRef: depProtoModuleRef,
				},
			)
		} else {
			// If the dependency is remote, add it as a dep ref.
			depCommitID := moduleDep.CommitID()
			if depCommitID.IsNil() {
				return nil, syserror.Newf("did not have a commit ID for a remote module dependency %q", moduleDep.OpaqueID())
			}
			protoDepRefs = append(
				protoDepRefs,
				&modulev1beta1.UploadRequest_DepRef{
					ModuleRef: depProtoModuleRef,
					CommitId:  depCommitID.String(),
				},
			)
		}
	}

	protoFiles, err := bucketToProtoFiles(ctx, bufmodule.ModuleReadBucketToStorageReadBucket(module))
	if err != nil {
		return nil, err
	}
	v1BufYAMLObjectData, err := module.V1Beta1OrV1BufYAMLObjectData()
	if err != nil {
		return nil, err
	}
	v1BufLockObjectData, err := module.V1Beta1OrV1BufLockObjectData()
	if err != nil {
		return nil, err
	}

	return &modulev1beta1.UploadRequest_Content{
		ModuleRef:       protoModuleRef,
		Files:           protoFiles,
		DepRefs:         protoDepRefs,
		ScopedLabelRefs: protoScopedLabelRefs,
		// TODO: We may end up synthesizing v1 buf.yamls/buf.locks on bufmodule.Module,
		// if we do, we should consider whether we should be sending them over, as the
		// backend may come to rely on this.
		V1BufYamlFile: objectDataToProtoFile(v1BufYAMLObjectData),
		V1BufLockFile: objectDataToProtoFile(v1BufLockObjectData),
		// TODO: vcs_commit
	}, nil
}

func newRequireModuleFullNameOnUploadError(module bufmodule.Module) error {
	// This error will likely actually go back to users.
	return fmt.Errorf("A name must be specified in buf.yaml for module %s for push.", module.OpaqueID())
}

type uploadOptions struct {
	labels []string
}

func newUploadOptions() *uploadOptions {
	return &uploadOptions{}
}
