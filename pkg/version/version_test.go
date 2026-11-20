/*
Copyright 2026 The cert-manager Authors.

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

package version

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestClientVersionWithoutKubeconfig(t *testing.T) {
	for _, config := range []string{"missing", "malformed"} {
		t.Run(config, func(t *testing.T) {
			kubeconfig := filepath.Join(t.TempDir(), "config")
			if config == "malformed" {
				if err := os.WriteFile(kubeconfig, []byte("invalid: ["), 0600); err != nil {
					t.Fatal(err)
				}
			}

			for _, output := range []struct {
				name string
				args []string
				want string
			}{
				{name: "default", want: "Client Version:"},
				{name: "short", args: []string{"--short"}, want: "Client Version:"},
				{name: "json", args: []string{"--output=json"}, want: `"clientVersion"`},
				{name: "yaml", args: []string{"--output=yaml"}, want: "clientVersion:"},
			} {
				t.Run(output.name, func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					cmd := NewCmdVersion(t.Context(), genericclioptions.IOStreams{Out: &stdout, ErrOut: &stderr})
					cmd.SetArgs(append([]string{"--client", "--kubeconfig", kubeconfig}, output.args...))
					cmd.SetOut(&stdout)
					cmd.SetErr(&stderr)

					if err := cmd.Execute(); err != nil {
						t.Fatalf("client-only version failed: %v", err)
					}
					if !strings.Contains(stdout.String(), output.want) {
						t.Fatalf("expected client version output containing %q, got %s", output.want, stdout.String())
					}
					if strings.Contains(stdout.String(), "Server Version") || strings.Contains(stdout.String(), "serverVersion") {
						t.Fatalf("client-only output contains a server version: %s", stdout.String())
					}
				})
			}
		})
	}
}

func TestClientVersionValidatesOutput(t *testing.T) {
	cmd := NewCmdVersion(t.Context(), genericclioptions.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"--client", "--kubeconfig", filepath.Join(t.TempDir(), "missing"), "--output", "invalid"})

	if err := cmd.Execute(); err == nil || err.Error() != "--output must be '', 'yaml' or 'json'" {
		t.Fatalf("expected output validation error, got %v", err)
	}
}

func TestServerVersionRequiresKubeconfig(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "missing")
	cmd := NewCmdVersion(t.Context(), genericclioptions.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"--kubeconfig", kubeconfig})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), kubeconfig) {
		t.Fatalf("expected kubeconfig loading error, got %v", err)
	}
}
