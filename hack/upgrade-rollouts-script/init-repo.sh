#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

cd $SCRIPT_DIR

rm -rf "$SCRIPT_DIR/argo-rollouts-manager" || true

git clone "git@github.com:$GITHUB_FORK_USERNAME/argo-rollouts-manager"
cd argo-rollouts-manager

git remote add parent "git@github.com:argoproj-labs/argo-rollouts-manager"
git fetch parent

