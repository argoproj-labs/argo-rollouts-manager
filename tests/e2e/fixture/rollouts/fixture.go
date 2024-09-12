package rollouts

import (
	"context"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

var rolloutGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "rollouts",
}

func buildGenericRolloutResource(name, namespace, activeService, previewService string) string {

	rolloutStr := `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: ` + name + `
  namespace: ` + namespace + `
spec:
  replicas: 2
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: test-argo-app
  strategy:
    blueGreen:
      activeService: ` + activeService + `
      autoPromotionEnabled: false
      previewService: ` + previewService + `

  template:
    metadata:
      labels:
        app: test-argo-app
    spec:
      containers:
      - image: nginxinc/nginx-unprivileged@sha256:0569e319d06556564ad40882ed35231461d06bec788b5aec00b83b6e9f3ced1a
        # nginx v1.27, from https://hub.docker.com/layers/nginxinc/nginx-unprivileged/1.27/images/sha256-37404ae33c744fe1c1e1f752225f986ff32c0327240ab6e960573c0f6cb577c3?context=explore
        name: webserver-simple
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        resources: {}`

	return rolloutStr
}

func CreateArgoRollout(ctx context.Context, name, namespace, activeService, previewService string) (string, error) {

	dynclient, err := fixture.GetDynamicClient()
	if err != nil {
		return "", err
	}

	rolloutStr := buildGenericRolloutResource(name, namespace, activeService, previewService)

	var un unstructured.Unstructured
	if err := yaml.UnmarshalStrict([]byte(rolloutStr), &un, yaml.DisallowUnknownFields); err != nil {
		return "", err
	}

	if _, err := dynclient.Resource(rolloutGVR).Namespace(namespace).Create(ctx, &un, metav1.CreateOptions{}); err != nil {
		return "", err
	}

	return rolloutStr, nil
}

func GetArgoRollout(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {

	dynclient, err := fixture.GetDynamicClient()
	if err != nil {
		return nil, err
	}

	return dynclient.Resource(rolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})

}

func DeleteArgoRollout(ctx context.Context, name, namespace string) error {

	dynclient, err := fixture.GetDynamicClient()
	if err != nil {
		return err
	}

	return dynclient.Resource(rolloutGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func HasEmptyStatus(ctx context.Context, name, namespace string) (bool, error) {

	rollout, err := GetArgoRollout(ctx, name, namespace)
	if err != nil {
		return false, err
	}

	mapVal := rollout.UnstructuredContent()

	if mapVal["status"] == nil {
		return true, nil
	}

	statusMapVal := (mapVal["status"]).(map[string]interface{})

	return len(statusMapVal) == 0, nil
}

func HasStatusPhase(ctx context.Context, name, namespace string, expectedPhase string) (bool, error) {

	rollout, err := GetArgoRollout(ctx, name, namespace)
	if err != nil {
		return false, err
	}

	mapVal := rollout.UnstructuredContent()

	if mapVal["status"] == nil {
		return false, nil
	}

	statusMapVal := (mapVal["status"]).(map[string]interface{})

	if statusMapVal["phase"] == nil {
		return false, nil
	}

	return (statusMapVal["phase"]).(string) == expectedPhase, nil

}
