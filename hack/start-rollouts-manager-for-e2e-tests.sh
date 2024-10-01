#!/bin/bash

SCRIPTPATH="$(
  cd -- "$(dirname "$0")" >/dev/null 2>&1 || exit
  pwd -P
)"

cd "$SCRIPTPATH/.."

killall main

sleep 5s

rm -f /tmp/e2e-operator-run.log || true

set -o pipefail
set -ex

make install generate fmt vet

# Set namespaces used for cluster-scoped e2e tests
export CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES="argo-rollouts,test-rom-ns-1,rom-ns-1"

if [ "$RUN_IN_BACKGROUND" == "true" ]; then
  go run ./cmd/main.go 2>&1 | tee /tmp/e2e-operator-run.log &
else
  go run ./cmd/main.go 2>&1 | tee /tmp/e2e-operator-run.log
fi
