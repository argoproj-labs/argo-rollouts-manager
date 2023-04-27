# Manual Installation using kustomize

The following steps can be used to manually install the operator on any Kubernetes environment with minimal overhead.

!!! info
    Several of the steps in this process require the `cluster-admin` ClusterRole or equivalent.

## Namespace

By default, the operator is installed into the `argo-rollouts-operator-system` namespace. To modify this, update the
value of the `namespace` specified in the `config/default/kustomization.yaml` file. 

## Deploy Operator

```bash
make deploy
```

If you want to use your own custom operator container image, you can specify the image name using the `IMG` variable.

```bash
make deploy IMG=quay.io/my-org/argo-rollouts-operator:latest
```

The operator pod should start and enter a `Running` state after a few seconds.

```bash
kubectl get pods -n argo-rollouts-operator-system
```

```bash
NAME                                                  READY   STATUS    RESTARTS   AGE
argo-rollouts-operator-controller-manager-65777cf998-pr9fg   2/2     Running   0          69s
```
    
## Usage 

Once the operator is installed and running, new ArgoRollout resources can be created. See the getting started [guide](../usage/getting_started.md) to learn how to create new `ArgoRollout` resources.

## Cleanup 

To remove the operator from the cluster, run the following comand. This will remove all resources that were created,
including the namespace.

```bash
make undeploy
```