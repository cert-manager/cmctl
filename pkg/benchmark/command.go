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

func description() string {
	return build.WithTemplate(`This command runs a cert-manager benchmark.

Some example uses:
	$ {{.BuildName}} x benchmark
`)
}

type options struct {
	genericiooptions.IOStreams
	*factory.Factory

	measurementInterval          time.Duration
	rampUpTargetCertificateCount int64
	steadyStateDuration          time.Duration
	finalMeasurementsDuration    time.Duration
}

func NewCmd(ctx context.Context, ioStreams genericiooptions.IOStreams) *cobra.Command {
	options := options{
		IOStreams: ioStreams,
	}

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "benchmark cert-manager",
		Long:  description(),
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

	cmd.Flags().Int64Var(&options.rampUpTargetCertificateCount, "benchmark.phases.ramp-up.target-certificate-count", 2000,
		"The number of Certificate resources to create during the ramp-up phase.")

	cmd.Flags().DurationVar(&options.steadyStateDuration, "benchmark.phases.steady-state.duration", time.Minute*10,
		"The duration of the steady-state phase.")

	cmd.Flags().DurationVar(&options.finalMeasurementsDuration, "benchmark.phases.final-measurements.duration", time.Minute*2,
		"The duration of the final-measurements phase.")

	options.Factory = factory.New(cmd)
	return cmd
}
