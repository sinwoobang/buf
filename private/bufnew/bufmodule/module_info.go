package bufmodule

import (
	"context"

	"github.com/bufbuild/buf/private/bufpkg/bufcas"
)

// ModuleInfo contains identifying information for a Module.
//
// It is embedded inside a Module, and therefore is always available from FileInfos as well.
// It can also be used to get Modules with the ModuleProvider.
//
// A caller using ModuleInfo can choose to verify whether or not certain properties are set
// depending on the context. For example, when dealing with non-colocated Modules, we will
// expect that ModuleFullName and CommitID are present, and this can be validated for.
type ModuleInfo interface {
	// ModuleFullName returns the full name of the Module.
	//
	// May be nil depending on context. For example, when read from lock files, this will
	// never be nil, however on Modules, it may be. You should check if this is nil when
	// performing operations, and error if you have a different expectation.
	ModuleFullName() ModuleFullName
	// CommitID returns the ID of the Commit, if present.
	//
	// This is an ID of a Commit on the BSR, and can be used in API operations.
	//
	// May be empty depending on context. For example, when read from lock files, this will
	// never be empty, however on Modules, it may be. You should check if this is empty when
	// performing operations, and error if you have a different expectation.
	//
	// If ModuleFullName is nil, this will always be empty.
	CommitID() string
	// Digest returns the Module digest.
	//
	// Implementations may choose to cache the Digest, in which case contexts passed
	// to this function in future calls will be ignored.
	Digest(context.Context) (bufcas.Digest, error)

	isModuleInfo()
}

// *** PRIVATE ***

type moduleInfo struct {
	moduleFullName ModuleFullName
	commitID       string
	digest         bufcas.Digest
}

func newModuleInfo(
	moduleFullName ModuleFullName,
	commitID string,
	digest bufcas.Digest,
) *moduleInfo {
	return &moduleInfo{
		moduleFullName: moduleFullName,
		commitID:       commitID,
		digest:         digest,
	}
}

func (m *moduleInfo) ModuleFullName() ModuleFullName {
	return m.moduleFullName
}

func (m *moduleInfo) CommitID() string {
	return m.commitID
}

func (m *moduleInfo) Digest(context.Context) (bufcas.Digest, error) {
	return m.digest, nil
}

func (*moduleInfo) isModuleInfo() {}