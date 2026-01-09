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
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"helm.sh/helm/v4/pkg/action"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	v1release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
	"helm.sh/helm/v4/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"

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

func description(ctx context.Context) string {
	return build.WithTemplate(ctx, `This command safely uninstalls any Helm-managed release of cert-manager.

This command is safe because it will not delete any of the cert-manager CRDs even if they were
installed as part of the Helm release. This is to avoid accidentally deleting CRDs and custom resources.
This feature is why this command should always be used instead of 'helm uninstall'.

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

func NewCmd(setupCtx context.Context, ioStreams genericclioptions.IOStreams) *cobra.Command {
	settings := helm.NewNormalisedEnvSettings()

	options := options{
		settings: settings,
		client:   action.NewUninstall(settings.ActionConfiguration),

		IOStreams: ioStreams,
	}

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall cert-manager",
		Long:  description(setupCtx),
		// nolint:contextcheck // False positive
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := run(cmd.Context(), options)
			if err != nil {
				return err
			}

			if options.dryRun {
				if res.Release == nil {
					return errors.New("res.Release must not be nil")
				}

				rac, err := release.NewAccessor(res.Release)
				if err != nil {
					return err
				}
				fmt.Fprintf(ioStreams.Out, "%s", rac.Manifest())
				return nil
			}

			if res != nil && res.Info != "" {
				fmt.Fprintln(ioStreams.Out, res.Info)
			}

			fmt.Fprintf(ioStreams.Out, "release \"%s\" uninstalled\n", options.releaseName)
			return nil
		},
	}

	settings.Setup(setupCtx, cmd)

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
	if o.wait {
		o.client.WaitStrategy = kube.StatusWatcherStrategy
		o.client.DeletionPropagation = "foreground"
	} else {
		o.client.WaitStrategy = kube.HookOnlyStrategy // we don't have any uninstall hooks
		o.client.DeletionPropagation = "background"
	}
	o.client.KeepHistory = false
	o.client.IgnoreNotFound = true

	if !o.client.DryRun {
		if err := addCRDAnnotations(ctx, o); err != nil {
			return nil, err
		}
	}

	res, err := o.client.Run(o.releaseName)

	if errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, fmt.Errorf("release %v not found in namespace %v, did you use the correct namespace?", releaseName, o.settings.Namespace())
	}

	return res, nil
}

func addCRDAnnotations(_ context.Context, o options) error {
	if err := o.settings.ActionConfiguration.KubeClient.IsReachable(); err != nil {
		return err
	}

	if err := chartutil.ValidateReleaseName(o.releaseName); err != nil {
		return fmt.Errorf("uninstall: %v", err)
	}

	lastRelease, err := o.settings.ActionConfiguration.Releases.Last(o.releaseName)
	if err != nil {
		return fmt.Errorf("uninstall: %v", err)
	}

	rac, err := release.NewAccessor(lastRelease)
	if err != nil {
		return err
	}

	if rac.Status() != common.StatusDeployed.String() {
		return fmt.Errorf("release %v is in a non-deployed state: %v", o.releaseName, rac.Status())
	}

	const (
		customResourceDefinitionApiVersionV1      = "apiextensions.k8s.io/v1"
		customResourceDefinitionApiVersionV1Beta1 = "apiextensions.k8s.io/v1beta1"
		customResourceDefinitionKind              = "CustomResourceDefinition"
	)

	// Check if the release manifest contains CRDs. If it does, we need to modify the
	// release manifest to add the "helm.sh/resource-policy: keep" annotation to the CRDs.
	manifests := releaseutil.SplitManifests(rac.Manifest())
	foundNonAnnotatedCRD := false
	for key, manifest := range manifests {
		var entry releaseutil.SimpleHead
		if err := yaml.Unmarshal([]byte(manifest), &entry); err != nil {
			return fmt.Errorf("failed to unmarshal manifest: %v", err)
		}

		if entry.Kind != customResourceDefinitionKind || (entry.Version != customResourceDefinitionApiVersionV1 &&
			entry.Version != customResourceDefinitionApiVersionV1Beta1) {
			continue
		}

		if entry.Metadata != nil && entry.Metadata.Annotations != nil && entry.Metadata.Annotations["helm.sh/resource-policy"] == "keep" {
			continue
		}

		foundNonAnnotatedCRD = true

		var object unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(manifest), &object); err != nil {
			return fmt.Errorf("failed to unmarshal manifest: %v", err)
		}

		annotations := object.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["helm.sh/resource-policy"] = "keep"
		object.SetAnnotations(annotations)

		updatedManifestJSON, err := object.MarshalJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal manifest: %v", err)
		}

		updatedManifest, err := yaml.JSONToYAML(updatedManifestJSON)
		if err != nil {
			return fmt.Errorf("failed to convert manifest to YAML: %v", err)
		}

		manifests[key] = string(updatedManifest)
	}

	if foundNonAnnotatedCRD {
		manifestNames := releaseutil.BySplitManifestsOrder(slices.Collect(maps.Keys(manifests)))
		sort.Sort(manifestNames)
		var fullManifest strings.Builder
		for _, manifest := range manifestNames {
			fullManifest.WriteString(manifests[manifest])
			fullManifest.WriteString("\n---\n")
		}

		lastRelease, err = setManifest(lastRelease, fullManifest.String())
		if err != nil {
			return err
		}

		if err := o.settings.ActionConfiguration.Releases.Update(lastRelease); err != nil {
			o.settings.ActionConfiguration.Logger().Error(
				"uninstall: Failed to store updated release",
				"err", err,
			)
		}
	}

	return nil
}

// similar to the code in Helm (https://github.com/helm/helm/blob/c3a0d3b86025d2a06e370b9f12bf1e593418b45a/pkg/action/uninstall.go#L102-L126)
// we first convert the releaser to a v1 release to be able to set the manifest
func setManifest(release release.Releaser, manifest string) (release.Releaser, error) {
	switch r := release.(type) {
	case v1release.Release:
		r.Manifest = manifest
		return r, nil
	case *v1release.Release:
		r.Manifest = manifest
		return r, nil
	default:
		return nil, fmt.Errorf("unsupported release type: %T", release)
	}
}
