# Security Model for Argo Rollouts Manager Operator 

Author(s): Jonathan West (@jgwest)

This document describes the security model of the Argo Rollouts Manager operator (also referred to as the Argo Rollouts operator), which installs and maintains Argo Rollouts on a Kubernetes cluster. 

**Note**: Argo Rollouts is entirely separate from Argo CD. The 'Argo' prefix refers to a collection of minimaly related sibling projects.

# Analysis

At a high level, you can think of Argo Rollouts as a replacement for Kubernetes Deployments. Instead of using the `Deployment` resource of Kubernetes (to deploy your application), you instead use the `Rollout` resource of Argo Rollouts. Argo Rollouts has a similar API to Deployment, but adds additional functionality that allows you to do fancy stuff like blue-green deployments, canary deployments, traffic routing between old and new versions, etc.

The more detailed description from upstream: 

* Argo Rollouts is a Kubernetes controller and set of CRDs which provide advanced deployment capabilities such as blue-green, canary, canary analysis, experimentation, and progressive delivery features to Kubernetes. These are used to apply custom 'intelligent' logic when deploying new versions of applications.   
* Argo Rollouts integrates with ingress controllers and service meshes (such as Istio), leveraging their traffic shaping abilities to gradually shift traffic to the new version during an update. Additionally, Rollouts can query and interpret metrics from various providers to verify drive automated promotion or rollback during an update.

Argo Rollouts is deployed by the Argo Rollouts operator, which is a Kubernetes controller that installs and maintains an Argo Rollouts installation on a cluster.

Argo Rollouts includes the following key components:

* Argo Rollouts controller  
* Argo Rollouts kubectl plugin  
* CRDs: Rollout, AnalysisRun, Cluster/AnalysisTemplate, Experiment

The Argo Rollouts operator includes the following key components:

* Argo Rollouts operator  
* CRDs: RolloutManager  
* OpenShift Routes Argo Rollouts Traffic Management Plugin (https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-openshift)

## Scope

The scope of this document is the Argo Rollouts Manager operator and the Argo Rollouts instances it manages.

**Out of scope:**

The following is out of scope for this Argo Rollouts/operator security model:

* **Argo CD**: Argo Rollouts and Argo CD are separate, unrelated projects, which share minimal code. 'Argo' refers to the name of the upstream group which manages both projects. (Similar to ‘Apache’, for the Apache foundation.)

## Components

Argo Rollouts supports two installation mechanisms, which control the level of access the Argo Rollouts instance has:

* **Cluster-scoped Argo Rollouts**: Argo Rollouts controller has RBAC access to the full cluster (albeit only minimal resources required)  
* **Namespace-scoped Argo Rollouts**: Argo Rollouts controller has RBAC access only to the Namespace it is installed in.

Cluster-scoped Argo Rollouts requires significant Kubernetes RBAC access to the cluster:

* Cluster-wide CRUD access to: ReplicaSets, Deployments, PodTemplates, Services, Pods, Ingresses, Jobs, Routes, various service mesh CRDs (e.g. istio)  
* Cluster-wide read access to: Secrets, ConfigMaps

Argo Rollouts operator requires significant Kubernetes RBAC access to the cluster:

* A superset of above, in order to grant the above.

### Namespace-scoped Argo Rollouts

* Namespace scope permissions are limited to a single Namespace (that is, restricted via Roles/RoleBindings), and thus do not have the same requirements as above.  
* Namespace-scoped Argo Rollouts installs are the safest, but also the least practical:  
  * They require you to maintain an instance of Argo Rollouts for each Namespace for which you are deploying.  
  * E.g. if you are deploying to 50 different namespaces, this is 50 Argo Rollouts instance (one per namespace!) to maintain.  
* For this reason, cluster-scoped Argo Rollouts installs are likely used by users the vast majority of the time.

### Cluster-scoped Argo Rollouts

* Due to the high level of privileges that comes with a cluster-scoped Argo Rollouts instance, the following security mechanism exists within the Argo Rollouts operator:  
* To install a cluster-scoped Argo Rollouts instance within a namespace, all of the following conditions must be met:  
  * `CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES=(comma separated list of namespaces)` must be defined as an environment variable on the Argo Rollouts operator deployment (for example, via `.spec.config.env` on an OLM Subscription CR when installed through OLM)  
    * This restricts the list of Namespaces which are allowed to contain cluster-scoped Argo Rollouts installs  
  * `NAMESPACE_SCOPED_ARGO_ROLLOUTS` must be false (or not defined) in the that operator environment configuration  
  * The namespace of that cluster-scoped Argo Rollouts install (via RolloutManager CR) must be in the `CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES` list.  
  * The `RolloutManager` CR `.spec.namespaceScoped` field must be 'false'  
  * There can exist only 0 or 1 cluster-scoped Argo Rollouts instance per cluster.
  * If 1 cluster-scoped Argo Rollouts instance exists, there must exist 0 namespace-scoped instances. (That is, the modes are mutually exclusive)


