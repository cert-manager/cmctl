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

package ctl

import (
	"context"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cert-manager/cmctl/v2/test/integration/install_framework"
)

func TestCtlUninstall(t *testing.T) {
	tests := map[string]struct {
		prerun       bool
		prehelm      bool
		preInputArgs []string
		preExpErr    bool
		preExpOutput string

		inputArgs []string
		expErr    bool
		expOutput string

		didInstallCRDs bool
	}{
		"install and uninstall cert-manager": {
			prerun:       true,
			preInputArgs: []string{"x", "install", "--wait=false"},
			preExpErr:    false,
			preExpOutput: `STATUS: deployed`,

			inputArgs: []string{"x", "uninstall", "--wait=false"},
			expErr:    false,
			expOutput: `release "cert-manager" uninstalled`,

			didInstallCRDs: true,
		},
		"uninstall cert-manager installed by helm": {
			prehelm: true,
			preInputArgs: []string{
				"install", "cert-manager", "cert-manager",
				"--repo=https://charts.jetstack.io",
				"--namespace=cert-manager",
				"--create-namespace",
				"--version=v1.12.0",
				"--set=installCRDs=true",
				"--wait=false",
				"--no-hooks",
			},
			preExpErr:    false,
			preExpOutput: `STATUS: deployed`,

			inputArgs: []string{"x", "uninstall", "--wait=false"},
			expErr:    false,
			expOutput: `These resources were kept due to the resource policy:`,

			didInstallCRDs: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			testApiServer, cleanup := install_framework.NewTestInstallApiServer(t)
			defer cleanup()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*40)
			defer cancel()

			if test.prerun {
				executeCmctlAndCheckOutput(
					t, ctx, testApiServer.KubeConfigFilePath(),
					test.preInputArgs,
					test.preExpErr,
					test.preExpOutput,
				)
			} else if test.prehelm {
				executeHelmAndCheckOutput(
					t, ctx, testApiServer.KubeConfigFilePath(),
					test.preInputArgs,
					test.preExpErr,
					test.preExpOutput,
				)
			}

			executeCmctlAndCheckOutput(
				t, ctx, testApiServer.KubeConfigFilePath(),
				test.inputArgs,
				test.expErr,
				test.expOutput,
			)

			// if we installed CRDs, check that they were not deleted
			if test.didInstallCRDs {
				clientset, err := apiextensionsv1.NewForConfig(testApiServer.RestConfig())
				require.NoError(t, err)

				_, err = clientset.CustomResourceDefinitions().Get(ctx, "certificates.cert-manager.io", metav1.GetOptions{})
				require.NoError(t, err)
			}
		})
	}
}

func executeHelmAndCheckOutput(
	t *testing.T,
	ctx context.Context,
	kubeConfig string,
	inputArgs []string,
	expErr bool,
	expOutput string,
) {
	// find Helm binary
	helmBinPath, ok := os.LookupEnv("HELM_BIN")
	if !ok {
		t.Fatal("HELM_BIN environment variable not set")
	}

	cmd := exec.CommandContext(ctx, helmBinPath, inputArgs...)

	// Set empty environment variables except for KUBECONFIG and HOME environment variable
	cmd.Env = []string{"KUBECONFIG=" + kubeConfig, "HOME=" + os.Getenv("HOME")}

	executeAndCheckOutput(t, func(stdin io.Reader, stdout io.Writer) error {
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stdout

		return cmd.Run()
	}, expErr, expOutput)
}
