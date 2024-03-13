#!/bin/bash

SCRIPTPATH="$(
  cd -- "$(dirname "$0")" >/dev/null 2>&1 || exit
  pwd -P
)"

cd "$SCRIPTPATH/.."

rm -f /tmp/e2e-operator-run.log || true

set -o pipefail
set -ex

make install generate fmt vet
go run ./main.go 2>&1 | tee /tmp/e2e-operator-run.log
