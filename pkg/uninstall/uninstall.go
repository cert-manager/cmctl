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
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
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

func description() string {
	return build.WithTemplate(`This command safely uninstalls any Helm-managed release of cert-manager.

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
		Long:  description(),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := run(cmd.Context(), options)
			if err != nil {
				return err
			}

			if options.dryRun {
				fmt.Fprintf(ioStreams.Out, "%s", res.Release.Manifest)
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
	o.client.Wait = o.wait
	if o.client.Wait {
		o.client.DeletionPropagation = "foreground"
	} else {
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

func addCRDAnnotations(ctx context.Context, o options) error {
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

	if lastRelease.Info.Status != release.StatusDeployed {
		return fmt.Errorf("release %v is in a non-deployed state: %v", o.releaseName, lastRelease.Info.Status)
	}

	const (
		customResourceDefinitionApiVersionV1      = "apiextensions.k8s.io/v1"
		customResourceDefinitionApiVersionV1Beta1 = "apiextensions.k8s.io/v1beta1"
		customResourceDefinitionKind              = "CustomResourceDefinition"
	)

	// Check if the release manifest contains CRDs. If it does, we need to modify the
	// release manifest to add the "helm.sh/resource-policy: keep" annotation to the CRDs.
	manifests := releaseutil.SplitManifests(lastRelease.Manifest)
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
		manifestNames := releaseutil.BySplitManifestsOrder(maps.Keys(manifests))
		sort.Sort(manifestNames)
		var fullManifest strings.Builder
		for _, manifest := range manifestNames {
			fullManifest.WriteString(manifests[manifest])
			fullManifest.WriteString("\n---\n")
		}

		lastRelease.Manifest = fullManifest.String()

		if err := o.settings.ActionConfiguration.Releases.Update(lastRelease); err != nil {
			o.settings.ActionConfiguration.Log("uninstall: Failed to store updated release: %s", err)
		}
	}

	return nil
}
