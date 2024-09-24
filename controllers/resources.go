package rollouts

import (
	"context"
	"fmt"
	"reflect"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconciles Rollouts ServiceAccount.
func (r *RolloutManagerReconciler) reconcileRolloutsServiceAccount(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) (*corev1.ServiceAccount, error) {
	expectedServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabelsAndAnnotationsToObject(&expectedServiceAccount.ObjectMeta, cr)

	liveServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: expectedServiceAccount.Name, Namespace: expectedServiceAccount.Namespace}}
	if err := fetchObject(ctx, r.Client, cr.Namespace, liveServiceAccount.Name, liveServiceAccount); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the ServiceAccount associated with %s: %w", liveServiceAccount.Name, err)
		}

		if err := controllerutil.SetControllerReference(&cr, expectedServiceAccount, r.Scheme); err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Creating ServiceAccount %s", expectedServiceAccount.Name))
		return expectedServiceAccount, r.Client.Create(ctx, expectedServiceAccount)
	}

	updateNeeded := false

	normalizedLiveServiceAccount := liveServiceAccount.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveServiceAccount.ObjectMeta, cr)

	if !reflect.DeepEqual(normalizedLiveServiceAccount.Labels, expectedServiceAccount.Labels) || !reflect.DeepEqual(normalizedLiveServiceAccount.Annotations, expectedServiceAccount.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of ServiceAccount %s do not match the expected state, hence updating it", liveServiceAccount.Name))

		liveServiceAccount.Labels = combineStringMaps(liveServiceAccount.Labels, expectedServiceAccount.Labels)
		liveServiceAccount.Annotations = combineStringMaps(liveServiceAccount.Annotations, expectedServiceAccount.Annotations)
	}

	if updateNeeded {
		// Update if the Role already exists and needs to be modified
		return liveServiceAccount, r.Client.Update(ctx, liveServiceAccount)
	}

	return liveServiceAccount, nil
}

// Reconciles Rollouts Role.
func (r *RolloutManagerReconciler) reconcileRolloutsRole(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) (*rbacv1.Role, error) {
	expectedPolicyRules := GetPolicyRules()

	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabelsAndAnnotationsToObject(&expectedRole.ObjectMeta, cr)

	liveRole := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: expectedRole.Name, Namespace: expectedRole.Namespace}}

	if err := fetchObject(ctx, r.Client, cr.Namespace, liveRole.Name, liveRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the Role for the ServiceAccount associated with %s: %w", liveRole.Name, err)
		}

		if err = controllerutil.SetControllerReference(&cr, expectedRole, r.Scheme); err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Creating Role %s", expectedRole.Name))
		expectedRole.Rules = expectedPolicyRules
		return expectedRole, r.Client.Create(ctx, expectedRole)
	}

	updateNeeded := false

	if !reflect.DeepEqual(liveRole.Rules, expectedPolicyRules) {
		updateNeeded = true

		log.Info(fmt.Sprintf("PolicyRules of Role %s do not match the expected state, hence updating it", liveRole.Name))
		liveRole.Rules = expectedPolicyRules
	}

	normalizedLiveRole := liveRole.DeepCopy()

	removeUserLabelsAndAnnotations(&normalizedLiveRole.ObjectMeta, cr)

	if !reflect.DeepEqual(normalizedLiveRole.Labels, expectedRole.Labels) || !reflect.DeepEqual(normalizedLiveRole.Annotations, expectedRole.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of Role %s do not match the expected state, hence updating it", liveRole.Name))

		liveRole.Labels = combineStringMaps(liveRole.Labels, expectedRole.Labels)
		liveRole.Annotations = combineStringMaps(liveRole.Annotations, expectedRole.Annotations)
	}

	if updateNeeded {
		// Update if the Role already exists and needs to be modified
		return liveRole, r.Client.Update(ctx, liveRole)
	}

	return liveRole, nil
}

