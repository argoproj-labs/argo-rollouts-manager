# RolloutManager Custom Resource

This page provides the information about Argo Rollout Custom Resource specification.

Name | Default | Description
--- | --- | ---
Env | [Empty] | Adds environment variables to the Rollouts controller.
ExtraCommandArgs | [Empty] | Extra Command arguments allows user to pass command line arguments to rollouts controller.
Image | `quay.io/argoproj/argo-rollouts` | The container image for the rollouts controller. This overrides the `ARGO_ROLLOUTS_IMAGE` environment variable.
NodePlacement | [Empty] | Refer NodePlacement [Section](#nodeplacement)
Version | *(recent rollouts version)* | The tag to use with the rollouts container image.

## NodePlacement

The following properties are available for configuring the NodePlacement component.

Name | Default | Description
--- | --- | ---
NodeSelector | [Empty] | A map of key value pairs for node selection.
Tolerations | [Empty] | Tolerations allow pods to schedule on nodes with matching taints.

### Basic RolloutManager example

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
  labels:
    example: basic
spec: {}
```

### RolloutManager example with properties

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
  labels:
    example: with-properties
spec:
  env:
   - name: "foo"
     value: "bar"
  extraCommandArgs:
   - --foo
   - bar
  image: "quay.io/random/my-rollout-image"
  version: "sha256:...."
```

### RolloutManager with NodePlacement Example

The following example sets a NodeSelector and tolerations using NodePlacement property in the RolloutManager CR.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
  labels:
    example: nodeplacement-example
spec:
  nodePlacement: 
    nodeSelector: 
      key1: value1
    tolerations: 
    - key: key1
      operator: Equal
      value: value1
      effect: NoSchedule
    - key: key1
      operator: Equal
      value: value1
      effect: NoExecute   
```


