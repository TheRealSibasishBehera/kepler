#!/usr/bin/env bash
#
# This file is part of the Kepler project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2022 IBM, Inc.
#

set -e

CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kubernetes}
MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/manifests/${CLUSTER_PROVIDER}/generated"}

function main() {
    echo "Cleaning up ..."

    if [ ! -d "${MANIFESTS_OUT_DIR}" ]; then
        echo "Directory ${MANIFESTS_OUT_DIR} DOES NOT exists. Run make generate first."
        exit
    fi

    kubectl delete --ignore-not-found=true -f ${MANIFESTS_OUT_DIR}

    sleep 2

    echo "Done $0"
}

main "$@"