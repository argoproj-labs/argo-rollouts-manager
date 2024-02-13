#!/bin/bash

CURRENT_ROLLOUTS_VERSION=v1.6.6

function cleanup {
  echo "* Cleaning up"
	killall main
  killall go
}

set -x
set -e

trap cleanup EXIT

# Directory of bash script
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# 1) Clone a specific version of argo-rollouts into a temporary directory
TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'mytmpdir')
cd $TMP_DIR

git clone https://github.com/argoproj/argo-rollouts
cd argo-rollouts
git checkout $CURRENT_ROLLOUTS_VERSION
go mod tidy

# 2) Setup the Namespace

kubectl delete ns argo-rollouts || true

kubectl wait --timeout=5m --for=delete namespace/argo-rollouts 

kubectl create ns argo-rollouts
kubectl config set-context --current --namespace=argo-rollouts


# 3) Build, install, and start the argo-rollouts-manager controller
cd $SCRIPT_DIR/..
make generate fmt vet install

set +e

rm -f /tmp/e2e-operator-run.log || true
go run ./main.go 2>&1 | tee /tmp/e2e-operator-run.log &

set -e

# 4) Install Argo Rollouts into the Namespace via RolloutManater CR

cd $TMP_DIR/argo-rollouts

cat << EOF > $TMP_DIR/rollout-manager.yaml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
spec:
  extraCommandArgs:
    - "--loglevel" 
    - "debug" 
    - "--kloglevel" 
    - "6"
    - "--instance-id"
    - "argo-rollouts-e2e"
EOF

kubectl apply -f $TMP_DIR/rollout-manager.yaml

echo "* Waiting for Argo Rollouts Deployment to exist"

until kubectl get -n argo-rollouts deployment/argo-rollouts
do
  sleep 1s
done

kubectl wait --for=condition=Available --timeout=10m -n argo-rollouts deployment/argo-rollouts

kubectl apply -f test/e2e/crds

# Required because the rollouts containers run as root, and OpenShift's default security policy doesn't like that
oc adm policy add-scc-to-user anyuid -z argo-rollouts -n argo-rollouts || true
oc adm policy add-scc-to-user anyuid -z default -n argo-rollouts || true


# 5) Run the E2E tests
rm -f /tmp/test-e2e.log

set +e

make test-e2e | tee /tmp/test-e2e.log

set +x

# 6) Check the results for unexpected failures

# Here we grep out the failures we expect, as of January 2024:
# - Most of these tests fail 100% of the time, because they were not designed to run against Argo Rollouts running on a cluster. (These are safe to ignore.)
#     - The rollouts tests are written to assume they are running locally: not within a container, and not on a K8s cluster.
# - Some are intermittently failing, implying a race condition in the test/product.
# - Finally, some still need to be investigated, to determine why they are failing in this case.
UNEXPECTED_FAILURES=`cat /tmp/test-e2e.log | grep "FAIL:" | grep -v "re-run" \
  | grep -v "TestAPISIXSuite (" \
  | grep -v "TestFunctionalSuite (" \
  | grep -v "TestCanarySuite (" \
  | grep -v "TestAWSSuite (" \
  | grep -v "TestExperimentSuite (" \
  | grep -v "TestControllerMetrics" \
  | grep -v "TestAPISIXCanarySetHeaderStep" \
  | grep -v "TestALBExperimentStep " \
  | grep -v "TestALBExperimentStepNoSetWeightMultiIngress"  \
  | grep -v "TestCanaryDynamicStableScale"  \
  | grep -v "TestExperimentWithDryRunMetrics" \
  | grep -v "TestBlueGreenPromoteFull" \
  | grep -v "TestALBExperimentStepNoSetWeight" \
  | grep -v "TestCanaryScaleDownOnAbort"`

# As of January 2024 (Rollouts v1.6.4):
# 
# Always fail:
# - TestCanaryDynamicStableScale
# - TestCanaryScaleDownOnAbort
# - TestAPISIXCanarySetHeaderStep
# - TestControllerMetrics (also fails when running upstream rollouts as a container)
# - TestExperimentWithDryRunMetrics (also fails when running upstream rollouts as a container)
#
# Intermittently fail:
# - TestBlueGreenPromoteFull
# - TestALBExperimentStepNoSetWeight
# - TestALBExperimentStep
# - TestALBExperimentStepNoSetWeightMultiIngress

echo "-----------------------------------------------------------------"
echo
echo "These were the tests that succeeded:"
echo
cat /tmp/test-e2e.log | grep "PASS" | sort
echo
echo "These were the tests that failed:"
echo
cat /tmp/test-e2e.log | grep "    --- FAIL:" | grep -v "re-run" | sort -u
echo
echo

if [ -n "$UNEXPECTED_FAILURES" ]; then
  echo "* FAIL: Unexpected failures occurred."
  exit 1
else
  echo "* SUCCESS: No unexpected errors occurred."
fi


