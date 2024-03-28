#!/bin/bash

SCRIPTPATH="$(
  cd -- "$(dirname "$0")" >/dev/null 2>&1 || exit
  pwd -P
)"

cd "$SCRIPTPATH/.."

killall main

sleep 3s

rm -f /tmp/e2e-operator-run.log || true

set -o pipefail
set -ex

make install generate fmt vet

if [ "$RUN_IN_BACKGROUND" == "true" ]; then
  go run ./main.go 2>&1 | tee /tmp/e2e-operator-run.log &
else
  go run ./main.go 2>&1 | tee /tmp/e2e-operator-run.log
fi
