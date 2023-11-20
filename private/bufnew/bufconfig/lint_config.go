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

package bufconfig

// TODO
var DefaultLintConfig LintConfig = newLintConfig()

// LintConfig is lint configuration for a specific Module.
type LintConfig interface {
	CheckConfig

	EnumZeroValueSuffix() string
	RPCAllowSameRequestResponse() bool
	RPCAllowGoogleProtobufEmptyRequests() bool
	RPCAllowGoogleProtobufEmptyResponses() bool
	ServiceSuffix() string
	AllowCommentIgnores() bool

	isLintConfig()
}

// *** PRIVATE ***

type lintConfig struct {
	checkConfig
}

func newLintConfig() *lintConfig {
	return &lintConfig{}
}

func (l *lintConfig) EnumZeroValueSuffix() string {
	panic("not implemented") // TODO: Implement
}

func (l *lintConfig) RPCAllowSameRequestResponse() bool {
	panic("not implemented") // TODO: Implement
}

func (l *lintConfig) RPCAllowGoogleProtobufEmptyRequests() bool {
	panic("not implemented") // TODO: Implement
}

func (l *lintConfig) RPCAllowGoogleProtobufEmptyResponses() bool {
	panic("not implemented") // TODO: Implement
}

func (l *lintConfig) ServiceSuffix() string {
	panic("not implemented") // TODO: Implement
}

func (l *lintConfig) AllowCommentIgnores() bool {
	panic("not implemented") // TODO: Implement
}

func (*lintConfig) isLintConfig() {}