// Reconciles Rollouts ClusterRole.
func (r *RolloutManagerReconciler) reconcileRolloutsClusterRole(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) (*rbacv1.ClusterRole, error) {
	expectedPolicyRules := GetPolicyRules()

	expectedClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultArgoRolloutsResourceName,
		},
	}
	setRolloutsLabelsAndAnnotationsToObject(&expectedClusterRole.ObjectMeta, cr)
	liveClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: expectedClusterRole.Name, Namespace: expectedClusterRole.Namespace}}
	if err := fetchObject(ctx, r.Client, "", liveClusterRole.Name, liveClusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to Reconcile the ClusterRole for the ServiceAccount associated with %s: %w", liveClusterRole.Name, err)
		}

		log.Info(fmt.Sprintf("Creating ClusterRole %s", liveClusterRole.Name))
		expectedClusterRole.Rules = expectedPolicyRules
		return expectedClusterRole, r.Client.Create(ctx, expectedClusterRole)
	}

	updateNeeded := false

	if !reflect.DeepEqual(liveClusterRole.Rules, expectedPolicyRules) {
		updateNeeded = true
		log.Info(fmt.Sprintf("PolicyRules of ClusterRole %s do not match the expected state, hence updating it", liveClusterRole.Name))
		liveClusterRole.Rules = expectedPolicyRules
	}

	normalizedLiveClusterRole := liveClusterRole.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveClusterRole.ObjectMeta, cr)

	if !reflect.DeepEqual(normalizedLiveClusterRole.Labels, expectedClusterRole.Labels) || !reflect.DeepEqual(normalizedLiveClusterRole.Annotations, expectedClusterRole.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of Role %s do not match the expected state, hence updating it", liveClusterRole.Name))

		liveClusterRole.Labels = combineStringMaps(liveClusterRole.Labels, expectedClusterRole.Labels)
		liveClusterRole.Annotations = combineStringMaps(liveClusterRole.Annotations, expectedClusterRole.Annotations)
	}

	if updateNeeded {
		// Update if the ClusterRole already exists and needs to be modified
		return liveClusterRole, r.Client.Update(ctx, liveClusterRole)
	}
	return liveClusterRole, nil
}

// Reconcile Rollouts RoleBinding.
func (r *RolloutManagerReconciler) reconcileRolloutsRoleBinding(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager, role *rbacv1.Role, sa *corev1.ServiceAccount) error {

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
	setRolloutsLabelsAndAnnotationsToObject(&expectedRoleBinding.ObjectMeta, cr)

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

	// Fetch the RoleBinding if exists and store that in actualRoleBinding.
	liveRoleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: expectedRoleBinding.Name, Namespace: expectedRoleBinding.Namespace}}
	if err := fetchObject(ctx, r.Client, cr.Namespace, liveRoleBinding.Name, liveRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the RoleBinding associated with %s: %w", expectedRoleBinding.Name, err)
		}

		if err := controllerutil.SetControllerReference(&cr, expectedRoleBinding, r.Scheme); err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Creating RoleBinding %s", expectedRoleBinding.Name))
		return r.Client.Create(ctx, expectedRoleBinding)
	}

	updateNeeded := false

	// Reconcile if the RoleBinding already exists and modified.
	if !reflect.DeepEqual(expectedRoleBinding.Subjects, liveRoleBinding.Subjects) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Subjects of RoleBinding %s do not match the expected state, hence updating it", liveRoleBinding.Name))
		liveRoleBinding.Subjects = expectedRoleBinding.Subjects

	}

	normalizedLiveRoleBinding := liveRoleBinding.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveRoleBinding.ObjectMeta, cr)
	if !reflect.DeepEqual(normalizedLiveRoleBinding.Labels, expectedRoleBinding.Labels) || !reflect.DeepEqual(normalizedLiveRoleBinding.Annotations, expectedRoleBinding.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of RoleBinding %s do not match the expected state, hence updating it", liveRoleBinding.Name))

		liveRoleBinding.Labels = combineStringMaps(liveRoleBinding.Labels, expectedRoleBinding.Labels)
		liveRoleBinding.Annotations = combineStringMaps(liveRoleBinding.Annotations, expectedRoleBinding.Annotations)
	}

	if updateNeeded {
		// Update if the RoleBinding already exists and needs to be modified
		if err := r.Client.Update(ctx, liveRoleBinding); err != nil {
			return err
		}
	}

	return nil
}

