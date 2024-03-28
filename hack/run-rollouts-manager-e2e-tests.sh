#!/bin/bash

SCRIPTPATH="$(
  cd -- "$(dirname "$0")" >/dev/null 2>&1 || exit
  pwd -P
)"

cd "$SCRIPTPATH/.."

set -o pipefail
set -ex

if [ "$NAMESPACE_SCOPED_ARGO_ROLLOUTS" == "true" ]; then

  go test -v -p=1 -timeout=30m -race -count=1 -coverprofile=coverage.out ./tests/e2e/.  ./tests/e2e/namespa-scoped

else

  go test -v -p=1 -timeout=30m -race -count=1 -coverprofile=coverage.out ./tests/e2e/cluster-scoped

fi


set +e

# If the output from the E2E operator is available, then check it for errors
if [ -f "/tmp/e2e-operator-run.log" ]; then

  # Wait for the controller to flush to the file, before killing the controller
  sleep 8
  killall main
  sleep 3

  # Grep the log for unexpected errors
  # - Ignore errors that are expected to occur

  UNEXPECTED_ERRORS_FOUND_TEXT=`cat /tmp/e2e-operator-run.log | grep "ERROR" | grep -v "because it is being terminated" | grep -v "the object has been modified; please apply your changes to the latest version and try again" | grep -v "unable to fetch" | grep -v "StorageError"`
  UNEXPECTED_ERRORS_COUNT=`echo $UNEXPECTED_ERRORS_FOUND_TEXT | grep "ERROR" | wc -l`

  if [ "$UNEXPECTED_ERRORS_COUNT" != "0" ]; then
      echo "Unexpected errors found: $UNEXPECTED_ERRORS_FOUND_TEXT"
      exit 1
  fi
fi
