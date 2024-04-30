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

package completion

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/cert-manager/cmctl/v2/pkg/build"
)

func newCmdCompletionKubectl(setupCtx context.Context, ioStreams genericclioptions.IOStreams) *cobra.Command {
	return &cobra.Command{
		Use:   "kubectl",
		Short: "Generate cert-manager CLI scripts for Kubectl",
		Long: build.WithTemplate(setupCtx, `To load completions:
$ {{.BuildName}} completion kubectl > kubectl_complete-cert_manager
$ sudo install kubectl_complete-cert_manager /usr/local/bin
`),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(ioStreams.Out, `#!/usr/bin/env sh
kubectl cert-manager __complete "$@"
`)
			return err
		},
	}
}