// Reconcile Rollouts ClusterRoleBinding.
func (r *RolloutManagerReconciler) reconcileRolloutsClusterRoleBinding(ctx context.Context, clusterRole *rbacv1.ClusterRole, sa *corev1.ServiceAccount, cr rolloutsmanagerv1alpha1.RolloutManager) error {

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
	setRolloutsLabelsAndAnnotationsToObject(&expectedClusterRoleBinding.ObjectMeta, cr)

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

	// Fetch the ClusterRoleBinding if exists and store that in actualClusterRoleBinding.
	liveClusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: expectedClusterRoleBinding.Name}}
	if err := fetchObject(ctx, r.Client, "", liveClusterRoleBinding.Name, liveClusterRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the ClusterRoleBinding associated with %s: %w", expectedClusterRoleBinding.Name, err)
		}

		log.Info(fmt.Sprintf("Creating ClusterRoleBinding %s", expectedClusterRoleBinding.Name))
		return r.Client.Create(ctx, expectedClusterRoleBinding)
	}

	updateNeeded := false

	if !reflect.DeepEqual(expectedClusterRoleBinding.Subjects, liveClusterRoleBinding.Subjects) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Subjects of ClusterRoleBinding %s do not match the expected state, hence updating it", expectedClusterRoleBinding.Name))
		liveClusterRoleBinding.Subjects = expectedClusterRoleBinding.Subjects
	}

	normalizedLiveClusterRoleBinding := liveClusterRoleBinding.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveClusterRoleBinding.ObjectMeta, cr)
	if !reflect.DeepEqual(normalizedLiveClusterRoleBinding.Labels, expectedClusterRoleBinding.Labels) || !reflect.DeepEqual(normalizedLiveClusterRoleBinding.Annotations, expectedClusterRoleBinding.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of ClusterRoleBinding %s do not match the expected state, hence updating it", liveClusterRoleBinding.Name))

		liveClusterRoleBinding.Labels = combineStringMaps(liveClusterRoleBinding.Labels, expectedClusterRoleBinding.Labels)
		liveClusterRoleBinding.Annotations = combineStringMaps(liveClusterRoleBinding.Annotations, expectedClusterRoleBinding.Annotations)
	}

	if updateNeeded {
		// Update if the ClusterRoleBinding already exists and needs to be modified
		if err := r.Client.Update(ctx, liveClusterRoleBinding); err != nil {
			return err
		}
	}

	return nil
}