Described further here:

* https://argo-rollouts-manager.readthedocs.io/en/latest/usage/getting_started/#namespace-scoped-rollouts-instance  
* https://argo-rollouts-manager.readthedocs.io/en/latest/usage/getting_started/#cluster-scoped-rollouts-instance

### Plugins

Argo Rollouts includes a plugin mechanism which can be used to customize Argo Rollouts. Two types of plugins exist:

* **Traffic Management plugins**: Add support for new traffic managers. An example of a traffic manager is Istio, AWS ALB, NGINX, Traefik, etc. (https://argo-rollouts.readthedocs.io/en/stable/features/traffic-management/plugins/)  
* **Metrics plugins**: Add support for new metrics providers. An example of metrics provider is Prometheus, Datadog, etc. (https://argo-rollouts.readthedocs.io/en/stable/analysis/plugins/)

The specific set of plugins to install into Argo Rollouts are defined by the administrator of the Argo Rollouts instance. The list of plugins to install are specified via `RolloutManager` CR `.spec.plugins` field.

Plugins are executable binaries which are installed into the Argo Rollouts container, and run alongside the Argo Rollouts controller. Argo Rollouts uses a GRPC-based plugin mechanism to communicate with those plugins.

Plugins gain the same level of K8s cluster privileges/RBAC as the Argo Rollouts instance itself, and thus, users should be careful to only install trusted plugins.

Communication between plugin and controller is limited to intra-container communication.

The operator supports the plugin mechanism. Users are responsible for evaluating and trusting any third-party plugins they install.

### API: Argo Rollouts

Argo Rollouts does not expose a public API, nor any UI. No REST/HTTP/GRPC/etc API. The only TCP/IP port that Argo Rollouts exposes (via a K8s Service) is metrics/health endpoint, for which health/Prometheus (et al) metrics can be scraped. This Service is NOT exposed externally (e.g. via Ingress or Route). ([link](https://argo-rollouts.readthedocs.io/en/stable/best-practices/#there-is-no-argo-rollouts-api))

To interact with Argo Rollouts, one creates/modifies/updates/deletes Argo Rollouts CRs on the Kubernetes control plane. Users of Argo Rollouts are thus expected (and required) to have K8s API access to the cluster.

For example:

* To deploy an Application using Argo Rollouts, the user would create a Rollout CR in the namespace, via e.g. `kubectl create rolloutmanager/my-app -n (namespace) -f (...)`
* To see the status of the deployment, the user would look at .status field of Rollout CR via e.g. `kubectl get rolloutmanager/my-app -o yaml -n (namespace)`  
* Likewise to delete an Application e.g. `kubectl delete rolloutmanager/my-app`.

A user’s privilege to use Argo Rollouts resources is thus equivalent to their K8s RBAC API access to those resources. This is similar to other deployment mechanisms, such as K8s Deployments/ReplicaSets/Jobs/etc.

Other mechanisms exist to interact with Argo Rollouts resources (e.g. Argo CD), but they all use the K8s API described above.

### API: Argo Rollouts Operator

As above, no public API, no UI. Only metrics/health endpoints which are not exposed externally.

### Attack Surface

Since (cluster-scoped) Argo Rollouts has cluster-wide permissions (and so does the Argo Rollouts operator), a full compromise of Argo Rollouts would likely compromise the cluster itself.

This might take one of the following forms:

* A) User gains access to ServiceAccount credentials used by Argo Rollouts or Argo Rollouts operator
* B) Users gains execute permission within the Argo Rollouts container, which can then be further exploited via A)
* C) User gains execute permission within the Argo Rollouts OS process itself (via e.g. buffer overflow, but unlikely as its Go-based), and which can then be further exploited via A)
* D) User provides API input to Rollouts (e.g. malicious values within  RolloutManager/AnalysisTemplate/AnalysisRun CRs) that:
  * Cause Rollouts to leak secrets (for example, via .status field, or log statements)
  * Allow users to modify other K8s resources (ReplicaSets, Pods, etc) they themselves do not have access to, potentially allowing DOS, injection of malicious workloads, or compromise of credentials.
  * Other malicious behaviour

