//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
)

const (
	namespace = "keda"

	pollInterval = 2 * time.Second
	pollTimeout  = 3 * time.Minute
)

var (
	coreDeployments = []string{
		"keda-operator",
		"keda-metrics-apiserver",
		"keda-admission",
	}

	httpAddonDeployments = []string{
		"keda-add-ons-http-operator",
		"keda-add-ons-http-interceptor",
		"keda-add-ons-http-scaler",
	}
)

func TestKedaControllerLifecycle(t *testing.T) {
	ctx := t.Context()
	c := newClient(t)

	t.Cleanup(func() {
		cleanupCtx := context.Background()

		kc := &kedav1alpha1.KedaController{}
		if err := c.Get(cleanupCtx, client.ObjectKey{Name: "keda", Namespace: namespace}, kc); err != nil {
			return
		}
		if err := c.Delete(cleanupCtx, kc); err != nil {
			t.Logf("cleanup: failed to delete KedaController: %v", err)
			return
		}

		waitForDeploymentGone(t, cleanupCtx, c, coreDeployments[0])
	})

	step(t, "create KedaController", func(t *testing.T) {
		kc := &kedav1alpha1.KedaController{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "keda",
				Namespace: namespace,
			},
		}
		if err := c.Create(ctx, kc); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				t.Fatalf("creating KedaController: %v", err)
			}
		}
	})

	step(t, "core KEDA deployments become ready", func(t *testing.T) {
		for _, name := range coreDeployments {
			waitForDeploymentReady(t, ctx, c, name)
		}
	})

	step(t, "KedaController status shows success", func(t *testing.T) {
		var kc kedav1alpha1.KedaController
		err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
			if err := c.Get(ctx, client.ObjectKey{Name: "keda", Namespace: namespace}, &kc); err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, fmt.Errorf("unexpected error checking KedaController status: %w", err)
			}
			return kc.Status.Phase == kedav1alpha1.PhaseInstallSucceeded, nil
		})
		if err != nil {
			t.Fatalf("KedaController status did not reach %q: %v", kedav1alpha1.PhaseInstallSucceeded, err)
		}

		for _, name := range coreDeployments {
			logDeploymentImage(t, ctx, c, name)
		}
	})

	step(t, "enabling HTTP add-on deploys add-on components", func(t *testing.T) {
		kc := &kedav1alpha1.KedaController{}
		if err := c.Get(ctx, client.ObjectKey{Name: "keda", Namespace: namespace}, kc); err != nil {
			t.Fatalf("getting KedaController: %v", err)
		}
		kc.Spec.HTTPAddon.Enabled = true
		if err := c.Update(ctx, kc); err != nil {
			t.Fatalf("enabling HTTP add-on: %v", err)
		}

		for _, name := range httpAddonDeployments {
			waitForDeploymentReady(t, ctx, c, name)
		}

		var updated kedav1alpha1.KedaController
		err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
			if err := c.Get(ctx, client.ObjectKey{Name: "keda", Namespace: namespace}, &updated); err != nil {
				return false, fmt.Errorf("getting KedaController for HTTP add-on status: %w", err)
			}
			return updated.Status.HTTPAddon != nil && updated.Status.HTTPAddon.Phase == kedav1alpha1.PhaseInstallSucceeded, nil
		})
		if err != nil {
			t.Fatalf("HTTP add-on status did not reach %q: %v", kedav1alpha1.PhaseInstallSucceeded, err)
		}

		for _, name := range httpAddonDeployments {
			logDeploymentImage(t, ctx, c, name)
		}
	})

	step(t, "disabling HTTP add-on removes add-on components", func(t *testing.T) {
		kc := &kedav1alpha1.KedaController{}
		if err := c.Get(ctx, client.ObjectKey{Name: "keda", Namespace: namespace}, kc); err != nil {
			t.Fatalf("getting KedaController: %v", err)
		}
		kc.Spec.HTTPAddon.Enabled = false
		if err := c.Update(ctx, kc); err != nil {
			t.Fatalf("disabling HTTP add-on: %v", err)
		}

		for _, name := range httpAddonDeployments {
			waitForDeploymentGone(t, ctx, c, name)
		}
	})

	step(t, "deleting KedaController cleans up resources", func(t *testing.T) {
		kc := &kedav1alpha1.KedaController{}
		if err := c.Get(ctx, client.ObjectKey{Name: "keda", Namespace: namespace}, kc); err != nil {
			t.Fatalf("getting KedaController for deletion: %v", err)
		}
		if err := c.Delete(ctx, kc); err != nil {
			t.Fatalf("deleting KedaController: %v", err)
		}

		for _, name := range coreDeployments {
			waitForDeploymentGone(t, ctx, c, name)
		}
	})
}

func step(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.FailNow()
	}
}

func newClient(t *testing.T) client.Client {
	t.Helper()

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
	if err != nil {
		t.Fatalf("loading kubeconfig: %v", err)
	}

	if err := kedav1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatalf("registering KedaController scheme: %v", err)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	return c
}

func waitForDeploymentReady(t *testing.T, ctx context.Context, c client.Client, name string) {
	t.Helper()
	t.Logf("waiting for deployment %s/%s to become ready", namespace, name)

	err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
		dep := &appsv1.Deployment{}
		if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dep); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("unexpected error checking deployment %s: %w", name, err)
		}
		for _, cond := range dep.Status.Conditions {
			if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("deployment %s/%s did not become ready: %v", namespace, name, err)
	}
}

func waitForDeploymentGone(t *testing.T, ctx context.Context, c client.Client, name string) {
	t.Helper()
	t.Logf("waiting for deployment %s/%s to be deleted", namespace, name)

	err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
		dep := &appsv1.Deployment{}
		if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dep); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("unexpected error checking deployment %s: %w", name, err)
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("deployment %s/%s was not deleted: %v", namespace, name, err)
	}
}

func logDeploymentImage(t *testing.T, ctx context.Context, c client.Client, name string) {
	t.Helper()

	dep := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dep); err != nil {
		t.Fatalf("could not get deployment %s for image info: %v", name, err)
	}
	for _, container := range dep.Spec.Template.Spec.Containers {
		t.Logf("%s: %s", name, container.Image)
	}
}
