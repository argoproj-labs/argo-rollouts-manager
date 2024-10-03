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


### RolloutManager example with metadata for the resources generated

You can provide labels and annotation for all the resources generated (Argo Rollouts controller, ConfigMap, etc.).

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
  labels:
    example: with-metadata-example
spec:
  additionalMetadata:
    labels:
      mylabel: "true"
    annotations:
      myannotation: "myvalue"
```


### RolloutManager example with resources requests/limits for the Argo Rollouts controller

You can provide resources requests and limits for the Argo Rollouts controller.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
  labels:
    example: with-resources-example
spec:
  controllerResources:
    requests:
      memory: "64Mi"
      cpu: "250m"
    limits:
      memory: "128Mi"
      cpu: "500m"
```


### RolloutManager example with an option to skip the argo rollouts notification secret deployment

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
  labels:
    example: with-metadata-example
spec:
  skipNotificationSecretDeployment: true
```


### RolloutManager example with metric and trafficManagement Plugins

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: RolloutManager
metadata:
  name: argo-rollout
  labels:
    example: with-plugins
spec:
  plugins:
    trafficManagement:
      - name: argoproj-labs/gatewayAPI
        location: https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.4.0/gatewayapi-plugin-linux-amd64  
    metric:
      - name: "argoproj-labs/sample-prometheus"
        location: https://github.com/argoproj-labs/sample-rollouts-metric-plugin/releases/download/v0.0.3/metric-plugin-linux-amd64
        sha256: a597a017a9a1394a31b3cbc33e08a071c88f0bd8
```