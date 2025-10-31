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

exe_targets ?= darwin_amd64_v1,darwin_arm64,linux_amd64_v1,linux_arm_7,linux_arm64,linux_ppc64le,linux_s390x,windows_amd64_v1,windows_arm64

# Utility functions
fatal_if_undefined = $(if $(findstring undefined,$(origin $1)),$(error $1 is not set))

# Validate globals that are required
$(call fatal_if_undefined,bin_dir)
$(call fatal_if_undefined,exe_build_names)
$(call fatal_if_undefined,gorelease_file)

# Set default config values
CGO_ENABLED ?= 0
GOEXPERIMENT ?=  # empty by default

# Default variables per exe_build_names entry
#
# $1 - build_name
define default_per_build_variables
go_$1_cgo_enabled ?= $(CGO_ENABLED)
go_$1_goexperiment ?= $(GOEXPERIMENT)
go_$1_flags ?= -tags=
exe_$1_targets ?= $(exe_targets)
endef

$(foreach build_name,$(exe_build_names),$(eval $(call default_per_build_variables,$(build_name))))

# Validate variables per exe_build_names entry
#
# $1 - build_name
define check_per_build_variables
# Validate required config exists
$(call fatal_if_undefined,go_$1_ldflags)
$(call fatal_if_undefined,go_$1_main_dir)
$(call fatal_if_undefined,go_$1_mod_dir)

# Validate the config required to build the golang based executable
ifneq ($(go_$1_main_dir:.%=.),.)
$$(error go_$1_main_dir "$(go_$1_main_dir)" should be a directory path that DOES start with ".")
endif
ifeq ($(go_$1_main_dir:%/=/),/)
$$(error go_$1_main_dir "$(go_$1_main_dir)" should be a directory path that DOES NOT end with "/")
endif
ifeq ($(go_$1_main_dir:%.go=.go),.go)
$$(error go_$1_main_dir "$(go_$1_main_dir)" should be a directory path that DOES NOT end with ".go")
endif
ifneq ($(go_$1_mod_dir:.%=.),.)
$$(error go_$1_mod_dir "$(go_$1_mod_dir)" should be a directory path that DOES start with ".")
endif
ifeq ($(go_$1_mod_dir:%/=/),/)
$$(error go_$1_mod_dir "$(go_$1_mod_dir)" should be a directory path that DOES NOT end with "/")
endif
ifeq ($(go_$1_mod_dir:%.go=.go),.go)
$$(error go_$1_mod_dir "$(go_$1_mod_dir)" should be a directory path that DOES NOT end with ".go")
endif
ifeq ($(wildcard $(go_$1_mod_dir)/go.mod),)
$$(error go_$1_mod_dir "$(go_$1_mod_dir)" does not contain a go.mod file)
endif
ifeq ($(wildcard $(go_$1_mod_dir)/$(go_$1_main_dir)/main.go),)
$$(error go_$1_main_dir "$(go_$1_mod_dir)/$(go_$1_main_dir)" does not contain a main.go file)
endif
endef

$(foreach build_name,$(exe_build_names),$(eval $(call check_per_build_variables,$(build_name))))

# Dryrun release
RELEASE_DRYRUN ?= false
