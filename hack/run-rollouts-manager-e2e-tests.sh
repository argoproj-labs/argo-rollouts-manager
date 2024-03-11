#!/bin/bash

SCRIPTPATH="$(
  cd -- "$(dirname "$0")" >/dev/null 2>&1 || exit
  pwd -P
)"

cd "$SCRIPTPATH/.."

set -o pipefail
set -ex

go test -v -p=1 -timeout=30m -race -count=1 -coverprofile=coverage.out ./tests/e2e/...

set +e

# If the output from the E2E operator is available, then check it for errors
if [ -f "/tmp/e2e-operator-run.log" ]; then

    # Grep the log for unexpected errors
    # - Ignore errors that are expected to occur

    ERRORS_FOUND_TEXT=`cat /tmp/e2e-operator-run.log | grep "ERROR" | grep -v "unable to create new content in namespace argo-rollouts because it is being terminated" | grep -v "the object has been modified; please apply your changes to the latest version and try again"`

    ERRORS_FOUND=`echo $ERRORS_FOUND_TEXT | grep "ERROR" | wc -l`

    if [ "$ERRORS_FOUND" != "0" ]; then
        echo "Unexpected errors found: $ERRORS_FOUND_TEXT"
        exit 1
    fi

fi
