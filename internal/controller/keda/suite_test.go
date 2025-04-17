/*
Copyright 2020 The KEDA Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package keda

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	namespaceManifest     = "../../../config/testing/namespace.yaml"
	catalogManifest       = "../../../config/testing/catalog.yaml"
	operatorGroupManifest = "../../../config/testing/operator_group.yaml"
	subscriptionManifest  = "../../../config/testing/subscription.yaml"
)

var (
	cfg                      *rest.Config
	k8sClient                client.Client
	testEnv                  *envtest.Environment
	k8sManager               ctrl.Manager
	kedaControllerReconciler *KedaControllerReconciler
	err                      error
	testType                 string

	ctx    context.Context
	cancel context.CancelFunc
)

func init() {
	flag.StringVar(&testType, "test.type", "", "type of test: functionality / deployment")
}

func TestAPIs(t *testing.T) {
	// skip unit tests (or anything besides functionality or deployment)
	if testType != "functionality" && testType != "deployment" {
		return
	}
	RegisterFailHandler(Fail)

	RunSpecs(t,
		"Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")

	if testType == "functionality" {
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		}

		k8sManager, k8sClient, err = setupEnv(testEnv, scheme.Scheme)
		Expect(err).ToNot(HaveOccurred())

		Expect(deployManifest(namespaceManifest, k8sClient)).Should(Succeed())

		kedaControllerReconciler = &KedaControllerReconciler{
			Client: k8sClient,
			Log:    ctrl.Log.WithName("test").WithName("KedaController"),
			Scheme: k8sManager.GetScheme(),
		}
		err = (kedaControllerReconciler).SetupWithManager(k8sManager, "keda", kedaControllerReconciler.Log)
		Expect(err).ToNot(HaveOccurred())

	} else {
		useExistingCluster := true
		testEnv = &envtest.Environment{UseExistingCluster: &useExistingCluster}

		k8sManager, k8sClient, err = setupEnv(testEnv, scheme.Scheme)
		Expect(err).ToNot(HaveOccurred())

		Expect(deployManifest(namespaceManifest, k8sClient)).Should(Succeed())

		Expect(deployManifest(catalogManifest, k8sClient)).Should(Succeed())
		Expect(deployManifest(operatorGroupManifest, k8sClient)).Should(Succeed())
		Expect(deployManifest(subscriptionManifest, k8sClient)).Should(Succeed())
	}

	ctx, cancel = context.WithCancel(context.Background())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

}, 300)

var _ = AfterSuite(func() {
	By("tearing down the test environment")

	// stop k8sManager
	cancel()

	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func setupEnv(testEnv *envtest.Environment, scheme *runtime.Scheme) (manager ctrl.Manager, client client.Client, err error) {
	cfg, err = testEnv.Start()
	if err != nil {
		return
	}

	err = kedav1alpha1.AddToScheme(scheme)
	if err != nil {
		return
	}

	// +kubebuilder:scaffold:scheme
	manager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		return
	}

	client = manager.GetClient()
	if err != nil {
		return
	}

	return
}

func getObject(ctx context.Context, o Obj, namePrefix string, namespace string, c client.Client) (u *unstructured.Unstructured, err error) {
	u = &unstructured.Unstructured{}
	group, kind, version, err := o.getObjectGroupKindVersion()
	if err != nil {
		return
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    kind,
		Version: version,
	})
	uList := &unstructured.UnstructuredList{}
	group, kind, version, err = o.getListGroupKindVersion()
	if err != nil {
		return
	}
	uList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    kind,
		Version: version,
	})
	lo := &client.ListOptions{Namespace: namespace}
	err = c.List(ctx, uList, lo)
	if err != nil {
		return
	}
	found := false
	for _, p := range uList.Items {
		uName := p.GetName()
		if strings.HasPrefix(uName, namePrefix) {
			found = true
			err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: uName}, u)
			if err != nil {
				return
			}
			break
		}
	}
	if !found {
		err = errors.New("Object with name prefix: " + namePrefix + " was not found in namespace: " + namespace)
	}
	return
}

type Obj string

const (
	Pod        = "Pod"
	Deployment = "Deployment"
)

func (o Obj) getListGroupKindVersion() (group, kind, version string, err error) {
	switch o {
	case Pod:
		return "", "PodList", "v1", nil
	case Deployment:
		return "apps", "DeploymentList", "v1", nil
	default:
		return "", "", "", errors.New("Not a valid object")
	}
}

func (o Obj) getObjectGroupKindVersion() (group, kind, version string, err error) {
	switch o {
	case Pod:
		return "", "Pod", "v1", nil
	case Deployment:
		return "apps", "Deployment", "v1", nil
	default:
		return "", "", "", errors.New("Not a valid object")
	}
}

func deployManifest(pathname string, c client.Client) error {
	manifest, err := createManifest(pathname, c)
	if err != nil {
		return err
	}
	return manifest.Apply()
}

func createManifest(pathname string, c client.Client) (manifest mf.Manifest, err error) {
	manifest, err = mf.NewManifest(pathname)
	if err != nil {
		return
	}
	manifest.Client = mfc.NewClient(c)
	return
}
