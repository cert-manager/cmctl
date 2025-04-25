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
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
	logsapi "k8s.io/component-base/logs/api/v1"

	"github.com/cert-manager/cmctl/v2/cmd"
	"github.com/cert-manager/cmctl/v2/test/integration/install_framework"
)

func TestCtlInstall(t *testing.T) {
	tests := map[string]struct {
		prerun       bool
		preInputArgs []string
		preExpErr    bool
		preExpOutput string

		inputArgs []string
		expErr    bool
		expOutput string
	}{
		"install cert-manager": {
			inputArgs: []string{"x", "install", "--wait=false"},
			expErr:    false,
			expOutput: `STATUS: deployed`,
		},
		"install cert-manager (already installed)": {
			prerun:       true,
			preInputArgs: []string{"x", "install", "--wait=false"},
			preExpErr:    false,
			preExpOutput: `STATUS: deployed`,

			inputArgs: []string{"x", "install", "--wait=false"},
			expErr:    true,
			expOutput: `^Found existing installed cert-manager CRDs! Cannot continue with installation.$`,
		},
		"install cert-manager (already installed, in other namespace)": {
			prerun:       true,
			preInputArgs: []string{"x", "install", "--wait=false", "--namespace=test"},
			preExpErr:    false,
			preExpOutput: `STATUS: deployed`,

			inputArgs: []string{"x", "install", "--wait=false"},
			expErr:    true,
			expOutput: `^Found existing installed cert-manager CRDs! Cannot continue with installation.$`,
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
			}

			executeCmctlAndCheckOutput(
				t, ctx, testApiServer.KubeConfigFilePath(),
				test.inputArgs,
				test.expErr,
				test.expOutput,
			)
		})
	}
}

func executeAndCheckOutput(
	t *testing.T,
	f func(io.Reader, io.Writer) error,
	expErr bool,
	expOutput string,
) {
	stdin := bytes.NewBufferString("")
	stdout := bytes.NewBufferString("")

	err := f(stdin, stdout)
	if err != nil {
		fmt.Fprintf(stdout, "%s\n", err)

		if !expErr {
			t.Errorf("got unexpected error: %v", err)
		} else {
			t.Logf("got an error, which was expected, details: %v", err)
		}
	} else if expErr {
		// expected error but error is nil
		t.Errorf("expected but got no error")
	}

	match, err := regexp.MatchString(strings.TrimSpace(expOutput), strings.TrimSpace(stdout.String()))
	if err != nil {
		t.Error(err)
	}
	dmp := diffmatchpatch.New()
	if !match {
		diffs := dmp.DiffMain(strings.TrimSpace(expOutput), strings.TrimSpace(stdout.String()), false)
		t.Errorf(
			"got unexpected output, diff (ignoring line anchors ^ and $ and regex for creation time):\n"+
				"diff: %s\n\n"+
				" exp: %s\n\n"+
				" got: %s",
			dmp.DiffPrettyText(diffs),
			expOutput,
			stdout.String(),
		)
	}
}

func executeCmctlAndCheckOutput(
	t *testing.T,
	ctx context.Context,
	kubeConfig string,
	inputArgs []string,
	expErr bool,
	expOutput string,
) {
	if err := logsapi.ResetForTest(nil); err != nil {
		t.Fatal(err)
	}

	executeAndCheckOutput(t, func(stdin io.Reader, stdout io.Writer) error {
		cmd := cmd.NewCertManagerCtlCommand(ctx, stdin, stdout, stdout)
		cmd.SetArgs(append([]string{fmt.Sprintf("--kubeconfig=%s", kubeConfig)}, inputArgs...))

		return cmd.Execute()
	}, expErr, expOutput)
}
