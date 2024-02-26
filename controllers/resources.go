package rollouts

import (
	"context"
	"fmt"
	"reflect"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconciles rollouts serviceaccount.
func (r *RolloutManagerReconciler) reconcileRolloutsServiceAccount(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) (*corev1.ServiceAccount, error) {

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabels(&sa.ObjectMeta)

	if err := fetchObject(ctx, r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the serviceAccount associated with %s: %s", sa.Name, err)
		}

		if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Creating ServiceAccount %s", sa.Name))
		err := r.Client.Create(ctx, sa)
		if err != nil {
			return nil, err
		}

	}
	return sa, nil
}

// Reconciles rollouts role.
func (r *RolloutManagerReconciler) reconcileRolloutsRole(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) (*rbacv1.Role, error) {

	expectedPolicyRules := GetPolicyRules()

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabels(&role.ObjectMeta)

	if err := fetchObject(ctx, r.Client, cr.Namespace, role.Name, role); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s: %s", role.Name, err)
		}

		if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Creating Role %s", role.Name))
		role.Rules = expectedPolicyRules
		return role, r.Client.Create(ctx, role)
	}

	// Reconcile if the role already exists and modified.
	if !reflect.DeepEqual(role.Rules, expectedPolicyRules) {
		log.Info(fmt.Sprintf("PolicyRules of Role %s do not match the expected state, hence updating it", role.Name))
		role.Rules = expectedPolicyRules
		return role, r.Client.Update(ctx, role)
	}

	return role, nil
}

// Reconciles argo-rollouts clusterRole.
func (r *RolloutManagerReconciler) reconcileRolloutsClusterRole(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) (*rbacv1.ClusterRole, error) {

	expectedPolicyRules := GetPolicyRules()

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultArgoRolloutsResourceName,
		},
	}
	setRolloutsLabels(&clusterRole.ObjectMeta)

	if err := fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the ClusterRole for the ServiceAccount associated with %s: %w", clusterRole.Name, err)
		}

		log.Info(fmt.Sprintf("Creating ClusterRole %s", clusterRole.Name))
		clusterRole.Rules = expectedPolicyRules
		return clusterRole, r.Client.Create(ctx, clusterRole)
	}

	// Reconcile if the clusterRole already exists and modified.
	if !reflect.DeepEqual(clusterRole.Rules, expectedPolicyRules) {
		log.Info(fmt.Sprintf("PolicyRules of ClusterRole %s do not match the expected state, hence updating it", clusterRole.Name))
		clusterRole.Rules = expectedPolicyRules
		return clusterRole, r.Client.Update(ctx, clusterRole)
	}

	return clusterRole, nil
}

// Reconcile rollouts rolebinding.
func (r *RolloutManagerReconciler) reconcileRolloutsRoleBinding(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager, role *rbacv1.Role, sa *corev1.ServiceAccount) error {

	if role == nil {
		return fmt.Errorf("received Role is nil while reconciling RoleBinding")
	}

	if sa == nil {
		return fmt.Errorf("received ServiceAccount is nil while reconciling RoleBinding")
	}

	expectedRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabels(&expectedRoleBinding.ObjectMeta)

	expectedRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	expectedRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	actualRoleBinding := &rbacv1.RoleBinding{}

	// Fetch the rolebinding if exists and store that in actualRoleBinding.
	if err := fetchObject(ctx, r.Client, cr.Namespace, expectedRoleBinding.Name, actualRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s: %s", expectedRoleBinding.Name, err)
		}

		if err := controllerutil.SetControllerReference(cr, expectedRoleBinding, r.Scheme); err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Creating RoleBinding %s", expectedRoleBinding.Name))
		return r.Client.Create(ctx, expectedRoleBinding)
	}

	// Reconcile if the role already exists and modified.
	if !reflect.DeepEqual(expectedRoleBinding.Subjects, actualRoleBinding.Subjects) {
		log.Info(fmt.Sprintf("Subjects of RoleBinding %s do not match the expected state, hence updating it", actualRoleBinding.Name))
		actualRoleBinding.Subjects = expectedRoleBinding.Subjects
		if err := r.Client.Update(ctx, actualRoleBinding); err != nil {
			return err
		}
	}

	return nil
}

