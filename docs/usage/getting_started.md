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

