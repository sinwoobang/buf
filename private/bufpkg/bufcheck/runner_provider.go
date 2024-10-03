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

package bufcheck

import (
	"context"
	"sync"

	"github.com/bufbuild/buf/private/bufpkg/bufconfig"
	"github.com/bufbuild/buf/private/bufpkg/bufplugin"
	"github.com/bufbuild/buf/private/pkg/command"
	"github.com/bufbuild/buf/private/pkg/pluginrpcutil"
	"github.com/bufbuild/buf/private/pkg/syserror"
	"github.com/bufbuild/buf/private/pkg/wasm"
	"pluginrpc.com/pluginrpc"
)

type runnerProvider struct {
	commandRunner      command.Runner
	wasmRuntime        wasm.Runtime
	pluginKeyProvider  bufplugin.PluginKeyProvider
	pluginDataProvider bufplugin.PluginDataProvider
}

func newRunnerProvider(
	commandRunner command.Runner,
	wasmRuntime wasm.Runtime,
	pluginKeyProvider bufplugin.PluginKeyProvider,
	pluginDataProvider bufplugin.PluginDataProvider,
) *runnerProvider {
	return &runnerProvider{
		commandRunner:      commandRunner,
		wasmRuntime:        wasmRuntime,
		pluginKeyProvider:  pluginKeyProvider,
		pluginDataProvider: pluginDataProvider,
	}
}

func (r *runnerProvider) NewRunner(pluginConfig bufconfig.PluginConfig) (pluginrpc.Runner, error) {
	switch pluginConfig.Type() {
	case bufconfig.PluginConfigTypeLocal:
		path := pluginConfig.Path()
		return pluginrpcutil.NewRunner(
			r.commandRunner,
			// We know that Path is of at least length 1.
			path[0],
			path[1:]...,
		), nil
	case bufconfig.PluginConfigTypeLocalWasm:
		path := pluginConfig.Path()
		return pluginrpcutil.NewWasmRunner(
			r.wasmRuntime,
			// We know that Path is of at least length 1.
			path[0],
			path[1:]...,
		), nil
	case bufconfig.PluginConfigTypeRemote:
		var (
			once              sync.Once
			compiledModule    wasm.CompiledModule
			compiledModuleErr error
		)
		return rpcRunnerFunc(func(ctx context.Context, env pluginrpc.Env) error {
			once.Do(func() {
				compiledModule, compiledModuleErr = r.loadRemotePlugin(ctx, pluginConfig)
			})
			if compiledModuleErr != nil {
				return compiledModuleErr
			}
			return compiledModule.Run(ctx, env)

		}), nil

	default:
		return nil, syserror.Newf("unknown PluginConfigType: %v", pluginConfig.Type())
	}
}

type rpcRunnerFunc func(ctx context.Context, env pluginrpc.Env) error

func (f rpcRunnerFunc) Run(ctx context.Context, env pluginrpc.Env) error {
	return f(ctx, env)
}

func (r *runnerProvider) loadRemotePlugin(ctx context.Context, pluginConfig bufconfig.PluginConfig) (wasm.CompiledModule, error) {
	pluginRef := pluginConfig.PluginRef()
	if pluginRef == nil {
		return nil, syserror.New("pluginRef is required for remote plugins")
	}
	pluginKeys, err := r.pluginKeyProvider.GetPluginKeysForPluginRefs(
		ctx,
		[]bufplugin.PluginRef{pluginRef},
		bufplugin.DigestTypeP1,
	)
	if err != nil {
		return nil, err
	}
	if len(pluginKeys) != 1 {
		return nil, syserror.Newf("expected 1 plugin key for %s", pluginRef)
	}
	pluginDatas, err := r.pluginDataProvider.GetPluginDatasForPluginKeys(
		ctx,
		pluginKeys,
	)
	if err != nil {
		return nil, err
	}
	if len(pluginDatas) != 1 {
		return nil, syserror.Newf("expected 1 plugin data for %s", pluginRef)
	}
	pluginData := pluginDatas[0]

	data, err := pluginData.Data()
	if err != nil {
		return nil, err
	}
	return r.wasmRuntime.Compile(ctx, pluginConfig.Name(), data)
}
