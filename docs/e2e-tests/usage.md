# E2E Tests Guide

E2E tests are written using [Ginkgo](https://github.com/onsi/ginkgo).

## Requirements

This test suite assumes that an Argo Rollouts Manager is installed on the cluster or running locally.

The system executing the tests must have following tools installed:

* `kubectl` client

There should be a `kubeconfig` pointing to your cluster, user should have full admin privileges (i.e. `kubeadm`).

### Run e2e tests

Run the controller:
```sh
make install run
```

In a separate window/terminal, run the tests against the controller:
```sh
make test-e2e
```

### Running single tests

Sometimes (e.g. when initially writing a test or troubleshooting an existing
one), you may want to run single test cases isolated. To do so, you can use the
ginkgo CLI utility. It is also possible to use 'go test'.

```sh
ginkgo -r -focus "(name of test)" tests/e2e

# Example:
ginkgo -r -focus "Reconcile is called on a new, basic, namespaced-scoped RolloutManager" tests/e2e
```