// removeClusterScopedResourcesIfApplicable will remove the ClusterRole and ClusterRoleBinding that are created when a cluster-scoped RolloutManager is created.
func (r *RolloutManagerReconciler) removeClusterScopedResourcesIfApplicable(ctx context.Context) error {

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultArgoRolloutsResourceName,
		},
	}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRole), clusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error on retrieving rollouts ClusterRole")
			return err
		}
		// ClusterRole doesn't exist, which is the desired state.
	} else {
		// ClusterRole does exist, so delete it.
		log.Info("deleting Rollouts ClusterRole for RolloutManager that no longer exists")
		if err := r.Client.Delete(ctx, clusterRole); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	// List of ClusterRoles '*aggregate*' to delete
	clusterRoleSuffixes := []string{"aggregate-to-admin", "aggregate-to-edit", "aggregate-to-view"}

	// Iterate over each ClusterRole '*aggregate*' and delete if it exists
	for _, suffix := range clusterRoleSuffixes {
		roleName := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, suffix)

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleName,
			},
		}

		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRole), clusterRole); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "error on retrieving ClusterRole", "name", roleName)
				return err
			}
			// ClusterRole '*aggregate*' doesn't exist, which is the desired state.
		} else {
			// ClusterRole '*aggregate*' does exist, so delete it.
			log.Info("deleting ClusterRole", "name", roleName)
			if err := r.Client.Delete(ctx, clusterRole); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
			}
		}
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultArgoRolloutsResourceName,
		},
	}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error on retrieving rollouts ClusterRoleBinding")
			return err
		}
		// ClusterRoleBinding doesn't exist, which is the desired state.
	} else {
		// ClusterRoleBinding does exist, so delete it.
		log.Info("deleting Rollouts ClusterRoleBinding for RolloutManager that no longer exists")
		if err := r.Client.Delete(ctx, clusterRoleBinding); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// Reconciles aggregate-to-admin ClusterRole.
func (r *RolloutManagerReconciler) reconcileRolloutsAggregateToAdminClusterRole(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) error {

	var aggregationType string = "aggregate-to-admin"
	name := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	expectedPolicyRules := GetAggregateToAdminPolicyRules()

	expectedClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	setRolloutsAggregatedClusterRoleLabels(&expectedClusterRole.ObjectMeta, name, aggregationType)
	setAdditionalRolloutsLabelsAndAnnotationsToObject(&expectedClusterRole.ObjectMeta, cr)

	liveClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: expectedClusterRole.Name}}
	if err := fetchObject(ctx, r.Client, "", liveClusterRole.Name, liveClusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to reconcile the aggregated ClusterRole %s: %w", liveClusterRole.Name, err)
		}

		log.Info(fmt.Sprintf("Creating aggregated ClusterRole %s", liveClusterRole.Name))
		expectedClusterRole.Rules = expectedPolicyRules
		return r.Client.Create(ctx, expectedClusterRole)
	}

	updateNeeded := false

	if !reflect.DeepEqual(liveClusterRole.Rules, expectedPolicyRules) {
		updateNeeded = true
		log.Info(fmt.Sprintf("PolicyRules of ClusterRole %s do not match the expected state, hence updating it", liveClusterRole.Name))
		liveClusterRole.Rules = expectedPolicyRules
	}

	normalizedLiveClusterRole := liveClusterRole.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveClusterRole.ObjectMeta, cr)
	if !reflect.DeepEqual(normalizedLiveClusterRole.Labels, expectedClusterRole.Labels) || !reflect.DeepEqual(normalizedLiveClusterRole.Annotations, expectedClusterRole.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of aggregated ClusterRole %s do not match the expected state, hence updating it", liveClusterRole.Name))

		liveClusterRole.Labels = combineStringMaps(liveClusterRole.Labels, expectedClusterRole.Labels)
		liveClusterRole.Annotations = combineStringMaps(liveClusterRole.Annotations, expectedClusterRole.Annotations)
	}

	if updateNeeded {
		// Update if the aggregated ClusterRole already exists and needs to be modified
		return r.Client.Update(ctx, liveClusterRole)
	}
	return nil
}

// Reconciles aggregate-to-edit ClusterRole.
func (r *RolloutManagerReconciler) reconcileRolloutsAggregateToEditClusterRole(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) error {

	var aggregationType string = "aggregate-to-edit"
	name := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	expectedPolicyRules := GetAggregateToEditPolicyRules()

	expectedClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	setRolloutsAggregatedClusterRoleLabels(&expectedClusterRole.ObjectMeta, name, aggregationType)
	setAdditionalRolloutsLabelsAndAnnotationsToObject(&expectedClusterRole.ObjectMeta, cr)

	liveClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: expectedClusterRole.Name}}
	if err := fetchObject(ctx, r.Client, "", liveClusterRole.Name, liveClusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to reconcile the aggregated ClusterRole %s: %w", liveClusterRole.Name, err)
		}

		log.Info(fmt.Sprintf("Creating aggregated ClusterRole %s", expectedClusterRole.Name))
		expectedClusterRole.Rules = expectedPolicyRules
		return r.Client.Create(ctx, expectedClusterRole)
	}

	updateNeeded := false

	if !reflect.DeepEqual(liveClusterRole.Rules, expectedPolicyRules) {
		updateNeeded = true
		log.Info(fmt.Sprintf("PolicyRules of ClusterRole %s do not match the expected state, hence updating it", liveClusterRole.Name))
		liveClusterRole.Rules = expectedPolicyRules
	}

	normalizedLiveClusterRole := liveClusterRole.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveClusterRole.ObjectMeta, cr)
	if !reflect.DeepEqual(normalizedLiveClusterRole.Labels, expectedClusterRole.Labels) || !reflect.DeepEqual(normalizedLiveClusterRole.Annotations, expectedClusterRole.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of aggregated ClusterRole %s do not match the expected state, hence updating it", liveClusterRole.Name))

		liveClusterRole.Labels = combineStringMaps(liveClusterRole.Labels, expectedClusterRole.Labels)
		liveClusterRole.Annotations = combineStringMaps(liveClusterRole.Annotations, expectedClusterRole.Annotations)
	}

	if updateNeeded {
		// Update if the aggregated ClusterRole already exists and needs to be modified
		return r.Client.Update(ctx, liveClusterRole)
	}
	return nil
}

