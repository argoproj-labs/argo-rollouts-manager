# E2E Tests Guide

E2E tests are written using [KUTTL](https://kuttl.dev/docs/#install-kuttl-cli).

## Requirements

This test suite assumes that an Argo Rollouts Manager is installed on the cluster or running locally.

The system executing the tests must have following tools installed:

* `kuttl` kubectl plugin
* `kubectl` client

There should be a `kubeconfig` pointing to your cluster, user should have full admin privileges (i.e. `kubeadm`).

### Run e2e tests

```sh
make e2e
```

### Run e2e tests without make target

```sh
kubectl kuttl test --config ./tests/e2e/kuttl-tests.yaml
```

### Running single tests

Sometimes (e.g. when initially writing a test or troubleshooting an existing
one), you may want to run single test cases isolated. To do so, you can pass
the name of the test using `--test` to `kuttl`, i.e.

```sh
kubectl kuttl --config ./tests/e2e/kuttl-tests.yaml --test <name-of-the-test.yaml>
```

The name of the test is the name of the directory containing its steps and
assertions.

### Skip deletion of Kuttl namespace

If you are troubleshooting, you may want to prevent `kuttl` from deleting the
test's namespace afterwards. In order to do so, just pass the additional flag
`--skip-delete` to above command.