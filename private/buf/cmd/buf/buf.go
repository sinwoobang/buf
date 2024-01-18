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

package buf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"connectrpc.com/connect"
	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/buf/bufctl"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/package/goversion"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/package/mavenversion"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/package/npmversion"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/package/pythonversion"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/package/swiftversion"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/protoc"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/registry/token/tokendelete"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/registry/token/tokenget"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/alpha/registry/token/tokenlist"
	betagraph "github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/graph"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/price"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/commit/commitget"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/commit/commitlist"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/draft/draftdelete"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/draft/draftlist"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/organization/organizationcreate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/organization/organizationdelete"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/organization/organizationget"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/plugin/plugindelete"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/plugin/pluginpush"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/repository/repositorycreate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/repository/repositorydelete"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/repository/repositorydeprecate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/repository/repositoryget"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/repository/repositorylist"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/repository/repositoryundeprecate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/repository/repositoryupdate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/tag/tagcreate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/tag/taglist"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/webhook/webhookcreate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/webhook/webhookdelete"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/registry/webhook/webhooklist"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/stats"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/beta/studioagent"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/breaking"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/build"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/convert"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/curl"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/export"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/format"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/generate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/graph"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/lint"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/lsfiles"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/lsp"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/migrate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/mod/modclearcache"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/mod/modinit"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/mod/modlsbreakingrules"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/mod/modlslintrules"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/mod/modopen"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/mod/modprune"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/mod/modupdate"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/push"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/registry/registrylogin"
	"github.com/bufbuild/buf/private/buf/cmd/buf/command/registry/registrylogout"
	"github.com/bufbuild/buf/private/bufpkg/bufconnect"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appext"
	"github.com/bufbuild/buf/private/pkg/syserror"
)

// Main is the entrypoint to the buf CLI.
func Main(name string) {
	appcmd.Main(context.Background(), NewRootCommand(name))
}

// NewRootCommand returns a new root command.
//
// This is public for use in testing.
func NewRootCommand(name string) *appcmd.Command {
	builder := appext.NewBuilder(
		name,
		appext.BuilderWithTimeout(120*time.Second),
		appext.BuilderWithTracing(),
		appext.BuilderWithInterceptor(newErrorInterceptor()),
	)
	return &appcmd.Command{
		Use:                 name,
		Short:               "The Buf CLI",
		Long:                "A tool for working with Protocol Buffers and managing resources on the Buf Schema Registry (BSR)",
		Version:             bufcli.Version,
		BindPersistentFlags: builder.BindRoot,
		SubCommands: []*appcmd.Command{
			build.NewCommand("build", builder),
			export.NewCommand("export", builder),
			format.NewCommand("format", builder),
			lint.NewCommand("lint", builder),
			breaking.NewCommand("breaking", builder),
			generate.NewCommand("generate", builder),
			lsfiles.NewCommand("ls-files", builder),
			// TODO: beta?
			lsp.NewCommand("lsp", builder),
			// TODO: x?
			graph.NewCommand("graph", builder),
			migrate.NewCommand("migrate", builder),
			push.NewCommand("push", builder),
			convert.NewCommand("convert", builder),
			curl.NewCommand("curl", builder),
			{
				Use:   "mod",
				Short: "Manage Buf modules",
				SubCommands: []*appcmd.Command{
					modinit.NewCommand("init", builder),
					modprune.NewCommand("prune", builder),
					modupdate.NewCommand("update", builder),
					modopen.NewCommand("open", builder),
					modclearcache.NewCommand("clear-cache", builder, "cc"),
					modlslintrules.NewCommand("ls-lint-rules", builder),
					modlsbreakingrules.NewCommand("ls-breaking-rules", builder),
				},
			},
			{
				Use:   "registry",
				Short: "Manage assets on the Buf Schema Registry",
				SubCommands: []*appcmd.Command{
					registrylogin.NewCommand("login", builder),
					registrylogout.NewCommand("logout", builder),
				},
			},
			{
				Use:   "beta",
				Short: "Beta commands. Unstable and likely to change",
				SubCommands: []*appcmd.Command{
					betagraph.NewCommand("graph", builder),
					price.NewCommand("price", builder),
					stats.NewCommand("stats", builder),
					studioagent.NewCommand("studio-agent", builder),
					{
						Use:   "registry",
						Short: "Manage assets on the Buf Schema Registry",
						SubCommands: []*appcmd.Command{
							{
								Use:   "organization",
								Short: "Manage organizations",
								SubCommands: []*appcmd.Command{
									organizationcreate.NewCommand("create", builder),
									organizationget.NewCommand("get", builder),
									organizationdelete.NewCommand("delete", builder),
								},
							},
							{
								Use:   "repository",
								Short: "Manage repositories",
								SubCommands: []*appcmd.Command{
									repositorycreate.NewCommand("create", builder),
									repositoryget.NewCommand("get", builder),
									repositorylist.NewCommand("list", builder),
									repositorydelete.NewCommand("delete", builder),
									repositorydeprecate.NewCommand("deprecate", builder),
									repositoryundeprecate.NewCommand("undeprecate", builder),
									repositoryupdate.NewCommand("update", builder),
								},
							},
							{
								Use:   "tag",
								Short: "Manage a repository's tags",
								SubCommands: []*appcmd.Command{
									tagcreate.NewCommand("create", builder),
									taglist.NewCommand("list", builder),
								},
							},
							{
								Use:   "commit",
								Short: "Manage a repository's commits",
								SubCommands: []*appcmd.Command{
									commitget.NewCommand("get", builder),
									commitlist.NewCommand("list", builder),
								},
							},
							{
								Use:   "draft",
								Short: "Manage a repository's drafts",
								SubCommands: []*appcmd.Command{
									draftdelete.NewCommand("delete", builder),
									draftlist.NewCommand("list", builder),
								},
							},
							{
								Use:   "webhook",
								Short: "Manage webhooks for a repository on the Buf Schema Registry",
								SubCommands: []*appcmd.Command{
									webhookcreate.NewCommand("create", builder),
									webhookdelete.NewCommand("delete", builder),
									webhooklist.NewCommand("list", builder),
								},
							},
							{
								Use:   "plugin",
								Short: "Manage plugins on the Buf Schema Registry",
								SubCommands: []*appcmd.Command{
									pluginpush.NewCommand("push", builder),
									plugindelete.NewCommand("delete", builder),
								},
							},
						},
					},
				},
			},
			{
				Use:    "alpha",
				Short:  "Alpha commands. Unstable and recommended only for experimentation. These may be deleted",
				Hidden: true,
				SubCommands: []*appcmd.Command{
					protoc.NewCommand("protoc", builder),
					{
						Use:   "registry",
						Short: "Manage assets on the Buf Schema Registry",
						SubCommands: []*appcmd.Command{
							{
								Use:   "token",
								Short: "Manage user tokens",
								SubCommands: []*appcmd.Command{
									tokenget.NewCommand("get", builder),
									tokenlist.NewCommand("list", builder),
									tokendelete.NewCommand("delete", builder),
								},
							},
						},
					},
					{
						Use:   "sdk",
						Short: "Manage Generated SDKs",
						SubCommands: []*appcmd.Command{
							goversion.NewCommand("go-version", builder),
							mavenversion.NewCommand("maven-version", builder),
							npmversion.NewCommand("npm-version", builder),
							swiftversion.NewCommand("swift-version", builder),
							pythonversion.NewCommand("python-version", builder),
						},
					},
					//{
					//Use:   "repo",
					//Short: "Manage Git repositories",
					//SubCommands: []*appcmd.Command{
					//reposync.NewCommand("sync", builder),
					//},
					//},
				},
			},
		},
	}
}

