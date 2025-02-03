#!/bin/bash

CURRENT_ROLLOUTS_VERSION=v1.8.0

function cleanup {
  echo "* Cleaning up"
  killall main || true
  killall go || true
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


# Only start the controller if SKIP_RUN_STEP is empty
# - Otherwise, we assume that Argo Rollouts operator is already installed and running (for example, via OpenShift GitOps)
if [ -z "$SKIP_RUN_STEP" ]; then
  make generate fmt vet install

  set +e

  rm -f /tmp/e2e-operator-run.log || true
  
  # Set namespaces used for cluster-scoped e2e tests
  export CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES="argo-rollouts"
  
  go run ./cmd/main.go 2>&1 | tee /tmp/e2e-operator-run.log &

  set -e
fi

# 4) Install Argo Rollouts into the Namespace via RolloutManager CR

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

# 6) Check and report the results for unexpected failures

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

set -e

# Call a small Go script to verify expected test failures. See Go file for details.
"$SCRIPT_DIR/verify-rollouts-e2e-tests/verify-e2e-test-results.sh" /tmp/test-e2e.log

echo "* SUCCESS: No unexpected errors occurred."




