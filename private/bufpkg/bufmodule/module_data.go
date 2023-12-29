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
	"fmt"

	"github.com/bufbuild/buf/private/pkg/storage"
	"github.com/bufbuild/buf/private/pkg/syncext"
)

// ModuleData presents raw Module data read by ModuleKey.
//
// It is not a fully-formed Module; only ModuleSetBuilders (and ModuleSets) can provide Modules.
//
// A ModuleData generally represents the data on a Module read from the BSR API or a cache.
type ModuleData interface {
	// ModuleKey contains the ModuleKey that was used to download this ModuleData.
	//
	// The Digest from this ModuleKey is used for tamper-proofing. It will be checked against
	// the actual data downloaded before Bucket() or DeclaredDepModuleKeys() returns.
	ModuleKey() ModuleKey
	// Bucket returns a Bucket of the Module's files.
	//
	// This bucket is not self-contained - it requires the files from dependencies to be so.
	//
	// This bucket will only contain module files - it will be filtered via NewModuleData.
	Bucket() (storage.ReadBucket, error)
	// DeclaredDepModuleKeys returns the declared dependencies for this specific Module.
	DeclaredDepModuleKeys() ([]ModuleKey, error)

	isModuleData()
}

// NewModuleData returns a new ModuleData.
//
// getBucket and getDeclaredDepModuleKeys are meant to be lazily-loaded functions where possible.
//
// It is OK for getBucket to return a bucket that has extra files that are not part of the Module, this
// bucket will be filtered as part of this function.
func NewModuleData(
	ctx context.Context,
	moduleKey ModuleKey,
	getBucket func() (storage.ReadBucket, error),
	getDeclaredDepModuleKeys func() ([]ModuleKey, error),
) ModuleData {
	return newModuleData(
		ctx,
		moduleKey,
		getBucket,
		getDeclaredDepModuleKeys,
	)
}

// OptionalModuleData is a result from a ModuleDataProvider.
//
// It returns whether or not the ModuleData was found, and a non-nil
// ModuleData if the ModuleData was found.
type OptionalModuleData interface {
	ModuleData() ModuleData
	Found() bool

	isOptionalModuleData()
}

// NewOptionalModuleData returns a new OptionalModuleData.
//
// As opposed to most functions in this codebase, the input ModuleData can be nil.
// If it is nil, then Found() will return false.
func NewOptionalModuleData(moduleData ModuleData) OptionalModuleData {
	return newOptionalModuleData(moduleData)
}

// *** PRIVATE ***

// moduleData

type moduleData struct {
	moduleKey                ModuleKey
	getBucket                func() (storage.ReadBucket, error)
	getDeclaredDepModuleKeys func() ([]ModuleKey, error)

	checkDigest func() error
}

func newModuleData(
	ctx context.Context,
	moduleKey ModuleKey,
	getBucket func() (storage.ReadBucket, error),
	getDeclaredDepModuleKeys func() ([]ModuleKey, error),
) *moduleData {
	moduleData := &moduleData{
		moduleKey:                moduleKey,
		getBucket:                getSyncOnceValuesGetBucketWithStorageMatcherApplied(ctx, getBucket),
		getDeclaredDepModuleKeys: syncext.OnceValues(getDeclaredDepModuleKeys),
	}
	moduleData.checkDigest = syncext.OnceValue(
		func() error {
			bucket, err := moduleData.getBucket()
			if err != nil {
				return err
			}
			declaredDepModuleKeys, err := moduleData.getDeclaredDepModuleKeys()
			if err != nil {
				return err
			}
			expectedDigest, err := moduleKey.Digest()
			if err != nil {
				return err
			}
			// This isn't the Digest as computed by the Module exactly, as the Module uses
			// file imports to determine what the dependencies are. However, this is checking whether
			// or not the digest of the returned information matches the digest we expected, which is
			// what we need for this use case (tamper-proofing). What we are looking for is "does the
			// digest from the ModuleKey match the files and dependencies returned from the remote
			// provider of the ModuleData?" The mismatch case is that a file import changed/was removed,
			// which may result in a different computed set of dependencies, but in this case, the
			// actual files would have changed, which will result in a mismatched digest anyways, and
			// tamper-proofing failing.
			//
			// This mismatch is a bit weird, however, and also results in us effectively computing
			// the digest twice for any remote module: once here, and once within Module.Digest,
			// which does have a slight performance hit.
			actualDigest, err := getB5Digest(
				ctx,
				bucket,
				declaredDepModuleKeys,
			)
			if err != nil {
				return err
			}
			if !DigestEqual(expectedDigest, actualDigest) {
				return fmt.Errorf(
					"verification failed for module %s: expected digest %q but downloaded data had digest %q",
					moduleKey.String(),
					expectedDigest.String(),
					actualDigest.String(),
				)
			}
			return nil
		},
	)
	return moduleData
}

func (m *moduleData) ModuleKey() ModuleKey {
	return m.moduleKey
}

func (m *moduleData) Bucket() (storage.ReadBucket, error) {
	if err := m.checkDigest(); err != nil {
		return nil, err
	}
	return m.getBucket()
}

func (m *moduleData) DeclaredDepModuleKeys() ([]ModuleKey, error) {
	// Do we need to tamper-proof when getting declared deps? Probably yes - this is
	// data that could be tampered with.
	//
	// Note that doing so kills some of our lazy-loading, as we call DeclaredDepModuleKeys
	// in ModuleSetBuilder right away. However, we still do the lazy-loading here, in the case
	// where ModuleData is loaded outside of a ModuleSetBuilder and users may defer calling this
	// function if it is not needed.
	if err := m.checkDigest(); err != nil {
		return nil, err
	}
	return m.getDeclaredDepModuleKeys()
}

func (*moduleData) isModuleData() {}

type optionalModuleData struct {
	moduleData ModuleData
}

func newOptionalModuleData(moduleData ModuleData) *optionalModuleData {
	return &optionalModuleData{
		moduleData: moduleData,
	}
}

func (o *optionalModuleData) ModuleData() ModuleData {
	return o.moduleData
}

func (o *optionalModuleData) Found() bool {
	return o.moduleData != nil
}

func (*optionalModuleData) isOptionalModuleData() {}
