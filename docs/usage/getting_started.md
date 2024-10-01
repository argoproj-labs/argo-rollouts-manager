# Getting Started

## Install the Operator

Install the operator using one of the two steps mentioned below.

- [Install using kustomize](../install/kustomize.md)
- [Install using OLM](../install/olm.md)

Alternatively, if you are a developer looking to run operator locally or build a new version of operator for your changes, please follow the steps mentioned in developer [guide](../developer-guide/developer_guide.md).

## Deploy RolloutManager

It is recommended to start with [basic](../crd_reference.md/#basic-rolloutmanager-example) RolloutManager configuration. 

### Apply 

```bash
kubectl apply -f examples/basic_rolloutmanager.yaml
```

This will create the rollout controller and related resources such as serviceaccount, roles, rolebinding, deployment, service, secret and others.

You can check if the above mentioned resources are created by running the below command.

```bash
kubectl get all
```

If you would like to understand the siginificance of each rollout controller resource created by the operator, please go through the official rollouts controller [docs](https://argo-rollouts.readthedocs.io/en/stable/).




## Namespace Scoped Rollouts Instance

A namespace-scoped Rollouts instance can manage Rollouts resources of same namespace it is deployed into. To deploy a namespace-scoped Rollouts instance set `spec.namespaceScoped` field to `true`.

```yml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
spec:
  namespaceScoped: true
```


## Cluster Scoped Rollouts Instance

A cluster-scoped Rollouts instance can manage Rollouts resources from other namespaces as well. To install a cluster-scoped Rollouts instance first you need to add `NAMESPACE_SCOPED_ARGO_ROLLOUTS` and `CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES` environment variables in subscription resource. If `NAMESPACE_SCOPED_ARGO_ROLLOUTS` is set to `false` then only you are allowed to create a cluster-scoped instance and then you need to provide list of namespaces that are allowed host a cluster-scoped Rollouts instance via `CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES` environment variable.  

```yml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: argo-operator
spec:
  config:
   env: 
    - name: NAMESPACE_SCOPED_ARGO_ROLLOUTS
      value: 'false'   
    - name: CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES
      value: <list of namespaces of cluster-scoped Argo CD instances>
  (...)
```

Now set `spec.namespaceScoped` field to `false` to create a Rollouts instance.

```yml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
spec:
  namespaceScoped: false
```
