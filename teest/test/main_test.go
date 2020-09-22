package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/kedacore/keda-olm-operator/pkg/apis"
	kedav1alpha1 "github.com/kedacore/keda-olm-operator/pkg/apis/keda/v1alpha1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	name             = "keda"
	olmOperator      = "keda-olm-operator"
	operator         = "keda-operator"
	metricsAPIServer = "keda-metrics-apiserver"
	retryInterval    = time.Second * 5
	timeout          = time.Second * 30
)

func TestMain(m *testing.M) {
	framework.MainEntry(m)
}

func setupContext(t *testing.T) *framework.Context {
	kedacontrollerList := &kedav1alpha1.KedaControllerList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, kedacontrollerList)
	if err != nil {
		t.Fatalf("Failed to add custom resource scheme to framework: %v", err)
	}

	return framework.NewContext(t)
}

func TestKedaDeployment(t *testing.T) {

	ctx := setupContext(t)
	defer ctx.Cleanup()

	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx})
	if err != nil {
		t.Fatalf("Failed to initialize cluster resources: %v", err)
	}

	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		t.Fatal(err)
	}

	f := framework.Global

	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, olmOperator, 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	kedacontroller := &kedav1alpha1.KedaController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	err = f.Client.Create(context.TODO(), kedacontroller, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatal(err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, operator, 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, metricsAPIServer, 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

}
