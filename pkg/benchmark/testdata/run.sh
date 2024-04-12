#!/usr/bin/env bash

# Copyright 2022 The cert-manager Authors.
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

set -o nounset
set -o errexit
set -o pipefail

go run ../../../../ x benchmark \
   --benchmark.phase1.load-interval=$BENCHMARK_PHASE1_LOAD_INTERVAL \
   --benchmark.phase1.target-certificate-count=$BENCHMARK_PHASE1_TARGET_CERTIFICATE_COUNT \
   --benchmark.phase1.certificate-algorithm=$BENCHMARK_PHASE1_CERTIFICATE_ALGORITHM \
   --benchmark.phase1.certificate-size=$BENCHMARK_PHASE1_CERTIFICATE_SIZE \
   --benchmark.phase3.duration=$BENCHMARK_PHASE3_DURATION \
   --benchmark.phase4.cleanup-interval=$BENCHMARK_PHASE4_CLEANUP_INTERVAL \
    | tee -a experiment.${EXPERIMENT_ID}.json
