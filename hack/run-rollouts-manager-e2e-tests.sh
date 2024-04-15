#!/bin/bash

SCRIPTPATH="$(
  cd -- "$(dirname "$0")" >/dev/null 2>&1 || exit
  pwd -P
)"

# Treat undefined variables as errors
set -u

TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'mytmpdir')


extract_metrics_data() {
  
  # 1) Extract REST client get/put/post metrics
  
  # Example: the metrics from /metric endpoint look like this:
  # rest_client_requests_total{code="200",host="api.pgqqd-novoo-oqu.pa43.p3.openshiftapps.com:443",method="GET"} 42
  # rest_client_requests_total{code="200",host="api.pgqqd-novoo-oqu.pa43.p3.openshiftapps.com:443",method="PUT"} 88
  # rest_client_requests_total{code="201",host="api.pgqqd-novoo-oqu.pa43.p3.openshiftapps.com:443",method="POST"} 110

  curl http://localhost:8080/metrics -o "$TMP_DIR/rollouts-metric-endpoint-output.txt"
  GET_REQUESTS=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "rest_client_requests_total" | grep "GET" | rev | cut -d' ' -f1`
  PUT_REQUESTS=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "rest_client_requests_total" | grep "PUT" | rev | cut -d' ' -f1`
  POST_REQUESTS=`cat "$TMP_DIR/rollouts-metric-endpoint-output.txt" | grep "rest_client_requests_total" | grep "POST" | rev | cut -d' ' -f1`


  if [[ "$GET_REQUESTS" == "" ]]; then
    GET_REQUESTS=0
  fi
  if [[ "$POST_REQUESTS" == "" ]]; then
    POST_REQUESTS=0
  fi
  if [[ "$PUT_REQUESTS" == "" ]]; then
    PUT_REQUESTS=0
  fi


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


# Before the test starts, extract initial metrics values
extract_metrics_data

INITIAL_GET_REQUESTS=$GET_REQUESTS
INITIAL_PUT_REQUESTS=$PUT_REQUESTS
INITIAL_POST_REQUESTS=$POST_REQUESTS
INITIAL_ERROR_RECONCILES=$ERROR_RECONCILES
INITIAL_SUCCESS_RECONCILES=$SUCCESS_RECONCILES


set -ex


if [ "$NAMESPACE_SCOPED_ARGO_ROLLOUTS" == "true" ]; then

  go test -v -p=1 -timeout=30m -race -count=1 -coverprofile=coverage.out ./tests/e2e/namespace-scoped

else

  go test -v -p=1 -timeout=30m -race -count=1 -coverprofile=coverage.out ./tests/e2e/cluster-scoped

fi

set +e

# If the output from the E2E operator is available, then check it for errors
if [ -f "/tmp/e2e-operator-run.log" ]; then

  # Wait for the controller to flush to the file, before killing the controller
  sleep 10
  killall main
  sleep 5

  # Grep the log for unexpected errors
  # - Ignore errors that are expected to occur

  UNEXPECTED_ERRORS_FOUND_TEXT=`cat /tmp/e2e-operator-run.log | grep "ERROR" | grep -v "because it is being terminated" | grep -v "the object has been modified; please apply your changes to the latest version and try again" | grep -v "unable to fetch" | grep -v "StorageError"` | grep -v "client rate limiter Wait returned an error: context canceled"
  UNEXPECTED_ERRORS_COUNT=`echo $UNEXPECTED_ERRORS_FOUND_TEXT | grep "ERROR" | wc -l`
  
  if [ "$UNEXPECTED_ERRORS_COUNT" != "0" ]; then
      echo "Unexpected errors found: $UNEXPECTED_ERRORS_FOUND_TEXT"
      exit 1
  fi
fi


extract_metrics_data

set -x

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

# Sanity test the behaviour of the operator during the tests

# The # of PUT requests should be less than 40% of the # of POST requests
# - If the number is higher, this implies we are updating the .status or .spec fields of resources more than is necessary.
PUT_REQUEST_PERCENT=`expr "$DELTA_PUT_REQUESTS"00 / $DELTA_POST_REQUESTS`

if [[ "`expr $PUT_REQUEST_PERCENT \> 40`" == "1" ]]; then

  echo "Put request %$PUT_REQUEST_PERCENT was greater than the expected value"
  exit 1

fi

if [[ "`expr $DELTA_ERROR_RECONCILES \> 20`" == "1" ]]; then

  echo "Number of Reconcile calls that returned an error $DELTA_ERROR_RECONCILES was greater than the expected value"
  exit 1

fi

if [[ "`expr $DELTA_SUCCESS_RECONCILES \> 200`" == "1" ]]; then

  echo "Number of Reconcile calls that returned success $DELTA_SUCCESS_RECONCILES was greater than the expected value"
  exit 1

fi




# The # of reconcile calls should be within an expected 
# - This may need to be updated as we add new E2E tests.