// Reconciles aggregate-to-view ClusterRole.
func (r *RolloutManagerReconciler) reconcileRolloutsAggregateToViewClusterRole(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) error {

	var aggregationType string = "aggregate-to-view"
	name := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	expectedPolicyRules := GetAggregateToViewPolicyRules()

	expectedClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	setRolloutsAggregatedClusterRoleLabels(&expectedClusterRole.ObjectMeta, name, aggregationType)
	setAdditionalRolloutsLabelsAndAnnotationsToObject(&expectedClusterRole.ObjectMeta, cr)

	liveClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: expectedClusterRole.Name, Namespace: expectedClusterRole.Namespace}}
	if err := fetchObject(ctx, r.Client, "", liveClusterRole.Name, liveClusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to reconcile the aggregated ClusterRole %s: %w", liveClusterRole.Name, err)
		}

		log.Info(fmt.Sprintf("Creating aggregated ClusterRole %s", expectedClusterRole.Name))
		expectedClusterRole.Rules = expectedPolicyRules
		return r.Client.Create(ctx, expectedClusterRole)
	}

	updateNeeded := false

	if !reflect.DeepEqual(liveClusterRole.Rules, expectedPolicyRules) {
		updateNeeded = true
		log.Info(fmt.Sprintf("PolicyRules of ClusterRole %s do not match the expected state, hence updating it", liveClusterRole.Name))
		liveClusterRole.Rules = expectedPolicyRules
	}

	normalizedLiveClusterRole := liveClusterRole.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveClusterRole.ObjectMeta, cr)
	if !reflect.DeepEqual(normalizedLiveClusterRole.Labels, expectedClusterRole.Labels) || !reflect.DeepEqual(normalizedLiveClusterRole.Annotations, expectedClusterRole.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of aggregated ClusterRole %s do not match the expected state, hence updating it", liveClusterRole.Name))

		liveClusterRole.Labels = combineStringMaps(liveClusterRole.Labels, expectedClusterRole.Labels)
		liveClusterRole.Annotations = combineStringMaps(liveClusterRole.Annotations, expectedClusterRole.Annotations)
	}

	if updateNeeded {
		// Update if the aggregated ClusterRole already exists and needs to be modified
		return r.Client.Update(ctx, liveClusterRole)
	}

	return nil
}

