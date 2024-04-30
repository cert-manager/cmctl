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

repo_name := github.com/cert-manager/cmctl/v2

exe_build_names := cmctl
gorelease_file := .goreleaser.yml

go_cmctl_main_dir := .
go_cmctl_mod_dir := .
go_cmctl_ldflags := \
	-X github.com/cert-manager/cert-manager/pkg/util.AppVersion=$(VERSION) \
	-X github.com/cert-manager/cert-manager/pkg/util.AppGitCommit=$(GITCOMMIT)

golangci_lint_config := .golangci.yaml
