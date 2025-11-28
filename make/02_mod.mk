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

include make/test-unit.mk
include make/test-integration.mk

.PHONY: dryrun-release
## Dry-run release process
## @category [shared] Release
dryrun-release: export RELEASE_DRYRUN := true
dryrun-release: release

.PHONY: generate-conversion
## Generate code for converting between versions of the cert-manager API
## @category Generate/ Verify
generate-conversion: | $(NEEDS_CONTROLLER-GEN) $(NEEDS_DEFAULTER-GEN) $(NEEDS_CONVERSION-GEN)
	rm -rf ./pkg/convert/internal/apis/**/zz_generated.*
	rm -rf ./pkg/convert/internal/apis/**/**/zz_generated.*

	$(CONTROLLER-GEN) \
		object:headerFile=$(go_header_file) \
		paths=./pkg/convert/internal/apis/...

	$(DEFAULTER-GEN) \
		--go-header-file=$(go_header_file) \
		--output-file=zz_generated.defaults.go \
		./pkg/convert/internal/apis/{acme,certmanager}/v{1,1alpha2,1alpha3,1beta1}/... \
		./pkg/convert/internal/apis/meta/v1/...

	
	$(CONVERSION-GEN) \
		--go-header-file=$(go_header_file) \
		--output-file=zz_generated.conversion.go \
		./pkg/convert/internal/apis/{acme,certmanager}/v{1,1alpha2,1alpha3,1beta1}/... \
		./pkg/convert/internal/apis/meta/v1/...

shared_generate_targets += generate-conversion

.PHONY: release
## Publish all release artifacts (image + helm chart)
## @category [shared] Release
release: | $(NEEDS_CRANE) $(bin_dir)/scratch
	$(MAKE) exe-publish
	$(MAKE) oci-push-cmctl

	@echo "Release complete!"