// Reconcile rollouts clusterRoleBinding.
func (r *RolloutManagerReconciler) reconcileRolloutsClusterRoleBinding(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager, clusterRole *rbacv1.ClusterRole, sa *corev1.ServiceAccount) error {

	if clusterRole == nil {
		return fmt.Errorf("received ClusterRole is nil while reconciling ClusterRoleBinding")
	}

	if sa == nil {
		return fmt.Errorf("received ServiceAccount is nil while reconciling ClusterRoleBinding")
	}

	expectedClusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultArgoRolloutsResourceName,
		},
	}
	setRolloutsLabels(&expectedClusterRoleBinding.ObjectMeta)

	expectedClusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}

	expectedClusterRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	actualClusterRoleBinding := &rbacv1.ClusterRoleBinding{}

	// Fetch the clusterRoleBinding if exists and store that in actualClusterRoleBinding.
	if err := fetchObject(ctx, r.Client, "", expectedClusterRoleBinding.Name, actualClusterRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the ClusterRoleBinding associated with %s: %s", expectedClusterRoleBinding.Name, err)
		}

		log.Info(fmt.Sprintf("Creating ClusterRoleBinding %s", expectedClusterRoleBinding.Name))
		return r.Client.Create(ctx, expectedClusterRoleBinding)
	}

	// Reconcile if the clusterRoleBinding already exists and modified.
	if !reflect.DeepEqual(expectedClusterRoleBinding.Subjects, actualClusterRoleBinding.Subjects) {
		log.Info(fmt.Sprintf("Subjects of ClusterRoleBinding %s do not match the expected state, hence updating it", expectedClusterRoleBinding.Name))
		actualClusterRoleBinding.Subjects = expectedClusterRoleBinding.Subjects
		if err := r.Client.Update(ctx, actualClusterRoleBinding); err != nil {
			return err
		}
	}

	return nil
}

// Reconciles aggregate-to-admin ClusterRole.
func (r *RolloutManagerReconciler) reconcileRolloutsAggregateToAdminClusterRole(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	var aggregationType string = "aggregate-to-admin"
	name := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	expectedPolicyRules := GetAggregateToAdminPolicyRules()

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	setRolloutsAggregatedClusterRoleLabels(&clusterRole.ObjectMeta, name, aggregationType)

	if err := fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to reconcile the aggregated ClusterRole %s: %s", clusterRole.Name, err)
		}

		log.Info(fmt.Sprintf("Creating aggregated ClusterRole %s", clusterRole.Name))
		clusterRole.Rules = expectedPolicyRules
		return r.Client.Create(ctx, clusterRole)
	}

	// Reconcile if the aggregated role already exists and modified.
	if !reflect.DeepEqual(clusterRole.Rules, expectedPolicyRules) {
		log.Info(fmt.Sprintf("PolicyRules of ClusterRole %s do not match the expected state, hence updating it", clusterRole.Name))
		clusterRole.Rules = expectedPolicyRules
		return r.Client.Update(ctx, clusterRole)
	}

	return nil
}

// Reconciles aggregate-to-edit ClusterRole.
func (r *RolloutManagerReconciler) reconcileRolloutsAggregateToEditClusterRole(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	var aggregationType string = "aggregate-to-edit"
	name := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	expectedPolicyRules := GetAggregateToEditPolicyRules()

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	setRolloutsAggregatedClusterRoleLabels(&clusterRole.ObjectMeta, name, aggregationType)

	if err := fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to reconcile the aggregated ClusterRole %s: %s", clusterRole.Name, err)
		}

		log.Info(fmt.Sprintf("Creating aggregated ClusterRole %s", clusterRole.Name))
		clusterRole.Rules = expectedPolicyRules
		return r.Client.Create(ctx, clusterRole)
	}

	// Reconcile if the aggregated role already exists and modified.
	if !reflect.DeepEqual(clusterRole.Rules, expectedPolicyRules) {
		log.Info(fmt.Sprintf("PolicyRules of ClusterRole %s do not match the expected state, hence updating it", clusterRole.Name))
		clusterRole.Rules = expectedPolicyRules
		return r.Client.Update(ctx, clusterRole)
	}

	return nil
}