// reconcileRolloutsMetricsServiceAndMonitor reconciles the Rollouts Metrics Service and ServiceMonitor
func (r *RolloutManagerReconciler) reconcileRolloutsMetricsServiceAndMonitor(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) error {

	reconciledSvc, err := r.reconcileRolloutsMetricsService(ctx, cr)
	if err != nil {
		return fmt.Errorf("unable to reconcile metrics service: %w", err)
	}

	// Checks if user is using the Prometheus operator by checking CustomResourceDefinition for ServiceMonitor
	smCRD := &crdv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceMonitorsCRDName,
		},
	}

	if err := fetchObject(ctx, r.Client, smCRD.Namespace, smCRD.Name, smCRD); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the ServiceMonitor %s : %s", smCRD.Name, err)
		}
		return nil
	}

	// Create ServiceMonitor for Rollouts metrics
	existingServiceMonitor := &monitoringv1.ServiceMonitor{}
	if err := fetchObject(ctx, r.Client, cr.Namespace, DefaultArgoRolloutsResourceName, existingServiceMonitor); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.createServiceMonitorIfAbsent(ctx, cr.Namespace, cr, DefaultArgoRolloutsResourceName, reconciledSvc.Name); err != nil {
				return err
			}
			return nil

		} else {
			log.Error(err, "Error querying for ServiceMonitor", "Namespace", cr.Namespace, "Name", reconciledSvc.Name)
			return err
		}

	} else {
		log.Info("A ServiceMonitor instance already exists",
			"Namespace", existingServiceMonitor.Namespace, "Name", existingServiceMonitor.Name)

		// Check if existing ServiceMonitor matches expected content
		if !serviceMonitorMatches(existingServiceMonitor, reconciledSvc.Name) {
			log.Info("Updating existing ServiceMonitor instance",
				"Namespace", existingServiceMonitor.Namespace, "Name", existingServiceMonitor.Name)

			// Update ServiceMonitor with expected content
			existingServiceMonitor.Spec.Selector.MatchLabels = map[string]string{
				"app.kubernetes.io/name": reconciledSvc.Name,
			}
			existingServiceMonitor.Spec.Endpoints = []monitoringv1.Endpoint{
				{
					Port: "metrics",
				},
			}

			if err := r.Client.Update(ctx, existingServiceMonitor); err != nil {
				log.Error(err, "Error updating existing ServiceMonitor instance",
					"Namespace", existingServiceMonitor.Namespace, "Name", existingServiceMonitor.Name)
				return err
			}
		}
		return nil
	}

}

// reconcileRolloutsMetricsService reconciles the Service which is used to gather metrics from Rollouts install
func (r *RolloutManagerReconciler) reconcileRolloutsMetricsService(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) (*corev1.Service, error) {

	expectedSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsMetricsServiceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabelsAndAnnotationsToObject(&expectedSvc.ObjectMeta, cr)
	// overwrite the annotations for Rollouts Metrics Service
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

	liveService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: expectedSvc.Name, Namespace: expectedSvc.Namespace}}
	if err := fetchObject(ctx, r.Client, cr.Namespace, liveService.Name, liveService); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the Service %s: %w", expectedSvc.Name, err)
		}

		if err := controllerutil.SetControllerReference(&cr, expectedSvc, r.Scheme); err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Creating Service %s", expectedSvc.Name))
		if err := r.Client.Create(ctx, expectedSvc); err != nil {
			log.Error(err, "Error creating Service", "Name", expectedSvc.Name)
			return nil, err
		}
		liveService = expectedSvc

	}

	updateNeeded := false

	if !reflect.DeepEqual(liveService.Spec.Ports, expectedSvc.Spec.Ports) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Ports of metrics Service %s do not match the expected state, hence updating it", liveService.Name))
		liveService.Spec.Ports = expectedSvc.Spec.Ports
	}

	normalizedLiveService := liveService.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveService.ObjectMeta, cr)
	if !reflect.DeepEqual(normalizedLiveService.Labels, expectedSvc.Labels) || !reflect.DeepEqual(normalizedLiveService.Annotations, expectedSvc.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of metrics Service %s do not match the expected state, hence updating it", liveService.Name))

		liveService.Labels = combineStringMaps(liveService.Labels, expectedSvc.Labels)
		liveService.Annotations = combineStringMaps(liveService.Annotations, expectedSvc.Annotations)
	}

	if updateNeeded {
		// Update if the Service already exists and needs to be modified
		if err := r.Client.Update(ctx, liveService); err != nil {
			log.Error(err, "Error updating Ports of metrics Service", "Name", liveService.Name)
			return liveService, err
		}
	}

	return liveService, nil

}

