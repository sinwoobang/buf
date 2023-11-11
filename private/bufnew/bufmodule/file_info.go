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
	"github.com/bufbuild/buf/private/pkg/slicesextended"
	"github.com/bufbuild/buf/private/pkg/storage"
)

// FileInfo is the file info for a Module file.
//
// It comprises the typical storage.ObjectInfo, along with a pointer back to the Module.
// This allows callers to figure out i.e. the ModuleFullName, Commit, as well as any other
// data it may need.
type FileInfo interface {
	storage.ObjectInfo

	// Module returns the Module that contains this file.
	Module() Module
	// FileType returns the FileType of the file.
	//
	// This denotes if the File is a .proto file, documentation file, or license file.
	FileType() FileType

	// IsTargetFile returns true if the File is targeted.
	//
	// Files are either targets or imports.
	// If IsTargetModule() is false, this will always be false.
	//
	// If specific Files were not targeted but IsTargetModule() is true, all Files in
	// the Module will have IsTargetFile() set to true.
	IsTargetFile() bool

	//setIsTargetFile(bool)
	isFileInfo()
}

// FileInfoPaths is a convenience function that returns the paths of the FileInfos.
func FileInfoPaths(fileInfos []FileInfo) []string {
	return slicesextended.Map(fileInfos, func(fileInfo FileInfo) string { return fileInfo.Path() })
}

// *** PRIVATE ***

type fileInfo struct {
	storage.ObjectInfo

	module       Module
	fileType     FileType
	isTargetFile bool
}

func newFileInfo(
	objectInfo storage.ObjectInfo,
	module Module,
	fileType FileType,
	isTargetFile bool,
) *fileInfo {
	return &fileInfo{
		ObjectInfo:   objectInfo,
		module:       module,
		fileType:     fileType,
		isTargetFile: isTargetFile,
	}
}

func (f *fileInfo) Module() Module {
	return f.module
}

func (f *fileInfo) FileType() FileType {
	return f.fileType
}

func (f *fileInfo) IsTargetFile() bool {
	return f.isTargetFile
}

//func (f *fileInfo) setIsTargetFile(isTargetFile bool) {
//f.isTargetFile = isTargetFile
//}

func (*fileInfo) isFileInfo() {}