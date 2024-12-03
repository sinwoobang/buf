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

package bufplugin

import (
	"fmt"
	"strings"
	"sync"

	"github.com/bufbuild/buf/private/bufpkg/bufcas"
	"github.com/bufbuild/buf/private/bufpkg/bufparse"
	"github.com/bufbuild/buf/private/pkg/syserror"
	"github.com/google/uuid"
)

// Plugin presents a BSR plugin.
type Plugin interface {
	// OpaqueID returns an unstructured ID that can uniquely identify a Plugin
	// relative to the Workspace.
	//
	// An OpaqueID's structure should not be relied upon, and is not a
	// globally-unique identifier. It's uniqueness property only applies to
	// the lifetime of the Plugin, and only within Plugin commonly built
	// from the Workspace root.
	//
	// If two Plugins have the same FullName, they will have the same OpaqueID.
	OpaqueID() string
	// Path returns the path, including arguments, to invoke the binary plugin.
	//
	// This is not empty only when the plugin is local.
	Path() []string
	// FullName returns the FullName of the Plugin.
	//
	// This is nil
	FullName() bufparse.FullName
	// CommitID returns the BSR ID of the Commit.
	//
	// It is up to the caller to convert this to a dashless ID when necessary.
	//
	// May be empty, that is CommitID() == uuid.Nil may be true.
	// Callers should not rely on this value being present.
	//
	// If FullName is nil, this will always be empty.
	CommitID() uuid.UUID
	// Description returns a human-readable description of the Plugin.
	//
	// This is used to construct descriptive error messages pointing to configured plugins.
	//
	// This will never be empty. If a description was not explicitly set, this falls back to
	// OpaqueID.
	Description() string
	// Digest returns the Plugin digest for the given DigestType.
	//
	// Note this is *not* a bufcas.Digest - this is a Digest.
	// bufcas.Digests are a lower-level type that just deal in terms of
	// files and content. A Digest is a specific algorithm applied to the
	// content of a Plugin.
	//
	// Will return an error if the Plugin is not a Wasm Plugin.
	Digest(DigestType) (Digest, error)
	// Data returns the bytes of the Plugin as a Wasm module.
	//
	// This is the raw bytes of the Wasm module in an uncompressed form.
	//
	// Will return an error if the Plugin is not a Wasm Plugin.
	Data() ([]byte, error)
	// IsWasm returns true if the Plugin is a Wasm Plugin.
	//
	// Plugins are either Wasm or not Wasm.
	//
	// A Wasm Plugin is a Plugin that is a Wasm module. Wasm Plugins are invoked
	// with the wasm.Runtime. The Plugin will have Data and will be able to
	// calculate Digests.
	//
	// Wasm Plugins will always have Data.
	IsWasm() bool
	// IsLocal returns true if the Plugin is a local Plugin.
	//
	// Plugins are either local or remote.
	//
	// A local Plugin is one that is built from sources from the "local context",
	// such as a Workspace. Local Plugins are important for understanding what Plugins
	// to push.
	//
	// Remote Plugins will always have FullNames.
	IsLocal() bool

	isPlugin()
}

// NewLocalWasmPlugin returns a new Plugin for a local Wasm plugin.
func NewLocalWasmPlugin(
	pluginFullName bufparse.FullName,
	getData func() ([]byte, error),
) (Plugin, error) {
	return newPlugin(
		"", // description
		pluginFullName,
		nil,      // path
		uuid.Nil, // commitID
		true,     // isWasm
		true,     // isLocal
		getData,
	)
}

// *** PRIVATE ***

type plugin struct {
	description    string
	pluginFullName bufparse.FullName
	path           []string
	commitID       uuid.UUID
	isWasm         bool
	isLocal        bool
	getData        func() ([]byte, error)

	digestTypeToGetDigest map[DigestType]func() (Digest, error)
}

func newPlugin(
	description string,
	pluginFullName bufparse.FullName,
	path []string,
	commitID uuid.UUID,
	isWasm bool,
	isLocal bool,
	getData func() ([]byte, error),
) (*plugin, error) {
	if isWasm && getData == nil {
		return nil, syserror.Newf("getData not present when constructing a Wasm Plugin")
	}
	if !isWasm && len(path) == 0 {
		return nil, syserror.New("path not present when constructing a non-Wasm Plugin")
	}
	if !isLocal && pluginFullName == nil {
		return nil, syserror.New("pluginFullName not present when constructing a remote Plugin")
	}
	if !isLocal && !isWasm {
		return nil, syserror.New("non-Wasm remote Plugins are not supported")
	}
	if isLocal && commitID != uuid.Nil {
		return nil, syserror.New("commitID present when constructing a local Plugin")
	}
	if pluginFullName == nil && commitID != uuid.Nil {
		return nil, syserror.New("pluginFullName not present and commitID present when constructing a remote Plugin")
	}
	plugin := &plugin{
		description:    description,
		pluginFullName: pluginFullName,
		path:           path,
		commitID:       commitID,
		isWasm:         isWasm,
		isLocal:        isLocal,
		getData:        sync.OnceValues(getData),
	}
	plugin.digestTypeToGetDigest = newSyncOnceValueDigestTypeToGetDigestFuncForPlugin(plugin)
	return plugin, nil
}

func (p *plugin) OpaqueID() string {
	if p.pluginFullName != nil {
		return p.pluginFullName.String()
	}
	return strings.Join(p.path, " ")
}

func (p *plugin) Path() []string {
	return p.path
}

func (p *plugin) FullName() bufparse.FullName {
	return p.pluginFullName
}

func (p *plugin) CommitID() uuid.UUID {
	return p.commitID
}

func (p *plugin) Description() string {
	if p.description != "" {
		return p.description
	}
	return p.OpaqueID()
}

func (p *plugin) Data() ([]byte, error) {
	if !p.isWasm {
		return nil, fmt.Errorf("Plugin is not a Wasm Plugin")
	}
	return p.getData()
}

func (p *plugin) Digest(digestType DigestType) (Digest, error) {
	getDigest, ok := p.digestTypeToGetDigest[digestType]
	if !ok {
		return nil, syserror.Newf("DigestType %v was not in plugin.digestTypeToGetDigest", digestType)
	}
	return getDigest()
}

func (p *plugin) IsWasm() bool {
	return p.isWasm
}

func (p *plugin) IsLocal() bool {
	return p.isLocal
}

func (p *plugin) isPlugin() {}

func newSyncOnceValueDigestTypeToGetDigestFuncForPlugin(plugin *plugin) map[DigestType]func() (Digest, error) {
	m := make(map[DigestType]func() (Digest, error))
	for digestType := range digestTypeToString {
		m[digestType] = sync.OnceValues(newGetDigestFuncForPluginAndDigestType(plugin, digestType))
	}
	return m
}

func newGetDigestFuncForPluginAndDigestType(plugin *plugin, digestType DigestType) func() (Digest, error) {
	return func() (Digest, error) {
		data, err := plugin.getData()
		if err != nil {
			return nil, err
		}
		bufcasDigest, err := bufcas.NewDigest(data)
		if err != nil {
			return nil, err
		}
		return NewDigest(digestType, bufcasDigest)
	}
}