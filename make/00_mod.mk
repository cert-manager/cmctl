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

repo_name := github.com/cert-manager/ctl

build_names := ctl

go_ctl_source_path := main.go
go_ctl_ldflags := -X $(repo_name)/pkg/internal/version.AppVersion=$(VERSION) -X $(repo_name)/pkg/internal/version.GitCommit=$(GITCOMMIT)
oci_ctl_base_image_flavor := static
oci_ctl_image_name := quay.io/jetstack/cert-manager-ctl
oci_ctl_image_tag := $(VERSION)
oci_ctl_image_name_development := cert-manager.local/cert-manager-ctl
