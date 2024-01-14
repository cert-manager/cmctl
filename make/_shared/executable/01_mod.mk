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
endif

all_exe_build_names := $(sort $(build_names) $(exe_build_names))

fatal_if_undefined = $(if $(findstring undefined,$(origin $1)),$(error $1 is not set))

define check_variables
$(call fatal_if_undefined,go_$1_ldflags)
$(call fatal_if_undefined,go_$1_source_path)
endef

$(foreach build_name,$(all_exe_build_names),$(eval $(call check_variables,$(build_name))))

ifdef exe_build_names
$(call fatal_if_undefined,gorelease_file)
endif

##########################################

CGO_ENABLED ?= 0

run_targets := $(all_exe_build_names:%=run-%)
build_targets := $(all_exe_build_names:%=build-%)

.PHONY: $(run_targets)
ARGS ?= # default empty
## Directly run the go source locally.
## @category [shared] Build
$(run_targets): run-%: | $(NEEDS_GO)
	CGO_ENABLED=$(CGO_ENABLED) \
	$(GO) run \
		-ldflags '$(go_$*_ldflags)' \
		$(go_$*_source_path) $(ARGS)

## Build the go source locally for development/ testing
## on the local platform.
## @category [shared] Build
$(build_targets): build-%: | $(NEEDS_GO)
	CGO_ENABLED=$(CGO_ENABLED) \
	$(GO) build \
		-ldflags '$(go_$*_ldflags)' \
		-o $@ \
		$(go_$*_source_path)

#### TOOODODDD: ^^ set a output build binary file

define template_for_target
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .binary = "$(1)")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .main = "$(go_$(1)_source_path)")' | \
	$(YQ) 'with(.builds[]; select(.id == "$(1)") | .env[0] = "CGO_ENABLED={{.Env.CGO_ENABLED}}")' | \
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
exe-publish: | $(NEEDS_GO) $(NEEDS_GORELEASER)
	$(eval go_releaser_path := $(bin_dir)/scratch/exe-publish)
	rm -rf $(CURDIR)/$(go_releaser_path)

	cp $(gorelease_file) $(CURDIR)/$(go_releaser_path).goreleaser_config.yaml

	cat $(CURDIR)/$(go_releaser_path).goreleaser_config.yaml | \
	$(foreach target,$(exe_build_names),$(call template_for_target,$(target))) \
	$(YQ) '.dist = "$(CURDIR)/$(go_releaser_path)"' \
	> $(CURDIR)/$(go_releaser_path).goreleaser_config.yaml
	
	SOURCE_DATE_EPOCH=$(GITEPOCH) \
	CGO_ENABLED=$(CGO_ENABLED) \
	$(GORELEASER) build \
		--config=$(CURDIR)/$(go_releaser_path).goreleaser_config.yaml \
		--snapshot
