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

package bufcheckserverutil

import (
	"context"

	"github.com/bufbuild/buf/private/bufpkg/bufprotosource"
	"github.com/bufbuild/buf/private/pkg/slicesext"
	"github.com/bufbuild/bufplugin-go/check"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
)

type protosourceFilesContextKey struct{}
type againstProtosourceFilesContextKey struct{}

// Before should be attached to each check.Spec that uses the functionality in this package.
func Before(
	ctx context.Context,
	request check.Request,
) (context.Context, check.Request, error) {
	protosourceFiles, err := protosourceFilesForFiles(ctx, request.Files())
	if err != nil {
		return nil, nil, err
	}
	againstProtosourceFiles, err := protosourceFilesForFiles(ctx, request.Files())
	if err != nil {
		return nil, nil, err
	}
	if len(protosourceFiles) > 0 {
		ctx = context.WithValue(ctx, protosourceFilesContextKey{}, protosourceFiles)
	}
	if len(againstProtosourceFiles) > 0 {
		ctx = context.WithValue(ctx, againstProtosourceFilesContextKey{}, againstProtosourceFiles)
	}
	return ctx, request, nil
}

// NewRuleHandler returns a new check.RuleHandler for the given function.
func NewRuleHandler(
	f func(
		ctx context.Context,
		responseWriter ResponseWriter,
		request Request,
	) error,
) check.RuleHandler {
	return check.RuleHandlerFunc(
		func(
			ctx context.Context,
			responseWriter check.ResponseWriter,
			request check.Request,
		) error {
			protosourceFiles, _ := ctx.Value(protosourceFilesContextKey{}).([]bufprotosource.File)
			againstProtosourceFiles, _ := ctx.Value(againstProtosourceFilesContextKey{}).([]bufprotosource.File)
			return f(
				ctx,
				newResponseWriter(responseWriter),
				newRequest(
					request,
					protosourceFiles,
					againstProtosourceFiles,
				),
			)
		},
	)
}

func protosourceFilesForFiles(ctx context.Context, files []check.File) ([]bufprotosource.File, error) {
	if len(files) == 0 {
		return nil, nil
	}
	resolver, err := protodesc.NewFiles(
		&descriptorpb.FileDescriptorSet{
			File: slicesext.Map(files, check.File.FileDescriptorProto),
		},
	)
	if err != nil {
		return nil, err
	}
	return bufprotosource.NewFiles(ctx, slicesext.Map(files, newInputFile), resolver)
}