// Reconciles aggregate-to-view ClusterRole.
func (r *RolloutManagerReconciler) reconcileRolloutsAggregateToViewClusterRole(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	var aggregationType string = "aggregate-to-view"
	name := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	expectedPolicyRules := GetAggregateToViewPolicyRules()

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	setRolloutsAggregatedClusterRoleLabels(&clusterRole.ObjectMeta, name, aggregationType)

	if err := fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to reconcile the aggregated ClusterRole %s: %s", clusterRole.Name, err)
		}

		log.Info(fmt.Sprintf("Creating aggregated ClusterRole %s", clusterRole.Name))
		clusterRole.Rules = expectedPolicyRules
		return r.Client.Create(ctx, clusterRole)
	}

	// Reconcile if the aggregated role already exists and modified.
	if !reflect.DeepEqual(clusterRole.Rules, expectedPolicyRules) {
		log.Info(fmt.Sprintf("PolicyRules of ClusterRole %s do not match the expected state, hence updating it", clusterRole.Name))
		clusterRole.Rules = expectedPolicyRules
		return r.Client.Update(ctx, clusterRole)
	}

	return nil
}

// Reconcile rollouts metrics service.
func (r *RolloutManagerReconciler) reconcileRolloutsMetricsService(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	expectedSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsMetricsServiceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabels(&expectedSvc.ObjectMeta)
	// overwrite the annotations for rollouts metrics service
	expectedSvc.ObjectMeta.Labels["app.kubernetes.io/name"] = DefaultArgoRolloutsMetricsServiceName
	expectedSvc.ObjectMeta.Labels["app.kubernetes.io/component"] = "server"

	expectedSvc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8090,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8090),
		},
	}

	expectedSvc.Spec.Selector = map[string]string{
		DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName,
	}

	actualSvc := &corev1.Service{}

	// Fetch the service if exists and store that in actualSvc.
	if err := fetchObject(ctx, r.Client, cr.Namespace, expectedSvc.Name, actualSvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the service %s: %s", expectedSvc.Name, err)
		}

		if err := controllerutil.SetControllerReference(cr, expectedSvc, r.Scheme); err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Creating Service %s", expectedSvc.Name))
		return r.Client.Create(ctx, expectedSvc)
	}

	if !reflect.DeepEqual(actualSvc.Spec.Ports, expectedSvc.Spec.Ports) {
		log.Info(fmt.Sprintf("Ports of Service %s do not match the expected state, hence updating it", actualSvc.Name))
		actualSvc.Spec.Ports = expectedSvc.Spec.Ports
		return r.Client.Create(ctx, actualSvc)
	}

	return nil
}

// Reconciles secrets for rollouts controller
func (r *RolloutManagerReconciler) reconcileRolloutsSecrets(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultRolloutsNotificationSecretName,
			Namespace: cr.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}

	if err := fetchObject(ctx, r.Client, cr.Namespace, secret.Name, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the Secret %s: %s", secret.Name, err)
		}

		if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Creating Secret %s", secret.Name))
		return r.Client.Create(ctx, secret)
	}

	// secret found, do nothing
	return nil
}

// Deletes rollout resources when the corresponding rollout CR is deleted.
//
// TODO: Remove the nolint:all once this function is called
//
//nolint:unused
func (r *RolloutManagerReconciler) deleteRolloutResources(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	if cr.DeletionTimestamp != nil {
		log.Info(fmt.Sprintf("Argo Rollout resource in %s namespace is deleted, Deleting the Argo Rollout workloads",
			cr.Namespace))

		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: cr.Namespace,
			},
		}

		if err := r.Client.Delete(ctx, serviceAccount); err != nil {
			log.Error(err, fmt.Sprintf("Error deleting the service account %s in %s",
				DefaultArgoRolloutsResourceName, cr.Namespace))
		}

		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: cr.Namespace,
			},
		}
		if err := r.Client.Delete(ctx, role); err != nil {
			log.Error(err, fmt.Sprintf("Error deleting role %s in %s",
				DefaultArgoRolloutsResourceName, cr.Namespace))
		}

		rolebinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: cr.Namespace,
			},
		}
		if err := r.Client.Delete(ctx, rolebinding); err != nil {
			log.Error(err, fmt.Sprintf("Error deleting the rolebinding %s in %s",
				DefaultArgoRolloutsResourceName, cr.Namespace))
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultRolloutsNotificationSecretName,
				Namespace: cr.Namespace,
			},
		}
		if err := r.Client.Delete(ctx, secret); err != nil {
			log.Error(err, fmt.Sprintf("Error deleting the secret %s in %s",
				DefaultRolloutsNotificationSecretName, cr.Namespace))
		}

		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: cr.Namespace,
			},
		}
		if err := r.Client.Delete(ctx, deploy); err != nil {
			log.Error(err, fmt.Sprintf("Error deleting the deployment %s in %s",
				DefaultArgoRolloutsResourceName, cr.Namespace))
		}

		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: cr.Namespace,
			},
		}
		if err := r.Client.Delete(ctx, svc); err != nil {
			log.Error(err, fmt.Sprintf("Error deleting the service %s in %s",
				DefaultArgoRolloutsResourceName, cr.Namespace))
		}
	}

	return nil
}

