# Copyright 2023 The cert-manager Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

repo_name := github.com/cert-manager/cmctl

exe_build_names := cmctl kubectl_cert-manager
gorelease_file := .goreleaser.yml

go_cmctl_source_path := main.go
go_cmctl_ldflags := \
	-X $(repo_name)/pkg/build.name=cmctl \
	-X $(repo_name)/pkg/build/commands.registerCompletion=true \
	-X github.com/cert-manager/cert-manager/pkg/util.AppVersion=$(VERSION) \
	-X github.com/cert-manager/cert-manager/pkg/util.AppGitCommit=$(GITCOMMIT)

go_kubectl_cert-manager_source_path := main.go
go_kubectl_cert-manager_ldflags := \
	-X $(repo_name)/pkg/build.name=kubectl \
	-X $(repo_name)/pkg/build/commands.registerCompletion=false \
	-X github.com/cert-manager/cert-manager/pkg/util/version.AppVersion=$(VERSION) \
	-X github.com/cert-manager/cert-manager/pkg/util/version.AppGitCommit=$(GITCOMMIT)
