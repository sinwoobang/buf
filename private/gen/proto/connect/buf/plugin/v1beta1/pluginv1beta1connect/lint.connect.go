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

// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: buf/plugin/v1beta1/lint.proto

package pluginv1beta1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1beta1 "github.com/bufbuild/buf/private/gen/proto/go/buf/plugin/v1beta1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect.IsAtLeastVersion1_13_0

const (
	// LintServiceName is the fully-qualified name of the LintService service.
	LintServiceName = "buf.plugin.v1beta1.LintService"
	// BreakingServiceName is the fully-qualified name of the BreakingService service.
	BreakingServiceName = "buf.plugin.v1beta1.BreakingService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// LintServiceLintProcedure is the fully-qualified name of the LintService's Lint RPC.
	LintServiceLintProcedure = "/buf.plugin.v1beta1.LintService/Lint"
	// BreakingServiceBreakingProcedure is the fully-qualified name of the BreakingService's Breaking
	// RPC.
	BreakingServiceBreakingProcedure = "/buf.plugin.v1beta1.BreakingService/Breaking"
)

// These variables are the protoreflect.Descriptor objects for the RPCs defined in this package.
var (
	lintServiceServiceDescriptor            = v1beta1.File_buf_plugin_v1beta1_lint_proto.Services().ByName("LintService")
	lintServiceLintMethodDescriptor         = lintServiceServiceDescriptor.Methods().ByName("Lint")
	breakingServiceServiceDescriptor        = v1beta1.File_buf_plugin_v1beta1_lint_proto.Services().ByName("BreakingService")
	breakingServiceBreakingMethodDescriptor = breakingServiceServiceDescriptor.Methods().ByName("Breaking")
)

// LintServiceClient is a client for the buf.plugin.v1beta1.LintService service.
type LintServiceClient interface {
	Lint(context.Context, *connect.Request[v1beta1.LintRequest]) (*connect.Response[v1beta1.LintResponse], error)
}

// NewLintServiceClient constructs a client for the buf.plugin.v1beta1.LintService service. By
// default, it uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses,
// and sends uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the
// connect.WithGRPC() or connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewLintServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) LintServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &lintServiceClient{
		lint: connect.NewClient[v1beta1.LintRequest, v1beta1.LintResponse](
			httpClient,
			baseURL+LintServiceLintProcedure,
			connect.WithSchema(lintServiceLintMethodDescriptor),
			connect.WithIdempotency(connect.IdempotencyNoSideEffects),
			connect.WithClientOptions(opts...),
		),
	}
}

// lintServiceClient implements LintServiceClient.
type lintServiceClient struct {
	lint *connect.Client[v1beta1.LintRequest, v1beta1.LintResponse]
}

// Lint calls buf.plugin.v1beta1.LintService.Lint.
func (c *lintServiceClient) Lint(ctx context.Context, req *connect.Request[v1beta1.LintRequest]) (*connect.Response[v1beta1.LintResponse], error) {
	return c.lint.CallUnary(ctx, req)
}

// LintServiceHandler is an implementation of the buf.plugin.v1beta1.LintService service.
type LintServiceHandler interface {
	Lint(context.Context, *connect.Request[v1beta1.LintRequest]) (*connect.Response[v1beta1.LintResponse], error)
}

// NewLintServiceHandler builds an HTTP handler from the service implementation. It returns the path
// on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewLintServiceHandler(svc LintServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	lintServiceLintHandler := connect.NewUnaryHandler(
		LintServiceLintProcedure,
		svc.Lint,
		connect.WithSchema(lintServiceLintMethodDescriptor),
		connect.WithIdempotency(connect.IdempotencyNoSideEffects),
		connect.WithHandlerOptions(opts...),
	)
	return "/buf.plugin.v1beta1.LintService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case LintServiceLintProcedure:
			lintServiceLintHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedLintServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedLintServiceHandler struct{}

func (UnimplementedLintServiceHandler) Lint(context.Context, *connect.Request[v1beta1.LintRequest]) (*connect.Response[v1beta1.LintResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("buf.plugin.v1beta1.LintService.Lint is not implemented"))
}

// BreakingServiceClient is a client for the buf.plugin.v1beta1.BreakingService service.
type BreakingServiceClient interface {
	Breaking(context.Context, *connect.Request[v1beta1.BreakingRequest]) (*connect.Response[v1beta1.BreakingResponse], error)
}

// NewBreakingServiceClient constructs a client for the buf.plugin.v1beta1.BreakingService service.
// By default, it uses the Connect protocol with the binary Protobuf Codec, asks for gzipped
// responses, and sends uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the
// connect.WithGRPC() or connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewBreakingServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) BreakingServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &breakingServiceClient{
		breaking: connect.NewClient[v1beta1.BreakingRequest, v1beta1.BreakingResponse](
			httpClient,
			baseURL+BreakingServiceBreakingProcedure,
			connect.WithSchema(breakingServiceBreakingMethodDescriptor),
			connect.WithIdempotency(connect.IdempotencyNoSideEffects),
			connect.WithClientOptions(opts...),
		),
	}
}

// breakingServiceClient implements BreakingServiceClient.
type breakingServiceClient struct {
	breaking *connect.Client[v1beta1.BreakingRequest, v1beta1.BreakingResponse]
}

// Breaking calls buf.plugin.v1beta1.BreakingService.Breaking.
func (c *breakingServiceClient) Breaking(ctx context.Context, req *connect.Request[v1beta1.BreakingRequest]) (*connect.Response[v1beta1.BreakingResponse], error) {
	return c.breaking.CallUnary(ctx, req)
}

// BreakingServiceHandler is an implementation of the buf.plugin.v1beta1.BreakingService service.
type BreakingServiceHandler interface {
	Breaking(context.Context, *connect.Request[v1beta1.BreakingRequest]) (*connect.Response[v1beta1.BreakingResponse], error)
}

// NewBreakingServiceHandler builds an HTTP handler from the service implementation. It returns the
// path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewBreakingServiceHandler(svc BreakingServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	breakingServiceBreakingHandler := connect.NewUnaryHandler(
		BreakingServiceBreakingProcedure,
		svc.Breaking,
		connect.WithSchema(breakingServiceBreakingMethodDescriptor),
		connect.WithIdempotency(connect.IdempotencyNoSideEffects),
		connect.WithHandlerOptions(opts...),
	)
	return "/buf.plugin.v1beta1.BreakingService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case BreakingServiceBreakingProcedure:
			breakingServiceBreakingHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedBreakingServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedBreakingServiceHandler struct{}

func (UnimplementedBreakingServiceHandler) Breaking(context.Context, *connect.Request[v1beta1.BreakingRequest]) (*connect.Response[v1beta1.BreakingResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("buf.plugin.v1beta1.BreakingService.Breaking is not implemented"))
}