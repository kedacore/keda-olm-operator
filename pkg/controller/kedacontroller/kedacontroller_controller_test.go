package kedacontroller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/pkg/apis/keda/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mf "github.com/jcrossley3/manifestival"
)

var (
	name      			= "keda"
	namespace 			= "keda"

	moduleName			= "keda-olm-operator"

	logTimeFormatPrefix	= "--zap-time-encoding="
	logLevelPrefix		= "--zap-level="

	containerName		= "keda-operator"

	wd = basePath()

	kedacontroller = &kedav1alpha1.KedaController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
)

func basePath() string {
	wd, _ := os.Getwd()
	for !strings.HasSuffix(wd, moduleName) {
		wd = filepath.Dir(wd)
	}
	return wd
}

func setupScheme() *runtime.Scheme {
	s := scheme.Scheme
	s.AddKnownTypes(kedav1alpha1.SchemeGroupVersion, kedacontroller)
	return s
}

func setupReconcileKedaController(s *runtime.Scheme) (*ReconcileKedaController, error) {

	objs := []runtime.Object{kedacontroller}

	cl := fake.NewFakeClient(objs...)

	r := &ReconcileKedaController{client: cl, scheme: s}

	manifest, err := mf.NewManifest(fmt.Sprintf("%s,%s,%s", wd+"/"+resourceClusterRole, wd+"/"+resourceRoleBinding, wd+"/"+resourceOperator), false, cl)
	if err != nil {
		return nil, err
	}
	r.resourcesController = manifest

	reqLogger := log.WithValues("logger")

	if err := r.addFinalizer(reqLogger, kedacontroller); err != nil {
		return nil, err
	}

	return r, nil
}

func checkDeploymentArgs(dep appsv1.Deployment, expected string, prefix string, containerName string) error {
	for _, container := range dep.Spec.Template.Spec.Containers {
		if container.Name == containerName {

			for _, arg := range container.Args {
				if strings.HasPrefix(arg, prefix) {
					trimmedArg := strings.TrimPrefix(arg, prefix)
					if trimmedArg == expected {
						return nil
					}
					return errors.New("Wrong log time format, expected: " + expected + " got: " + trimmedArg)
				}
			}

		}
	}
	return errors.New("Could not find a container: " + containerName)
}

func TestReplaceKedaOperatorLogTimeFormat(t *testing.T) {

	tests := []struct {
		name					string
		initialLogTimeFormat	string
		actualLogTimeFormat		string
	}{
		{
			name:					"Change millis",
			initialLogTimeFormat:	"millis",
			actualLogTimeFormat:	"millis",
		},
		{
			name:					"Change nano",
			initialLogTimeFormat:	"nano",
			actualLogTimeFormat:	"nano",
		},
		{
			name:					"Change iso8601",
			initialLogTimeFormat:	"iso8601",
			actualLogTimeFormat:	"iso8601",
		},
		{
			name:					"Change epoch",
			initialLogTimeFormat:	"epoch",
			actualLogTimeFormat:	"epoch",
		},
		{
			name:					"Change empty",
			initialLogTimeFormat:	"",
			actualLogTimeFormat:	"epoch",
		},
		{
			name:					"Change wrong imput",
			initialLogTimeFormat:	"foo",
			actualLogTimeFormat:	"epoch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			kedacontroller.Spec = kedav1alpha1.KedaControllerSpec{
				LogTimeFormat: test.initialLogTimeFormat,
			}

			s := setupScheme()

			r, err := setupReconcileKedaController(s)
			if err != nil {
				t.Fatalf("Failed to set up reconciler: %v", err)
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Fatalf("Failed to reconcile: %v", err)
			}

			for _, res := range r.resourcesController.Resources {
				if res.GetKind() == "Deployment" {
					u := res.DeepCopy()

					dep := &appsv1.Deployment{}
					if err := s.Convert(u, dep, nil); err != nil {
						t.Fatalf("Failed to convert: %v", err)
					}

					err = checkDeploymentArgs(*dep, test.actualLogTimeFormat, logTimeFormatPrefix, containerName)
					if err != nil {
						t.Fatalf("%v", err)
					}

				}
			}

		})
	}
}

func TestReplaceKedaOperatorLogLevel(t *testing.T) {

	tests := []struct {
		name			string
		initialLogLevel	string
		actualLogLevel	string
	}{
		{
			name:				"Change debug",
			initialLogLevel:	"debug",
			actualLogLevel:		"debug",
		},
		{
			name:				"Change info",
			initialLogLevel:	"info",
			actualLogLevel:		"info",
		},
		{
			name:				"Change error",
			initialLogLevel:	"error",
			actualLogLevel:		"error",
		},
		{
			name:				"Change empty",
			initialLogLevel:	"",
			actualLogLevel:		"info",
		},
		{
			name:				"Change wrong imput",
			initialLogLevel:	"foo",
			actualLogLevel:		"info",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			kedacontroller.Spec = kedav1alpha1.KedaControllerSpec{
				LogLevel: test.initialLogLevel,
			}

			s := setupScheme()

			r, err := setupReconcileKedaController(s)
			if err != nil {
				t.Fatalf("Failed to set up reconciler: %v", err)
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
			}

			_, err = r.Reconcile(req)
			if err != nil {
				t.Fatalf("Failed to reconcile: %v", err)
			}

			for _, res := range r.resourcesController.Resources {
				if res.GetKind() == "Deployment" {
					u := res.DeepCopy()

					dep := &appsv1.Deployment{}
					if err := s.Convert(u, dep, nil); err != nil {
						t.Fatalf("Failed to convert: %v", err)
					}

					err = checkDeploymentArgs(*dep, test.actualLogLevel, logLevelPrefix, containerName)
					if err != nil {
						t.Fatalf("%v", err)
					}

				}
			}

		})
	}
}