Both within Argo Rollouts itself, and the Argo Rollouts operator, we (and the upstream open source community) have worked to ensure this attack surface is eliminated and/or mitigated.

### Exceptions to namespace-scoping of Argo Rollouts CRs

Argo Rollouts logic and CRs are largely restricted to operating within the same namespace that the Rollouts CR is defined. For example, if you create a Rollout in namespace ‘a’, the expectation is that that Rollout will only touch ReplicaSets/Ingresses/etc in namespace ‘a’.

However, there are a few exceptions to this, described below.

#### feat: allow analysis run to use separate kubeconfig for jobs (\#[3350](https://github.com/argoproj/argo-rollouts/pull/3350))

The administrator of an Argo Rollouts instance may set `ARGO_ROLLOUTS_ANALYSIS_JOB_KUBECONFIG` and `ARGO_ROLLOUTS_ANALYSIS_JOB_NAMESPACE` environment variables on an Argo Rollouts instance. When set, this controls the namespace that metrics jobs run in allowing it to be different than the parent `AnalysisRun` CR)

Analysis:

* We do not currently expose this feature via **RolloutManager** CR, but that does not prevent a user from setting these env vars manually via **.spec.env** in RolloutManager CR  
  * This feature is available in upstream Argo Rollouts and may be configured via the `.spec.env` field in RolloutManager CR, even though it is not explicitly exposed in the RolloutManager CRD.  
* Namespace-scoped Argo Rollouts instance:  
  * Setting a different namespace in `ARGO_ROLLOUTS_ANALYSIS_JOB_NAMESPACE`, does not allow namespace-scoped Argo CD instances to escape their boundary.  
  * Setting a different namespace than the one containing the RolloutsManager CR will cause the controller to throw an ‘unable to create Job in namespace’ error (but not break security boundaries)  
* Cluster-scoped Argo Rollouts instance:  
  * As a consequence of this feature, a user that can set `.spec` field in `RolloutManager` CR has the ability to create Jobs in any namespace.  
  * But, a user that can create or edit the `.spec` field of a RolloutManager is expected to already have this permission.  
  * So this is not a privilege escalation: this is why we restrict which namespaces cluster-scoped Argo Rollouts instances can appear in via `CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES` described above.   
* In every case, setting `ARGO_ROLLOUTS_ANALYSIS_JOB_KUBECONFIG` to some value, it is assumed that the kubeconfig you are providing contains credentials which you already have access to.

#### Ability to reference other Istio VirtualServices in another namespaces

For Istio traffic routing, a Rollout can reference a VirtualService in another namespace by naming it:
```yaml
virtualService:
  name: rollout-vsvc.other-namespace   # format: <vsvcName>.<vsvcNamespace>
```

If omitted, the VirtualService is assumed to be in the Rollout’s namespace.

An LLM-based analysis notes the following (which I have not manually vetted as of this writing):
- The target VS routes must match the Rollout's stable/canary service host names, or validation fails.
- The attacker still needs Rollout create/update in their namespace.
- Impact is traffic manipulation (weight shifting, route changes), not arbitrary code execution.
- In namespaced controller installs (--namespaced), the controller Role is namespace-scoped, so cross-namespace VS updates should fail at the API server with Forbidden — unless extra cross-namespace RBAC was granted manually.


## Security Analysis

### Economy of Mechanism

* Argo Rollouts operator  
  * As described elsewhere, cluster-scoped Argo Rollouts instances operate with high privileges  
  * Argo Rollouts operator thus requires cluster administrators to explicitly enable and define valid cluster-scoped instance namespaces via operator environment configuration.  
  * Since operator deployment configuration is controlled by cluster administrators, this ensures that cluster-scoped Argo Rollouts instances are only ever installed within admin-blessed namespaces.  
* Argo Rollouts  
  * Namespaces are the security boundary for Rollouts CRs  
    * The Argo Rollouts CRs (Rollout/AnalysisRun et al) are scoped to a single Namespace. A Rollout cannot create/access resources in namespaces other than the one in which it exists.  
    * Uses K8s API and RBAC, thus not required to implement its own  
    * Minimal configuration tunables, only 1 related to security: cluster-scoped enabled/disabled (which is controlled via operator)  
* Both:  
  * No external API

### Secure by Design, Default, and Deployment

