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
	"errors"
	"fmt"
	"sync"

	modulev1beta1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1beta1"
	"connectrpc.com/connect"
	"github.com/bufbuild/buf/private/bufnew/bufapi"
	"github.com/bufbuild/buf/private/bufpkg/bufcas"
)

// ModuleProvider provides Modules for ModuleInfos.
//
// TODO: Add plural method? Will make calls below a lot more efficient in the case
// of overlapinfog FileNodes.
type ModuleProvider interface {
	// GetModuleForModuleInfo gets the Module for the given ModuleInfo.
	//
	// The ModuleInfo must have a non-nil ModuleFullName.
	GetModuleForModuleInfo(context.Context, ModuleInfo) (Module, error)
}

// NewAPIModuleProvider returns a new ModuleProvider for the given API client.
//
// The Modules returned will be lazily-loaded: All functions except for the ModuleInfo
// functions will be loaded only when called. This allows us to more widely use the Module
// as a type (such as with dependencies) without incurring the lookup and building cost when
// all we want is ModuleInfo-related properties.
func NewAPIModuleProvider(clientProvider bufapi.ClientProvider) ModuleProvider {
	return newLazyModuleProvider(newAPIModuleProvider(clientProvider))
}

// *** PRIVATE ***

// apiModuleProvider

type apiModuleProvider struct {
	clientProvider bufapi.ClientProvider
}

func newAPIModuleProvider(clientProvider bufapi.ClientProvider) *apiModuleProvider {
	return &apiModuleProvider{
		clientProvider: clientProvider,
	}
}

func (a *apiModuleProvider) GetModuleForModuleInfo(ctx context.Context, moduleInfo ModuleInfo) (Module, error) {
	moduleFullName, err := getAndValidateModuleFullName(moduleInfo)
	if err != nil {
		return nil, err
	}
	var resourceRef *modulev1beta1.ResourceRef
	if commitID := moduleInfo.CommitID(); commitID != "" {
		resourceRef = &modulev1beta1.ResourceRef{
			Value: &modulev1beta1.ResourceRef_Id{
				Id: moduleInfo.CommitID(),
			},
		}
	} else {
		digest, err := moduleInfo.Digest()
		if err != nil {
			return nil, err
		}
		resourceRef = &modulev1beta1.ResourceRef{
			Value: &modulev1beta1.ResourceRef_Name_{
				Name: &modulev1beta1.ResourceRef_Name{
					Owner:  moduleFullName.Owner(),
					Module: moduleFullName.Name(),
					// TODO: change to digest when PR is merged
					Child: &modulev1beta1.ResourceRef_Name_Ref{
						Ref: digest.String(),
					},
				},
			},
		}
	}
	response, err := a.clientProvider.CommitServiceClient(moduleFullName.Registry()).GetCommitNodes(
		ctx,
		connect.NewRequest(
			&modulev1beta1.GetCommitNodesRequest{
				Values: []*modulev1beta1.GetCommitNodesRequest_Value{
					{
						ResourceRef: resourceRef,
					},
				},
			},
		),
	)
	if err != nil {
		return nil, err
	}
	if len(response.Msg.CommitNodes) != 1 {
		return nil, fmt.Errorf("expected 1 CommitNode, got %d", len(response.Msg.CommitNodes))
	}
	//commitNode := response.Msg.CommitNodes[0]
	// Can ignore the Commit field, as we already have all this information on ModuleInfo.
	// TODO: deal with Deps field when we have figured out deps on Modules
	return nil, errors.New("TODO")
}

// lazyModuleProvider

type lazyModuleProvider struct {
	delegate ModuleProvider
}

func newLazyModuleProvider(delegate ModuleProvider) *lazyModuleProvider {
	if lazyModuleProvider, ok := delegate.(*lazyModuleProvider); ok {
		return lazyModuleProvider
	}
	return &lazyModuleProvider{
		delegate: delegate,
	}
}

func (l *lazyModuleProvider) GetModuleForModuleInfo(ctx context.Context, moduleInfo ModuleInfo) (Module, error) {
	if _, err := getAndValidateModuleFullName(moduleInfo); err != nil {
		return nil, err
	}
	return newLazyModule(
		ctx,
		moduleInfo,
		func() (Module, error) {
			// Using ctx on GetModuleForModuleInfo and ignoring the contexts passed to
			// Module functions - arguable both ways for different reasons.
			return l.delegate.GetModuleForModuleInfo(ctx, moduleInfo)
		},
	), nil
}

// lazyModule

type lazyModule struct {
	ModuleInfo

	getModuleAndDigest func() (Module, bufcas.Digest, error)
	getDepModules      func() ([]Module, error)

	potentialDepModules []Module
}

func newLazyModule(
	ctx context.Context,
	// We know this ModuleInfo always has a ModuleFullName via lazyModuleProvider.
	moduleInfo ModuleInfo,
	getModuleFunc func() (Module, error),
) Module {
	lazyModule := &lazyModule{
		ModuleInfo: moduleInfo,
		getModuleAndDigest: onceThreeValues(
			func() (Module, bufcas.Digest, error) {
				module, err := getModuleFunc()
				if err != nil {
					return nil, nil, err
				}
				expectedDigest, err := moduleInfo.Digest()
				if err != nil {
					return nil, nil, err
				}
				actualDigest, err := module.Digest()
				if err != nil {
					return nil, nil, err
				}
				if !bufcas.DigestEqual(expectedDigest, actualDigest) {
					return nil, nil, fmt.Errorf("expected digest %v, got %v", expectedDigest, actualDigest)
				}
				return module, actualDigest, nil
			},
		),
	}
	lazyModule.getDepModules = sync.OnceValues(
		func() ([]Module, error) {
			module, _, err := lazyModule.getModuleAndDigest()
			if err != nil {
				return nil, err
			}
			potentialDepModules, err := module.DepModules(ctx)
			if err != nil {
				return nil, err
			}
			// Prefer declared dependencies first, as these are not ready from remote.
			return getActualDepModules(ctx, lazyModule, append(lazyModule.potentialDepModules, potentialDepModules...))
		},
	)
	return lazyModule
}

func (m *lazyModule) Digest() (bufcas.Digest, error) {
	_, digest, err := m.getModuleAndDigest()
	return digest, err
}

func (m *lazyModule) GetFile(ctx context.Context, path string) (File, error) {
	module, _, err := m.getModuleAndDigest()
	if err != nil {
		return nil, err
	}
	return module.GetFile(ctx, path)
}

func (m *lazyModule) StatFileInfo(ctx context.Context, path string) (FileInfo, error) {
	module, _, err := m.getModuleAndDigest()
	if err != nil {
		return nil, err
	}
	return module.StatFileInfo(ctx, path)
}

func (m *lazyModule) WalkFileInfos(ctx context.Context, f func(FileInfo) error) error {
	module, _, err := m.getModuleAndDigest()
	if err != nil {
		return err
	}
	return module.WalkFileInfos(ctx, f)
}

func (m *lazyModule) DepModules(ctx context.Context) ([]Module, error) {
	return m.getDepModules()
}

func (m *lazyModule) addPotentialDepModules(depModules ...Module) {
	m.potentialDepModules = append(m.potentialDepModules, depModules...)
}

func (m *lazyModule) opaqueID() string {
	// We know ModuleFullName is present via construction.
	return m.ModuleFullName().String()
}

func (*lazyModule) isModuleReadBucket() {}
func (*lazyModule) isModule()           {}
