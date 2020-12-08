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
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/v1alpha1"
	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	olmOperatorPrefix     = "keda-olm-operator"
	namespace             = "keda"
	namespaceManifest     = "../config/testing/namespace.yaml"
	catalogManifest       = "../config/testing/catalog.yaml"
	operatorGroupManifest = "../config/testing/operator_group.yaml"
	subscriptionManifest  = "../config/testing/subscription.yaml"
)

var (
	ctx                      = context.Background()
	cfg                      *rest.Config
	k8sClient                client.Client
	testEnv                  *envtest.Environment
	k8sManager               ctrl.Manager
	kedaControllerReconciler *KedaControllerReconciler
	manifest                 mf.Manifest
	err                      error
	timeout                  = time.Second * 150
	interval                 = time.Millisecond * 250
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
	useExistingCluster := true
	testEnv = &envtest.Environment{UseExistingCluster: &useExistingCluster}

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

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	Expect(deployManifest(namespaceManifest, k8sClient)).Should(Succeed())
	Expect(deployManifest(catalogManifest, k8sClient)).Should(Succeed())
	Expect(deployManifest(operatorGroupManifest, k8sClient)).Should(Succeed())
	Expect(deployManifest(subscriptionManifest, k8sClient)).Should(Succeed())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	Eventually(func() error {
		_, err = getPod(olmOperatorPrefix, namespace, k8sClient, ctx)
		return err
	}, timeout, interval).Should(Succeed())

	close(done)
}, 300)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func getPod(namePrefix string, namespace string, c client.Client, ctx context.Context) (pod *corev1.Pod, err error) {
	pod = &corev1.Pod{}
	podList := &corev1.PodList{}
	lo := &client.ListOptions{Namespace: namespace}
	err = c.List(ctx, podList, lo)
	if err != nil {
		return
	}
	found := false
	for _, p := range podList.Items {
		podName := p.ObjectMeta.Name
		if strings.HasPrefix(podName, namePrefix) {
			found = true
			err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: podName}, pod)
			if err != nil {
				return
			}
			break
		}
	}
	if !found {
		err = errors.New("Pod with name prefix: " + namePrefix + " was not found in namespace: " + namespace)
	}
	return
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