// newErrorInterceptor returns a CLI interceptor that wraps Buf CLI errors.
func newErrorInterceptor() appext.Interceptor {
	return func(next func(context.Context, appext.Container) error) func(context.Context, appext.Container) error {
		return func(ctx context.Context, container appext.Container) error {
			return wrapError(next(ctx, container))
		}
	}
}

// wrapError is used when a CLI command fails, regardless of its error code.
// Note that this function will wrap the error so that the underlying error
// can be recovered via 'errors.Is'.
func wrapError(err error) error {
	if err == nil {
		return nil
	}

	var connectErr *connect.Error
	isConnectError := errors.As(err, &connectErr)
	// If error is empty and not a system error or Connect error, we return it as-is.
	if !isConnectError && err.Error() == "" {
		return err
	}
	if isConnectError {
		connectCode := connectErr.Code()
		switch {
		case connectCode == connect.CodeUnauthenticated, isEmptyUnknownError(err):
			if authErr, ok := bufconnect.AsAuthError(err); ok && authErr.TokenEnvKey() != "" {
				return fmt.Errorf("Failure: the %[1]s environment variable is set, but is not valid. "+
					"Set %[1]s to a valid Buf API key, or unset it. For details, "+
					"visit https://docs.buf.build/bsr/authentication", authErr.TokenEnvKey())
			}
			return errors.New("Failure: you are not authenticated. Create a new entry in your netrc, " +
				"using a Buf API Key as the password. If you already have an entry in your netrc, check " +
				"to see that your token is not expited. For details, visit https://docs.buf.build/bsr/authentication")
		case connectCode == connect.CodeUnavailable:
			msg := `Failure: the server hosted at that remote is unavailable.`
			// If the returned error is Unavailable, then determine if this is a DNS error.  If so,
			// get the address usedso that we can display a more helpful error message.
			if dnsError := (&net.DNSError{}); errors.As(err, &dnsError) && dnsError.IsNotFound {
				return fmt.Errorf(`%s Are you sure "%s" is a valid remote address?`, msg, dnsError.Name)
			}
			// If the unavailable error wraps a tls.CertificateVerificationError, show a more specific
			// error message to the user to aid in troubleshooting.
			if tlsErr := wrappedTLSError(err); tlsErr != nil {
				return fmt.Errorf("tls certificate verification: %w", tlsErr)
			}
			return errors.New(msg)
		}
		err = connectErr.Unwrap()
	}

	sysError, isSysError := syserror.As(err)
	if isSysError {
		err = fmt.Errorf(
			"it looks like you have found a bug in buf. "+
				"Please file an issue at https://github.com/bufbuild/buf/issues "+
				"and provide the command you ran, as well as the following message: %w",
			sysError.Unwrap(),
		)
	}

	var importNotExistError *bufmodule.ImportNotExistError
	if errors.As(err, &importNotExistError) {
		// There must be a better place to do this, perhaps in the Controller, but this works for now.
		err = app.WrapError(bufctl.ExitCodeFileAnnotation, importNotExistError)
	}

	return fmt.Errorf("Failure: %w", err)
}

// isEmptyUnknownError returns true if the given
// error is non-nil, but has an empty message
// and an unknown error code.
//
// This is relevant for errors returned by
// envoyauthd when the client does not provide
// an authentication header.
func isEmptyUnknownError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "" && connect.CodeOf(err) == connect.CodeUnknown
}
