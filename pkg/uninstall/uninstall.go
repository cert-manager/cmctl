/*
Copyright 2022 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package uninstall

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/cert-manager/cmctl/v2/pkg/build"
	"github.com/cert-manager/cmctl/v2/pkg/install/helm"
)

type options struct {
	settings *helm.NormalisedEnvSettings
	client   *action.Uninstall

	releaseName string
	dryRun      bool
	wait        bool

	genericclioptions.IOStreams
}

const (
	releaseName = "cert-manager"
)

func description() string {
	return build.WithTemplate(`This command uninstalls any Helm-managed release of cert-manager.

The CRDs will be deleted if you installed cert-manager with the option --set CRDs=true.

Most of the features supported by 'helm uninstall' are also supported by this command.

Some example uses:
	$ {{.BuildName}} x uninstall
or
	$ {{.BuildName}} x uninstall --namespace my-cert-manager
or
	$ {{.BuildName}} x uninstall --dry-run
or
	$ {{.BuildName}} x uninstall --no-hooks
`)
}

func NewCmd(ctx context.Context, ioStreams genericclioptions.IOStreams) *cobra.Command {
	settings := helm.NewNormalisedEnvSettings()

	options := options{
		settings: settings,
		client:   action.NewUninstall(settings.ActionConfiguration),

		IOStreams: ioStreams,
	}

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall cert-manager",
		Long:  description(),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := run(ctx, options)
			if err != nil {
				return err
			}

			if options.dryRun {
				fmt.Fprintf(ioStreams.Out, "%s", res.Release.Manifest)
				return nil
			}

			fmt.Fprintf(ioStreams.Out, "release \"%s\" uninstalled\n", options.releaseName)
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	settings.Setup(ctx, cmd)

	helm.AddInstallUninstallFlags(cmd.Flags(), &options.client.Timeout, &options.wait)

	cmd.Flags().StringVar(&options.releaseName, "release-name", releaseName, "name of the helm release to uninstall")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "simulate uninstall and output manifests to be deleted")

	return cmd
}

// run assumes cert-manager was installed as a Helm release named cert-manager.
// this is not configurable to avoid uninstalling non-cert-manager releases.
func run(ctx context.Context, o options) (*release.UninstallReleaseResponse, error) {
	// The cert-manager Helm chart currently does not have any uninstall hooks.
	o.client.DisableHooks = false
	o.client.DryRun = o.dryRun
	o.client.Wait = o.wait

	res, err := o.client.Run(o.releaseName)

	if errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, fmt.Errorf("release %v not found in namespace %v, did you use the correct namespace?", releaseName, o.settings.Namespace())
	}

	return res, nil
}