func setRolloutsAggregatedClusterRoleLabels(obj *metav1.ObjectMeta, name string, aggregationType string) {

	obj.Labels = map[string]string{}
	obj.Labels["app.kubernetes.io/component"] = "aggregate-cluster-role"
	obj.Labels["app.kubernetes.io/name"] = name
	obj.Labels["app.kubernetes.io/part-of"] = DefaultArgoRolloutsResourceName
	obj.Labels["rbac.authorization.k8s.io/"+aggregationType] = "true"
}

// getPolicyRules returns the policy rules for Argo Rollouts Role.
func GetPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/status",
				"rollouts/finalizers",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"analysisruns",
				"analysisruns/finalizers",
				"experiments",
				"experiments/finalizers",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"analysistemplates",
				"clusteranalysistemplates",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"replicasets",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
				"apps",
			},
			Resources: []string{
				"deployments",
				"podtemplates",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"services",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"patch",
				"create",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"coordination.k8s.io",
			},
			Resources: []string{
				"leases",
			},
			Verbs: []string{
				"create",
				"get",
				"update",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"list",
				"update",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods/eviction",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"networking.k8s.io",
				"extensions",
			},
			Resources: []string{
				"ingresses",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"networking.istio.io",
			},
			Resources: []string{
				"virtualservices",
				"destinationrules",
			},
			Verbs: []string{
				"watch",
				"get",
				"update",
				"patch",
				"list",
			},
		},
		{
			APIGroups: []string{
				"split.smi-spec.io",
			},
			Resources: []string{
				"trafficsplits",
			},
			Verbs: []string{
				"create",
				"watch",
				"get",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"getambassador.io",
				"x.getambassador.io",
			},
			Resources: []string{
				"mappings",
				"ambassadormappings",
			},
			Verbs: []string{
				"create",
				"watch",
				"get",
				"update",
				"list",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"endpoints",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"elbv2.k8s.aws",
			},
			Resources: []string{
				"targetgroupbindings",
			},
			Verbs: []string{
				"list",
				"get",
			},
		},
		{
			APIGroups: []string{
				"appmesh.k8s.aws",
			},
			Resources: []string{
				"virtualservices",
			},
			Verbs: []string{
				"watch",
				"get",
				"list",
			},
		},
		{
			APIGroups: []string{
				"appmesh.k8s.aws",
			},
			Resources: []string{
				"virtualnodes",
				"virtualrouters",
			},
			Verbs: []string{
				"watch",
				"get",
				"list",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"traefik.containo.us",
			},
			Resources: []string{
				"traefikservices",
			},
			Verbs: []string{
				"watch",
				"get",
				"update",
			},
		},
		{
			APIGroups: []string{
				"apisix.apache.org",
			},
			Resources: []string{
				"apisixroutes",
			},
			Verbs: []string{
				"watch",
				"get",
				"update",
			},
		},
	}
}

// Returns PolicyRules for the Cluster Role argo-rollouts-aggregate-to-admin
func GetAggregateToAdminPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/scale",
				"rollouts/status",
				"experiments",
				"analysistemplates",
				"clusteranalysistemplates",
				"analysisruns",
			},
			Verbs: []string{
				"create",
				"delete",
				"deletecollection",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
	}
}

// Returns PolicyRules for the Cluster Role argo-rollouts-aggregate-to-edit
func GetAggregateToEditPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/scale",
				"rollouts/status",
				"experiments",
				"analysistemplates",
				"clusteranalysistemplates",
				"analysisruns",
			},
			Verbs: []string{
				"create",
				"delete",
				"deletecollection",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
	}
}

// Returns PolicyRules for the Cluster Role argo-rollouts-aggregate-to-view
func GetAggregateToViewPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/scale",
				"experiments",
				"analysistemplates",
				"clusteranalysistemplates",
				"analysisruns",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
}
