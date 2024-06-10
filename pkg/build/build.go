/*
Copyright 2020 The cert-manager Authors.

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

package build

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var defaultCtlName string = "cmctl"
var defaultIsKubectlPlugin bool = false

func DetectCtlInfo(args []string) (name string, isKubectlPlugin bool) {
	commandName := filepath.Base(os.Args[0])
	if strings.HasPrefix(commandName, "kubectl-") || strings.HasPrefix(commandName, "kubectl_") {
		return "kubectl cert-manager", true
	}

	return commandName, false
}

// contextNameKey is how we find the ctl name in a context.Context.
type contextNameKey struct{}

// contextIsKubectlPluginKey is how we find if the ctl is a Kubectl plugin in a context.Context.
type contextIsKubectlPluginKey struct{}

func WithCtlInfo(ctx context.Context, name string, isKubectlPlugin bool) context.Context {
	ctx = context.WithValue(ctx, contextNameKey{}, name)
	ctx = context.WithValue(ctx, contextIsKubectlPluginKey{}, isKubectlPlugin)
	return ctx
}

func Name(ctx context.Context) string {
	name, ok := ctx.Value(contextNameKey{}).(string)
	if !ok {
		return defaultCtlName
	}

	return name
}

func IsKubectlPlugin(ctx context.Context) bool {
	isKubectlPlugin, ok := ctx.Value(contextIsKubectlPluginKey{}).(bool)
	if !ok {
		return defaultIsKubectlPlugin
	}

	return isKubectlPlugin
}

// WithTemplate returns a string that has the build name templated out with the
// configured build name. Build name templates on '{{ .BuildName }}' variable.
func WithTemplate(ctx context.Context, str string) string {
	buildName := Name(ctx)
	tmpl := template.Must(template.New("build-name").Parse(str))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ BuildName string }{buildName}); err != nil {
		// We panic here as it should never be possible that this template fails.
		panic(err)
	}
	return buf.String()
}
