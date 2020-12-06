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

package controllers

import (
	// "context"
	"path/filepath"
	"testing"
	// "time"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	mf "github.com/manifestival/manifestival"
	mfc "github.com/manifestival/controller-runtime-client"
	// "k8s.io/apimachinery/pkg/types"
	// appsv1 "k8s.io/api/apps/v1"
	// corev1 "k8s.io/api/core/v1"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
	"fmt"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg                      *rest.Config
	k8sClient                client.Client
	testEnv                  *envtest.Environment
	k8sManager               ctrl.Manager
	kedaControllerReconciler *KedaControllerReconciler
	manifest mf.Manifest
	err error
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = kedav1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme
	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	kedaControllerReconciler = &KedaControllerReconciler{
		Client: k8sManager.GetClient(),
		Log:    ctrl.Log.WithName("controllers-test").WithName("KedaController"),
		Scheme: k8sManager.GetScheme(),
	}
	err = (kedaControllerReconciler).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())


	// Expect(deployManifest("../config/testing/crds.yaml", k8sManager)).Should(Succeed())
	// time.Sleep(3 * time.Second)
	// Expect(deployManifest("../config/testing/olm.yaml", k8sManager)).Should(Succeed())
	// time.Sleep(3 * time.Second)
	// Expect(deployManifest("../config/testing/namespace.yaml", k8sManager)).Should(Succeed())
	// // Expect(deployManifest("../config/testing/catalog.yaml", k8sManager)).Should(Succeed())
	// Expect(deployManifest("../config/testing/operator_group.yaml", k8sManager)).Should(Succeed())
	// Expect(deployManifest("../config/testing/subscription.yaml", k8sManager)).Should(Succeed())

	// ctx := context.Background()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	// kedaControllerInstance := &kedav1alpha1.KedaController{}
	// kedaControllerInstance := &appsv1.Deployment{}
	// kedaControllerInstance := &corev1.Pod{}
	// namespacedName := types.NamespacedName{Namespace: "keda", Name: "keda-olm-operator"}
	// timeout := time.Second * 60
	// interval := time.Millisecond * 250
	fmt.Println("tu to este bolo")
	// Eventually(func() error {
	// 	return k8sClient.Get(ctx, namespacedName, kedaControllerInstance)
	// }, timeout, interval).Should(Succeed())
	// fmt.Println("tu uz to asi neni")

	// lso := &client.ListOptions{
	// 	Namespace: "keda",
	// }

	// pod := &corev1.PodList{}
	// time.Sleep(5 * time.Second)
	// _ = k8sClient.List(ctx, pod, lso)
	// _ = k8sClient.List(ctx, pod)
	// fmt.Println("list:", pod)

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func deployManifest(pathname string, manager ctrl.Manager) error {
	manifest, err := mf.NewManifest(pathname)
	if err != nil {
		return err
	}
	manifest.Client = mfc.NewClient(manager.GetClient())
	return manifest.Apply()
}