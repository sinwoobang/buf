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

package migrate

import (
	"context"

	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/buf/bufmigrate"
	"github.com/bufbuild/buf/private/bufpkg/bufapi"
	"github.com/bufbuild/buf/private/pkg/app/appcmd"
	"github.com/bufbuild/buf/private/pkg/app/appext"
	"github.com/bufbuild/buf/private/pkg/storage/storageos"
	"github.com/spf13/pflag"
)

const (
	workspaceDirectoriesFlagName = "workspace"
	moduleDirectoriesFlagName    = "module"
	bufGenYAMLPathFlagName       = "template"
	dryRunFlagName               = "dry-run"
)

// NewCommand returns a new Command.
func NewCommand(
	name string,
	builder appext.SubCommandBuilder,
) *appcmd.Command {
	flags := newFlags()
	return &appcmd.Command{
		Use:   name,
		Short: `Migrate configuration to the latest version`,
		Long:  `Migrate configuration files at the specified directories or paths to the latest version.`,
		Args:  appcmd.MaximumNArgs(0),
		Run: builder.NewRunFunc(
			func(ctx context.Context, container appext.Container) error {
				return run(ctx, container, flags)
			},
		),
		BindFlags: flags.Bind,
	}
}

type flags struct {
	WorkspaceDirPaths []string
	ModuleDirPaths    []string
	BufGenYAMLPaths   []string
	DryRun            bool
}

func newFlags() *flags {
	return &flags{}
}

func (f *flags) Bind(flagSet *pflag.FlagSet) {
	flagSet.StringSliceVar(
		&f.WorkspaceDirPaths,
		workspaceDirectoriesFlagName,
		nil,
		"The workspace directories to migrate. buf.work.yaml, buf.yamls and buf.locks will be migrated.",
	)
	flagSet.StringSliceVar(
		&f.ModuleDirPaths,
		moduleDirectoriesFlagName,
		nil,
		"The module directories to migrate. buf.yaml and buf.lock will be migrated",
	)
	flagSet.BoolVar(
		&f.DryRun,
		dryRunFlagName,
		false,
		"Print the changes to be made without writing to the disk",
	)
	flagSet.StringSliceVar(
		&f.BufGenYAMLPaths,
		bufGenYAMLPathFlagName,
		nil,
		"The paths to the generation templates to migrate",
	)
}

func run(
	ctx context.Context,
	container appext.Container,
	flags *flags,
) error {
	var migrateOptions []bufmigrate.MigrateOption
	if flags.DryRun {
		migrateOptions = append(migrateOptions, bufmigrate.MigrateAsDryRun())
	}
	clientConfig, err := bufcli.NewConnectClientConfig(container)
	if err != nil {
		return err
	}
	return bufmigrate.Migrate(
		ctx,
		container.Stderr(),
		storageos.NewProvider(storageos.ProviderWithSymlinks()),
		bufapi.NewClientProvider(clientConfig),
		flags.WorkspaceDirPaths,
		flags.ModuleDirPaths,
		flags.BufGenYAMLPaths,
		migrateOptions...,
	)
}
