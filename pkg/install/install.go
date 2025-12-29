/*
Copyright 2021 The cert-manager Authors.

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

package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	logf "github.com/cert-manager/cert-manager/pkg/logs"
	"github.com/spf13/cobra"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/cert-manager/cmctl/v2/pkg/build"
	"github.com/cert-manager/cmctl/v2/pkg/install/helm"
)

type InstallOptions struct {
	settings  *helm.NormalisedEnvSettings
	client    *action.Install
	valueOpts *values.Options

	ChartName string
	DryRun    bool
	Wait      bool

	genericclioptions.IOStreams
}

const (
	installCRDsFlagName = "installCRDs"
)

func installDesc(ctx context.Context) string {
	return build.WithTemplate(ctx, `This command installs cert-manager. It uses the Helm libraries to do so.

The latest published cert-manager chart in the "https://charts.jetstack.io" repo is used.
Most of the features supported by 'helm install' are also supported by this command.
In addition, this command will always correctly install the required CRD resources.

Some example uses:
	$ {{.BuildName}} x install
or
	$ {{.BuildName}} x install -n new-cert-manager
or
	$ {{.BuildName}} x install --version v1.4.0
or
	$ {{.BuildName}} x install --set prometheus.enabled=false

To override values in the cert-manager chart, use either the '--values' flag and
pass in a file or use the '--set' flag and pass configuration from the command line.
`)
}

func NewCmdInstall(setupCtx context.Context, ioStreams genericclioptions.IOStreams) *cobra.Command {
	settings := helm.NewNormalisedEnvSettings()

	options := &InstallOptions{
		settings:  settings,
		client:    action.NewInstall(settings.ActionConfiguration),
		valueOpts: &values.Options{},

		IOStreams: ioStreams,
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install cert-manager",
		Long:  installDesc(setupCtx),
		// nolint:contextcheck // False positive
		RunE: func(cmd *cobra.Command, args []string) error {
			options.client.Namespace = settings.Namespace()

			rel, err := options.runInstall(cmd.Context())
			if err != nil {
				return err
			}
			rac, err := release.NewAccessor(rel)
			if err != nil {
				return err
			}

			if options.DryRun {
				fmt.Fprintf(ioStreams.Out, "%s", rac.Manifest())
				return nil
			}

			printReleaseSummary(ioStreams.Out, rac)
			return nil
		},
	}

	settings.Setup(setupCtx, cmd)

	helm.AddInstallUninstallFlags(cmd.Flags(), &options.client.Timeout, &options.Wait)

	addInstallFlags(cmd.Flags(), options.client)
	addValueOptionsFlags(cmd.Flags(), options.valueOpts)
	addChartPathOptionsFlags(cmd.Flags(), &options.client.ChartPathOptions)

	cmd.Flags().BoolVar(&options.client.CreateNamespace, "create-namespace", true, "Create the release namespace if not present")
	if err := cmd.Flags().MarkHidden("create-namespace"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&options.ChartName, "chart-name", "cert-manager", "Name of the chart to install")
	if err := cmd.Flags().MarkHidden("chart-name"); err != nil {
		panic(err)
	}
	cmd.Flags().BoolVar(&options.DryRun, "dry-run", false, "Simulate install and output manifest")

	return cmd
}

// The overall strategy is to install the CRDs first, and not as part of a Helm
// release, and then to install a Helm release without the CRDs.  This is to
// ensure that CRDs are not removed by a subsequent helm uninstall or by a
// future cmctl uninstall. We want the removal of CRDs to only be performed by
// an administrator who understands that the consequences of removing CRDs will
// be the garbage collection of all the related CRs in the cluster.  We first
// do a dry-run install of the chart (effectively helm template
// --validate=false) to render the CRDs from the CRD templates in the Chart.
// The ClientOnly option is required, otherwise Helm will return an error in
// case the CRDs are already installed in the cluster.  We then extract the
// CRDs from the resulting dry-run manifests and install those first.  Finally,
// we perform a helm install to install the remaining non-CRD resources and
// wait for those to be "Ready".
// This creates a Helm "release" artifact in a Secret in the target namespace, which contains
// a record of all the resources installed by Helm (except the CRDs).
func (o *InstallOptions) runInstall(ctx context.Context) (release.Releaser, error) {
	log := logf.FromContext(ctx, "install")

	// Find chart
	cp, err := o.client.ChartPathOptions.LocateChart(o.ChartName, o.settings.EnvSettings)
	if err != nil {
		return nil, err
	}

	chrt, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}
	cac, err := chart.NewAccessor(chrt)
	if err != nil {
		return nil, err
	}

	// Check if chart is installable
	if err := checkIfInstallable(cac); err != nil {
		return nil, err
	}

	// Console print if chart is deprecated
	if cac.Deprecated() {
		log.Error(fmt.Errorf("chart.Metadata.Deprecated is true"), "This chart is deprecated")
	}

	// Merge all values flags
	p := getter.All(o.settings.EnvSettings)
	chartValues, err := o.valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	// Dryrun template generation (used for rendering the CRDs in /templates)
	o.client.DryRunStrategy = action.DryRunClient // Do not validate against cluster (otherwise double CRDs can cause error)
	// Kube version to be used in dry run template generation which does not
	// talk to kube apiserver. This is to ensure that template generation
	// does not fail because our Kubernetes minimum version requirement is
	// higher than that hardcoded in Helm codebase for client-only runs
	o.client.KubeVersion = &common.KubeVersion{
		Version: "v999.999.999",
		Major:   "999",
		Minor:   "999",
	}
	chartValues[installCRDsFlagName] = true // Make sure to render CRDs
	dryRunResult, err := o.client.Run(chrt, chartValues)
	if err != nil {
		return nil, err
	}

	if o.DryRun {
		return dryRunResult, nil
	}

	// The o.client.Run() call above will have altered the settings.ActionConfiguration
	// object, so we need to re-initialise it.
	if err := o.settings.InitActionConfiguration(); err != nil {
		return nil, err
	}
	rac, err := release.NewAccessor(dryRunResult)
	if err != nil {
		return nil, err
	}

	// Extract the resource.Info objects from the manifest
	resources, err := helm.ParseMultiDocumentYAML(rac.Manifest(), o.settings.ActionConfiguration.KubeClient)
	if err != nil {
		return nil, err
	}

	// Filter resource.Info objects and only keep the CRDs
	crds := helm.FilterCrdResources(resources)

	// Abort in case CRDs were not found in chart
	if len(crds) == 0 {
		return nil, fmt.Errorf("Found no CRDs in provided cert-manager chart.")
	}

	// Make sure that no CRDs are currently installed
	originalCRDs, err := helm.FetchResources(crds, o.settings.ActionConfiguration.KubeClient)
	if err != nil {
		return nil, err
	}

	if len(originalCRDs) > 0 {
		return nil, fmt.Errorf("Found existing installed cert-manager CRDs! Cannot continue with installation.")
	}

	// Install CRDs
	if err := helm.CreateCRDs(crds, o.settings.ActionConfiguration); err != nil {
		return nil, err
	}

	// Install chart
	o.client.DryRunStrategy = action.DryRunNone // Perform install against cluster
	o.client.KubeVersion = nil

	if o.Wait {
		o.client.WaitStrategy = kube.StatusWatcherStrategy // Wait for resources to be ready
		// If part of the installation fails and the RollbackOnFailure option is set to True,
		// all resource installs are reverted. RollbackOnFailure cannot be enabled without
		// waiting, so only enable RollbackOnFailure if we are waiting.
		o.client.RollbackOnFailure = true
	} else {
		o.client.WaitStrategy = kube.HookOnlyStrategy
		// The cert-manager chart currently has only a startupapicheck hook,
		// if waiting is disabled, this hook should be disabled too; otherwise
		// the hook will still wait for the installation to succeed.
		o.client.DisableHooks = true
	}

	chartValues[installCRDsFlagName] = false // Do not render CRDs, as this might cause problems when uninstalling using helm

	return o.client.Run(chrt, chartValues)
}

func printReleaseSummary(out io.Writer, rac release.Accessor) {
	fmt.Fprintf(out, "NAME: %s\n", rac.Name())
	if !rac.DeployedAt().IsZero() {
		fmt.Fprintf(out, "LAST DEPLOYED: %s\n", rac.DeployedAt().Format(time.ANSIC))
	}
	fmt.Fprintf(out, "NAMESPACE: %s\n", rac.Namespace())
	fmt.Fprintf(out, "STATUS: %s\n", rac.Status())
	fmt.Fprintf(out, "REVISION: %d\n", rac.Version())
	// Helm v4's release.Accessor does not expose a description field,
	// so we intentionally omit the DESCRIPTION line that was present in
	// earlier Helm versions to avoid printing an empty value.

	if len(rac.Notes()) > 0 {
		fmt.Fprintf(out, "NOTES:\n%s\n", strings.TrimSpace(rac.Notes()))
	}
}

// Only Application chart types are installable.
func checkIfInstallable(cac chart.Accessor) error {
	if cac.IsLibraryChart() {
		return errors.New("library charts are not installable")
	}
	return nil
}