* Argo Rollouts operator  
  * Secure out of the box:  
    * Does not expose any API endpoints  
    * Requires explicit administrator configuration of allowed namespaces for cluster-scoped Argo Rollouts instances.  
* Argo Rollouts  
  * Secure out of the box:  
    * Minimal configuration: the only security related setting is namespace/cluster-scoped.  
    * Does not expose any API endpoints  
    * Argo Rollouts CRs (Rollout, AnalysisTemplate, etc) are namespace-scoped  
    * Installing Argo Rollouts on a cluster does not grant users of that cluster any additional permissions than they had previously.

### Open Design

* Argo Rollouts operator  
  * As of this writing, Argo Rollouts operator has none of its own sensitive data (input Secrets, etc).  
  * Likewise, it does not expose any external API endpoints.  
* Argo Rollouts  
  * The Argo Rollouts CR behaves similarly to the standard K8s Deployment CR. An understanding of Deployment CR shape/behavior will easily translate to Argo Rollouts.  
  * All confidential data that Argo Rollouts interacts with is contained with K8s Secrets, and is protected via standard K8s RBAC mechanisms.  
  * Argo Rollouts itself does not have its own k8s secrets.  
  * Likewise, it does not expose any external API endpoints.

### Complete Mediation through Access Control

* Argo Rollouts relies entirely on K8s API, and does not implement its own authentication/authorization.  
* K8s cluster administrators should have no trouble translating their existing understanding of cluster security to Rollouts

### Least Privilege

* Argo Rollouts operator  
  * The only security sensitive resources that Argo Rollouts operator has are those that are granted to it by OLM (Secrets and ServiceAccounts), which are controlled via that mechanism.  
* Argo Rollouts  
  * Argo Rollouts CRs are restricted to operating only within the namespace they are defined. For instance, a Rollout CR (used to deploy an application) in namespace ‘a’ cannot touch any resources (ReplicaSets, Ingresses, etc) in any other namespace, e.g. a namespace ‘b’.

### Least Common Mechanism

* Argo Rollouts operator  
  * The only security sensitive resources that Argo Rollouts operator has are those that are granted to it by OLM (Secrets and ServiceAccounts), which are controlled via that mechanism.  
* Argo Rollouts  
  * Argo Rollouts CRs are restricted to operating only within the namespace they are defined. For instance, a Rollout CR (used to deploy an application) in namespace ‘a’ cannot touch any resources (ReplicaSets, Ingresses, etc) in any other namespace, e.g. a namespace ‘b’.  
  * The only other opportunity for side channel attacks is data transfer between Argo Rollouts container and Argo Rollouts traffic manager/metrics plugin  
  * Plugins communicate via intra-container RPC, so opportunity for attack is limited.

### Psychological Acceptability

* Argo Rollouts operator  
  * With the Argo Rollouts operator, you have one main security choice to make: cluster-scoped or namespace-scoped Argo Rollouts.  
  * With cluster-scoped, we have an additional restriction that forces the administrator to name the specific namespace into which the cluster-scoped Rollouts will be installed.  
  * Between these two points, it should be clear (for the admin) to understand the behaviour of the operator and rollouts instance.  
* Argo Rollouts  
  * Argo Rollouts CRs are namespace-scoped: this makes it easy to reason about their impact. To the best of my knowledge, a Rollouts CR cannot under any circumstance reach outside the Namespace in which it is defined.  
  * Likewise, since Argo Rollouts inherits K8s auth mechanisms, a cluster admin's existing understanding of cluster security should translate to securing Argo Rollouts.

### Compromise Recording

Both Argo Rollouts and Argo Rollouts operator log events via console output and/or K8s Events. Any creation/modification/deletion by those operator/rollouts will be logged to Kubernetes cluster audit logs (when enabled).

### Defense In-Depth

As above, the authentication and privileges largely depend on the existing K8s security mode.

For Argo Rollouts plugins, users can provide SHA256 hash for any plugin binaries, to ensure that they are the expected version and content of that plugin.

## Resources 

### Information Gathering/Resources

The following is a list of resources used during this SAR

| Component | Resource |
| :---- | :---- |
| Argo Rollouts | [https://argo-rollouts.readthedocs.io/en/stable/](https://argo-rollouts.readthedocs.io/en/stable/)  |
| Argo Rollouts Manager operator | [https://argo-rollouts-manager.readthedocs.io/en/latest/](https://argo-rollouts-manager.readthedocs.io/en/latest/)  |
