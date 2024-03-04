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

.PHONY: generate-testgroup
## Generate code and crds for testgroup API
## @category Generate/ Verify
generate-testgroup: | $(NEEDS_CONTROLLER-GEN) $(NEEDS_DEFAULTER-GEN) $(NEEDS_CONVERSION-GEN)
	rm -rf ./test/integration/testdata/apis/testgroup/crds/*.yaml
	rm -rf ./test/integration/testdata/apis/testgroup/**/zz_generated.*

	$(CONTROLLER-GEN) \
		object:headerFile=$(go_header_file) \
		paths=./test/integration/testdata/apis/testgroup/...

	$(CONTROLLER-GEN) \
		crd \
		paths=./test/integration/testdata/apis/testgroup/v{1,2}/... \
		output:crd:dir=./test/integration/testdata/apis/testgroup/crds

	$(DEFAULTER-GEN) \
		--go-header-file=$(go_header_file) \
		--input-dirs=./test/integration/testdata/apis/testgroup/v{1,2}/... \
		--trim-path-prefix="$(repo_name)" \
		-O=zz_generated.defaults \
		--output-base=./
	
	$(CONVERSION-GEN) \
		--go-header-file=$(go_header_file) \
		--input-dirs=./test/integration/testdata/apis/testgroup/v{1,2}/... \
		--trim-path-prefix="$(repo_name)" \
		-O=zz_generated.conversion \
		--output-base=./

shared_generate_targets += generate-testgroup

.PHONY: test-integration
## Integration tests
## @category Testing
test-integration: | $(cert_manager_crds) $(NEEDS_GO) $(NEEDS_GOTESTSUM) $(NEEDS_ETCD) $(NEEDS_KUBE-APISERVER) $(NEEDS_KUBECTL) $(NEEDS_HELM) $(ARTIFACTS)
	TEST_ASSET_ETCD=$(ETCD) \
	TEST_ASSET_KUBE_APISERVER=$(KUBE-APISERVER) \
	TEST_ASSET_KUBECTL=$(KUBECTL) \
	TEST_CRDS=$(CURDIR)/test/integration/testdata/apis/testgroup \
	CERT_MANAGER_CRDS=$(CURDIR)/$(cert_manager_crds) \
	HELM_BIN=$(HELM) \
	$(GOTESTSUM) \
		--junitfile=$(ARTIFACTS)/junit-go-e2e.xml \
		-- \
		-coverprofile=$(ARTIFACTS)/filtered.cov \
		./test/integration/... \
		-- \
		-ldflags $(go_cmctl_ldflags)

	$(GO) tool cover -html=$(ARTIFACTS)/filtered.cov -o=$(ARTIFACTS)/filtered.html
