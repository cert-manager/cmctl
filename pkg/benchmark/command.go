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

package benchmark

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/cert-manager/cmctl/v2/pkg/build"
	"github.com/cert-manager/cmctl/v2/pkg/factory"
)

const (
	label = "benchmark.cmctl.cert-manager.io/experiment"
)

const description = `
This command runs a cert-manager benchmark which stress tests the cert-manager
components and measures their CPU and memory.

The default benchmark takes ~50 minutes with the default installation of cert-manager.
There are five phases:

1. ramp-up (~8 minutes)

   Creates 2000 self-signed RSA(4096) Certificate resources spread across 200 namespaces;
   1 self-signed Issuer per namespace.
   Certificates are created in batches: 1 namespace, 1 issuer, 10 Certificates.
   All the benchmark resources are labelled: 'benchmark.cmctl.cert-manager.io/experiment=true'
   so that they can easily be identified and deleted afterwards.

2. catch-up (~26 minutes)

   Waits for cert-manager to reconcile all 2000 Certificates.

3. steady-state (~10 minutes)

   Continues to measure the cert-manager CPU and memory consumption for 10 minutes.

4. cleanup (~3 minutes)

   Deletes all 2000 Certificates and other benchmark resources.
   The benchmark namespaces are deleted in batches of 10 per second.

5. final-measurements (~2 minutes)

   Continues to measure the cert-manager CPU and memory consumption for 2 minutes.

Example:
    kind create cluster

    # Install metrics-server which is required for measuring cert-manager resource usage
    helm upgrade metrics-server metrics-server \
      --repo https://kubernetes-sigs.github.io/metrics-server/ \
      --install \
      --namespace kube-system \
      --set args={--kubelet-insecure-tls}

    # Install cert-manager
    {{.BuildName}} x install

    {{.BuildName}} x benchmark > data.json
`

type options struct {
	genericiooptions.IOStreams
	*factory.Factory

	measurementInterval  time.Duration
	certManagerNamepsace string

	rampUpLoadInterval           time.Duration
	rampUpCertificateAlgorithm   string
	rampUpCertificateSize        int
	rampUpTargetCertificateCount int64
	steadyStateDuration          time.Duration
	cleanupDisabled              bool
	finalMeasurementsDuration    time.Duration
}

func NewCmd(ctx context.Context, ioStreams genericiooptions.IOStreams) *cobra.Command {
	options := options{
		IOStreams: ioStreams,
	}

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "benchmark cert-manager",
		Long:  build.WithTemplate(description),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := experiment{
				options:      options,
				measurements: newMeasurements(options),
			}
			if err := e.run(cmd.Context()); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&options.measurementInterval, "benchmark.measurement-interval", time.Second*10,
		"The interval between measurements.")

	cmd.Flags().StringVar(&options.certManagerNamepsace, "benchmark.cert-manager-namespace", "cert-manager",
		"The namespace where cert-manager is installed.")

	cmd.Flags().DurationVar(&options.rampUpLoadInterval, "benchmark.phase1.load-interval", time.Second,
		"The private key algorithm of Certificate resources created during the ramp-up phase: RSA, ECDSA")

	cmd.Flags().StringVar(&options.rampUpCertificateAlgorithm, "benchmark.phase1.certificate-algorithm", "RSA",
		"The private key algorithm of Certificate resources created during the ramp-up phase: RSA, ECDSA")

	cmd.Flags().IntVar(&options.rampUpCertificateSize, "benchmark.phase1.certificate-size", 4096,
		"The private key size of Certificate resources created during the ramp-up phase.")

	cmd.Flags().Int64Var(&options.rampUpTargetCertificateCount, "benchmark.phase1.target-certificate-count", 2000,
		"The number of Certificate resources to create during the ramp-up phase.")

	cmd.Flags().DurationVar(&options.steadyStateDuration, "benchmark.phase3.duration", time.Minute*10,
		"The duration of the steady-state phase.")

	cmd.Flags().BoolVar(&options.cleanupDisabled, "benchmark.phase4.disabled", false,
		"Disable the cleanup phase.")

	cmd.Flags().DurationVar(&options.finalMeasurementsDuration, "benchmark.phase5.duration", time.Minute*2,
		"The duration of the final-measurements phase.")

	options.Factory = factory.New(cmd)
	return cmd
}