// Reconciles Secrets for Rollouts controller
func (r *RolloutManagerReconciler) reconcileRolloutsSecrets(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) error {

	expectedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultRolloutsNotificationSecretName,
			Namespace: cr.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}

	setRolloutsLabelsAndAnnotationsToObject(&expectedSecret.ObjectMeta, cr)

	// If the Secret doesn't exist (or an unrelated error occurred)....
	liveSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: expectedSecret.Name, Namespace: expectedSecret.Namespace}}
	if err := fetchObject(ctx, r.Client, cr.Namespace, liveSecret.Name, liveSecret); err != nil {
		if !apierrors.IsNotFound(err) { // unrelated error: return
			return fmt.Errorf("failed to get the Secret %s: %w", liveSecret.Name, err)
		}

		if cr.Spec.SkipNotificationSecretDeployment {
			// Secret does not exist, but SkipNotificationSecretDeployment is set to true, hence skipping the creation
			return nil
		}

		// Secret does not exist (and SkipNotificationSecretDeployment is set to false) so create Secret
		if err := controllerutil.SetControllerReference(&cr, expectedSecret, r.Scheme); err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Creating Secret %s", expectedSecret.Name))
		return r.Client.Create(ctx, expectedSecret)

	}

	// If SkipNotificationSecretDeployment is true, and the secret exists (and is owned by us), delete it
	if cr.Spec.SkipNotificationSecretDeployment {

		// If the controller created/owns the Secret, delete it
		controller := metav1.GetControllerOf(liveSecret)
		if controller != nil && controller.Name == cr.Name {
			log.Info(fmt.Sprintf("SkipNotificationSecretDeployment has been set to true, deleting secret %s", liveSecret.Name))
			return r.Client.Delete(ctx, liveSecret)
		}

		// Otherwise, the secret exists, but the controller didn't create it, so just return (don't touch it)
		return nil
	}

	// Otherwise, the Secret exists, so update it if the labels/annotations are inconsistent

	updateNeeded := false

	normalizedLiveSecret := liveSecret.DeepCopy()
	removeUserLabelsAndAnnotations(&normalizedLiveSecret.ObjectMeta, cr)

	if !reflect.DeepEqual(normalizedLiveSecret.Labels, expectedSecret.Labels) || !reflect.DeepEqual(normalizedLiveSecret.Annotations, expectedSecret.Annotations) {
		updateNeeded = true
		log.Info(fmt.Sprintf("Labels/Annotations of Secret %s do not match the expected state, hence updating it", liveSecret.Name))

		liveSecret.Labels = combineStringMaps(liveSecret.Labels, expectedSecret.Labels)
		liveSecret.Annotations = combineStringMaps(liveSecret.Annotations, expectedSecret.Annotations)
	}

	if updateNeeded {
		// Update if the Secret already exists and needs to be modified
		return r.Client.Update(ctx, liveSecret)
	}

	// secret found, do nothing
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
				"update",
				"patch",
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
				"traefik.io",
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
		{
			APIGroups: []string{
				"route.openshift.io",
			},
			Resources: []string{
				"routes",
			},
			Verbs: []string{
				"create",
				"watch",
				"get",
				"update",
				"patch",
				"list",
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

func (r *RolloutManagerReconciler) createServiceMonitorIfAbsent(ctx context.Context, namespace string, rolloutManager rolloutsmanagerv1alpha1.RolloutManager, name, serviceMonitorLabel string) error {
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": serviceMonitorLabel,
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: "metrics",
				},
			},
		},
	}
	log.Info("Creating a new ServiceMonitor instance",
		"Namespace", serviceMonitor.Namespace, "Name", serviceMonitor.Name)

	// Set the RolloutManager instance as the owner and controller
	if err := controllerutil.SetControllerReference(&rolloutManager, serviceMonitor, r.Scheme); err != nil {
		log.Error(err, "Error setting read role owner ref",
			"Namespace", serviceMonitor.Namespace, "Name", serviceMonitor.Name, "RolloutManager Name", rolloutManager.Name)
		return err
	}

	err := r.Client.Create(ctx, serviceMonitor)
	if err != nil {
		log.Error(err, "Error creating a new ServiceMonitor instance",
			"Namespace", serviceMonitor.Namespace, "Name", serviceMonitor.Name)
		return err
	}

	return nil

}

func serviceMonitorMatches(sm *monitoringv1.ServiceMonitor, matchLabel string) bool {
	// Check if labels match
	labels := sm.Spec.Selector.MatchLabels
	if val, ok := labels["app.kubernetes.io/name"]; ok {
		if val != matchLabel {
			return false
		}
	} else {
		return false
	}

	// Check if endpoints match
	if len(sm.Spec.Endpoints) == 0 || sm.Spec.Endpoints[0].Port != "metrics" {
		return false
	}

	return true
}
