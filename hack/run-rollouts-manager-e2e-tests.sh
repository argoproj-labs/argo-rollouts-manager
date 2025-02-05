#!/bin/bash

SCRIPTPATH="$(
  cd -- "$(dirname "$0")" >/dev/null 2>&1 || exit
  pwd -P
)"

function cleanup {
  echo "* Cleaning up"
  killall main || true
  killall go || true
}

trap cleanup EXIT


# Treat undefined variables as errors
set -u

extract_metrics_data() {

  set +eu

  TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'mytmpdir')

  while true; do
    curl http://localhost:8080/metrics -o "$TMP_DIR/rollouts-metric-endpoint-output.txt"
    if [ "$?" == "0" ]; then
        break
    fi
    echo "* Waiting for Metrics endpoint to become available"
    sleep 3
  done

  # 1) Extract REST client get/put/post metrics
  
  # Example: the metrics from /metric endpoint look like this:
  # rest_client_requests_total{code="200",host="api.pgqqd-novoo-oqu.pa43.p3.openshiftapps.com:443",method="GET"} 42
  # rest_client_requests_total{code="200",host="api.pgqqd-novoo-oqu.pa43.p3.openshiftapps.com:443",method="PUT"} 88
  # rest_client_requests_total{code="201",host="api.pgqqd-novoo-oqu.pa43.p3.openshiftapps.com:443",method="POST"} 110

  curl http://localhost:8080/metrics -o "$TMP_DIR/rollouts-metric-endpoint-output.txt"

  echo "* Metrics gathered raw output ---------------------------------------------------------------"
  cat "$TMP_DIR/rollouts-metric-endpoint-output.txt"
  echo "---------------------------------------------------------------------------------------------"

  GET_REQUESTS=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "rest_client_requests_total" | grep "GET" | grep "200" | rev | cut -d' ' -f1 | rev`
  
  PUT_REQUESTS_200=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "rest_client_requests_total" | grep "PUT" | grep "200" | rev | cut -d' ' -f1 | rev`

  # 409 Conflict error code
  PUT_REQUESTS_409=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "rest_client_requests_total" | grep "PUT" | grep "409" | rev | cut -d' ' -f1 | rev`

  # 201 Created error code
  POST_REQUESTS_201=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "rest_client_requests_total" | grep "POST" | grep "201" | rev | cut -d' ' -f1 | rev`

  # 409 Conflict error code
  POST_REQUESTS_409=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "rest_client_requests_total" | grep "POST" | grep "409" | rev | cut -d' ' -f1 | rev`

  if [[ "$GET_REQUESTS" == "" ]]; then
    GET_REQUESTS=0
  fi

  if [[ "$POST_REQUESTS_201" == "" ]]; then
    POST_REQUESTS_201=0
  fi
  if [[ "$POST_REQUESTS_409" == "" ]]; then
    POST_REQUESTS_409=0
  fi

  if [[ "$PUT_REQUESTS_200" == "" ]]; then
    PUT_REQUESTS_200=0
  fi
  if [[ "$PUT_REQUESTS_409" == "" ]]; then
    PUT_REQUESTS_409=0
  fi

  PUT_REQUESTS=`expr $PUT_REQUESTS_200 + $PUT_REQUESTS_409`
  POST_REQUESTS=`expr $POST_REQUESTS_201 + $POST_REQUESTS_409`

  # 2) Extract the # of RolloutManager reconciles

  # Example: the metrics from /metric endpoint look like this:
  # controller_runtime_reconcile_total{controller="rolloutmanager",result="error"} 0
  # controller_runtime_reconcile_total{controller="rolloutmanager",result="requeue"} 0
  # controller_runtime_reconcile_total{controller="rolloutmanager",result="requeue_after"} 0
  # controller_runtime_reconcile_total{controller="rolloutmanager",result="success"} 135
  ERROR_RECONCILES=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "controller_runtime_reconcile_total" | grep "error" | rev | cut -d' ' -f1`
  SUCCESS_RECONCILES=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "controller_runtime_reconcile_total" | grep "success" | rev | cut -d' ' -f1`

  if [[ "$ERROR_RECONCILES" == "" ]]; then
    ERROR_RECONCILES=0
  fi

  if [[ "$SUCCESS_RECONCILES" == "" ]]; then
    SUCCESS_RECONCILES=0
  fi

}


cd "$SCRIPTPATH/.."

set -o pipefail

# Check if the CRD exists
kubectl get crd/servicemonitors.monitoring.coreos.com &> /dev/null
retVal=$?
if [ $retVal -ne 0 ]; then
  # If the CRD is not found, apply the CRD YAML
  kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/release-0.52/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml
fi

# Before the test starts, extract initial metrics values from the /metrics endpoint of the operator
extract_metrics_data

set -eu

INITIAL_GET_REQUESTS=$GET_REQUESTS
INITIAL_PUT_REQUESTS=$PUT_REQUESTS
INITIAL_POST_REQUESTS=$POST_REQUESTS
INITIAL_ERROR_RECONCILES=$ERROR_RECONCILES
INITIAL_SUCCESS_RECONCILES=$SUCCESS_RECONCILES


# Run the tests

set -ex

if [ "$NAMESPACE_SCOPED_ARGO_ROLLOUTS" == "true" ]; then

  go test -v -p=1 -timeout=30m -race -count=1 -coverprofile=coverage.out ./tests/e2e/namespace-scoped

else

  go test -v -p=1 -timeout=30m -race -count=1 -coverprofile=coverage.out ./tests/e2e/cluster-scoped

fi

# Sanity test the behaviour of the operator during the tests:
# - We check the (prometheus) metrics coming from the 'localhost:8080/metrics' endpoint of the operator.
# - For example, if the reported # of Reconcile calls was unusually high, this might mean that the operator was stuck in a Reconcile loop
# - Or, if the number of REST client POST requests (e.g. k8s objection creation) was equal to the number of PUT request (e.g. k8s object update), this may imply we are updating .status or .spec of an object too frequently.
sanity_test_metrics_data() {

  extract_metrics_data

  set -eux

  FINAL_GET_REQUESTS=$GET_REQUESTS
  FINAL_PUT_REQUESTS=$PUT_REQUESTS
  FINAL_POST_REQUESTS=$POST_REQUESTS
  FINAL_ERROR_RECONCILES=$ERROR_RECONCILES
  FINAL_SUCCESS_RECONCILES=$SUCCESS_RECONCILES

  DELTA_GET_REQUESTS=`expr $FINAL_GET_REQUESTS - $INITIAL_GET_REQUESTS`
  DELTA_PUT_REQUESTS=`expr $FINAL_PUT_REQUESTS - $INITIAL_PUT_REQUESTS`
  DELTA_POST_REQUESTS=`expr $FINAL_POST_REQUESTS - $INITIAL_POST_REQUESTS`

  DELTA_ERROR_RECONCILES=`expr $FINAL_ERROR_RECONCILES - $INITIAL_ERROR_RECONCILES`
  DELTA_SUCCESS_RECONCILES=`expr $FINAL_SUCCESS_RECONCILES - $INITIAL_SUCCESS_RECONCILES`

  if [[ "$DELTA_POST_REQUESTS" == "0" ]]; then
    echo "Unexpected number of REST client post requests: should be at least 1"
    exit 1
  fi 

  # The # of PUT requests should be less than 40% of the # of POST requests
  # - If the number is higher, this implies we are updating the .status or .spec fields of resources more than is necessary.
  PUT_REQUEST_PERCENT=`expr "$DELTA_PUT_REQUESTS"00 / $DELTA_POST_REQUESTS`

  if [[ "`expr $PUT_REQUEST_PERCENT \> 40`" == "1" ]]; then
    # This value is arbitrary, and should be updated if at any point it becomes inaccurate (but first audit the test/code to make sure it is not an actual product/test issue, before increasing)

    echo "Put request was %$PUT_REQUEST_PERCENT greater than the expected value"
    exit 1

  fi

  if [[ "`expr $DELTA_ERROR_RECONCILES \> 70`" == "1" ]]; then
    # This value is arbitrary, and should be updated if at any point it becomes inaccurate (but first audit the test/code to make sure it is not an actual product/test issue, before increasing)
    echo "Number of Reconcile calls that returned an error '$DELTA_ERROR_RECONCILES' was greater than the expected value"
    exit 1

  fi

  if [[ "`expr $DELTA_SUCCESS_RECONCILES \> 1200`" == "1" ]]; then
    # This value is arbitrary, and should be updated if at any point it becomes inaccurate (but first audit the test/code to make sure it is not an actual product/test issue, before increasing)

    echo "Number of Reconcile calls that returned success '$DELTA_SUCCESS_RECONCILES' was greater than the expected value"
    exit 1

  fi
}

sanity_test_metrics_data

set +e

# If the output from the E2E operator is available, then check it for errors
if [ -f "/tmp/e2e-operator-run.log" ]; then

  # Wait for the controller to flush to the file, before killing the controller
  sleep 10
  killall main
  sleep 5

  # Grep the log for unexpected errors
  # - Ignore errors that are expected to occur

  set +u # allow undefined vars

  UNEXPECTED_ERRORS_FOUND_TEXT=`cat /tmp/e2e-operator-run.log | grep "ERROR" | grep -v "because it is being terminated" | grep -v "the object has been modified; please apply your changes to the latest version and try again" | grep -v "unable to fetch" | grep -v "StorageError" | grep -v "client rate limiter Wait returned an error: context canceled" | grep -v "failed to reconcile Rollout's ClusterRoleBinding" | grep -v "clusterrolebindings.rbac.authorization.k8s.io \"argo-rollouts\" already exists"`

  if [ "$UNEXPECTED_ERRORS_FOUND_TEXT" != "" ]; then
  
    UNEXPECTED_ERRORS_COUNT=`echo $UNEXPECTED_ERRORS_FOUND_TEXT | grep "ERROR" | wc -l`
    
    if [ "$UNEXPECTED_ERRORS_COUNT" != "0" ]; then
        echo "Unexpected errors found: $UNEXPECTED_ERRORS_FOUND_TEXT"
        exit 1
    fi  
  fi


fi
