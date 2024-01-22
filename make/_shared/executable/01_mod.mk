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

exe_platforms := all

ifndef bin_dir
$(error bin_dir is not set)
endif

ifndef build_names
ifndef exe_build_names
$(error build_names and exe_build_names are not set)
endif
build_names := # empty
endif

all_exe_build_names := $(sort $(build_names) $(exe_build_names))

fatal_if_undefined = $(if $(findstring undefined,$(origin $1)),$(error $1 is not set))

define check_variables
$(call fatal_if_undefined,go_$1_ldflags)
$(call fatal_if_undefined,go_$1_main_dir)
$(call fatal_if_undefined,go_$1_mod_dir)

ifneq ($(go_$1_main_dir:.%=.),.)
$$(error go_$1_main_dir "$(go_$1_main_dir)" should be a directory path that DOES start with ".")
endif
ifeq ($(go_$1_main_dir:%/=/),/)
$$(error go_$1_main_dir "$(go_$1_main_dir)" should be a directory path that DOES NOT end with "/")
endif
ifeq ($(go_$1_main_dir:%.go=.go),.go)
$$(error go_$1_main_dir "$(go_$1_main_dir)" should be a directory path that DOES NOT end with ".go")
endif
ifneq ($(go_$1_mod_dir:\.%=\.),.)
$$(error go_$1_mod_dir "$(go_$1_mod_dir)" should be a directory path that DOES start with ".")
endif
ifeq ($(go_$1_mod_dir:%/=/),/)
$$(error go_$1_mod_dir "$(go_$1_mod_dir)" should be a directory path that DOES NOT end with "/")
endif
ifeq ($(go_$1_mod_dir:%.go=.go),.go)
$$(error go_$1_mod_dir "$(go_$1_mod_dir)" should be a directory path that DOES NOT end with ".go")
endif

endef

$(foreach build_name,$(all_exe_build_names),$(eval $(call check_variables,$(build_name))))

ifdef exe_build_names
$(call fatal_if_undefined,gorelease_file)
endif

##########################################

RELEASE_DRYRUN ?= false

CGO_ENABLED ?= 0
GOEXPERIMENT ?=  # empty by default

run_targets := $(all_exe_build_names:%=run-%)
build_targets := $(all_exe_build_names:%=$(bin_dir)/bin/%)

$(bin_dir)/bin:
	mkdir -p $@

.PHONY: $(run_targets)
ARGS ?= # default empty
## Directly run the go source locally.
## Any Go workfile is ignored.
## @category [shared] Build
$(run_targets): run-%: | $(NEEDS_GO)
	GOWORK=off \
	CGO_ENABLED=$(CGO_ENABLED) \
	GOEXPERIMENT=$(GOEXPERIMENT) \
	$(GO) run \
		-ldflags '$(go_$*_ldflags)' \
		$(go_$*_source_path) $(ARGS)

## Build the go source locally for development/ testing
## on the local platform. Any Go workfile is ignored.
## @category [shared] Build
$(build_targets): $(bin_dir)/bin/%: FORCE | $(NEEDS_GO)
	GOWORK=off \
	CGO_ENABLED=$(CGO_ENABLED) \
	GOEXPERIMENT=$(GOEXPERIMENT) \
	$(GO) build \
		-ldflags '$(go_$*_ldflags)' \
		-o $@ \
		$(go_$*_source_path)

define template_for_target
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .binary = "$(1)")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .main = "$(go_$(1)_main_dir)")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .dir = "$(go_$(1)_mod_dir)")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .env[0] = "CGO_ENABLED={{.Env.CGO_ENABLED}}")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .env[1] = "GOEXPERIMENT={{.Env.GOEXPERIMENT}}")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .mod_timestamp = "{{.Env.SOURCE_DATE_EPOCH}}")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .flags[0] = "-trimpath")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .ldflags[0] = "-s")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .ldflags[1] = "-w")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .ldflags[2] = "$(go_$(1)_ldflags)")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .gobinary = "$(GO)")' |
endef

## Build the go source for release. This will build the source
## for all release platforms and architectures. Additionally,
## this will create a checksums file, sboms and sign the binaries.
## @category [shared] Build
exe-publish: | $(NEEDS_GO) $(NEEDS_GORELEASER) $(NEEDS_SYFT) $(NEEDS_YQ) $(NEEDS_COSIGN)
	$(eval go_releaser_path := $(bin_dir)/scratch/exe-publish)
	rm -rf $(CURDIR)/$(go_releaser_path)

	cat $(gorelease_file) | \
	$(foreach target,$(exe_build_names),$(call template_for_target,$(target))) \
	$(YQ) '.dist = "$(CURDIR)/$(go_releaser_path)"' | \
	$(YQ) 'with(.sboms[]; .cmd = "$(SYFT)" | .args = ["$$artifact", "--output", "spdx-json=$$document"] | .env = ["SYFT_FILE_METADATA_CATALOGER_ENABLED=true"])' | \
	$(YQ) 'with(.signs[]; .cmd = "$(COSIGN)")' \
	> $(CURDIR)/$(go_releaser_path).goreleaser_config.yaml

	$(eval extra_args := )
ifeq ($(RELEASE_DRYRUN),true)
	$(eval extra_args := $(extra_args) --skip=announce,publish,validate,sign)
endif

	GOWORK=off \
	SOURCE_DATE_EPOCH=$(GITEPOCH) \
	CGO_ENABLED=$(CGO_ENABLED) \
	GOEXPERIMENT=$(GOEXPERIMENT) \
	$(GORELEASER) release \
		$(extra_args) \
		--fail-fast \
		--config=$(CURDIR)/$(go_releaser_path).goreleaser_config.yaml
