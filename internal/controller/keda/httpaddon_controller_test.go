/*
Copyright 2025 The KEDA Authors

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
	"fmt"
	"time"

	mf "github.com/manifestival/manifestival"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
)

var _ = Describe("Deploying HTTP Add-on with KedaController", func() {
	const (
		httpAddonOperatorName    = "keda-http-add-on-operator"
		httpAddonScalerName      = "keda-http-add-on-external-scaler"
		httpAddonInterceptorName = "keda-http-add-on-interceptor"
		kedaManifestFilepath     = "../../../config/samples/keda_v1alpha1_kedacontroller.yaml"
		timeout                  = time.Second * 120
		interval                 = time.Millisecond * 500
		namespace                = "keda"
	)

	var (
		ctx      = context.Background()
		scheme   *runtime.Scheme
		manifest mf.Manifest
	)

	Context("When HTTP Add-on is enabled", func() {
		BeforeEach(func() {
			if testType != "functionality" {
				Skip("test.type isn't 'functionality'")
			}
			scheme = k8sManager.GetScheme()
			var err error
			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			// Disable HTTP Add-on and delete resources
			manifest, err := disableHTTPAddon(manifest, scheme)
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).Should(Succeed())

			// Wait for HTTP Add-on deployments to be deleted
			Eventually(func() error {
				_, err := getObject(ctx, "Deployment", httpAddonOperatorName, namespace, k8sClient)
				return err
			}, timeout, interval).ShouldNot(Succeed())
		})

		It("Should deploy HTTP Add-on components when enabled", func() {
			// Enable HTTP Add-on
			manifest, err := enableHTTPAddon(manifest, scheme)
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).Should(Succeed())

			// Verify HTTP Add-on operator deployment exists
			Eventually(func() error {
				_, err := getObject(ctx, "Deployment", httpAddonOperatorName, namespace, k8sClient)
				return err
			}, timeout, interval).Should(Succeed())

			// Verify HTTP Add-on scaler deployment exists
			Eventually(func() error {
				_, err := getObject(ctx, "Deployment", httpAddonScalerName, namespace, k8sClient)
				return err
			}, timeout, interval).Should(Succeed())

			// Verify HTTP Add-on interceptor deployment exists
			Eventually(func() error {
				_, err := getObject(ctx, "Deployment", httpAddonInterceptorName, namespace, k8sClient)
				return err
			}, timeout, interval).Should(Succeed())
		})

		It("Should not deploy HTTP Add-on when disabled", func() {
			// Apply manifest without HTTP Add-on enabled
			Expect(manifest.Apply()).Should(Succeed())

			// Verify KEDA is installed but HTTP Add-on is not
			Eventually(func() error {
				_, err := getObject(ctx, "Deployment", "keda-operator", namespace, k8sClient)
				return err
			}, timeout, interval).Should(Succeed())

			// HTTP Add-on operator should not exist
			Consistently(func() error {
				_, err := getObject(ctx, "Deployment", httpAddonOperatorName, namespace, k8sClient)
				return err
			}, time.Second*10, interval).ShouldNot(Succeed())
		})
	})

	Context("When configuring HTTP Add-on operator", func() {
		BeforeEach(func() {
			if testType != "functionality" {
				Skip("test.type isn't 'functionality'")
			}
			scheme = k8sManager.GetScheme()
			var err error
			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			manifest, err := disableHTTPAddon(manifest, scheme)
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).Should(Succeed())
		})

		It("Should respect custom operator replicas", func() {
			// Enable HTTP Add-on with custom operator replicas
			manifest, err := enableHTTPAddonWithOperatorReplicas(manifest, scheme, 2)
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).Should(Succeed())

			// Wait for deployment and verify replicas
			Eventually(func() int32 {
				deploy := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: httpAddonOperatorName, Namespace: namespace}, deploy)
				if err != nil {
					return 0
				}
				if deploy.Spec.Replicas != nil {
					return *deploy.Spec.Replicas
				}
				return 0
			}, timeout, interval).Should(Equal(int32(2)))
		})

		It("Should respect custom scaler replicas", func() {
			// Enable HTTP Add-on with custom scaler replicas
			manifest, err := enableHTTPAddonWithScalerReplicas(manifest, scheme, 5)
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).Should(Succeed())

			// Wait for deployment and verify replicas
			Eventually(func() int32 {
				deploy := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: httpAddonScalerName, Namespace: namespace}, deploy)
				if err != nil {
					return 0
				}
				if deploy.Spec.Replicas != nil {
					return *deploy.Spec.Replicas
				}
				return 0
			}, timeout, interval).Should(Equal(int32(5)))
		})
	})
})

func enableHTTPAddon(manifest mf.Manifest, scheme *runtime.Scheme) (mf.Manifest, error) {
	transformer := func(u *unstructured.Unstructured) error {
		kedaControllerInstance := &kedav1alpha1.KedaController{}
		if err := scheme.Convert(u, kedaControllerInstance, nil); err != nil {
			return err
		}
		kedaControllerInstance.Spec.HTTPAddon.Enabled = true
		return scheme.Convert(kedaControllerInstance, u, nil)
	}
	return manifest.Transform(transformer)
}

func disableHTTPAddon(manifest mf.Manifest, scheme *runtime.Scheme) (mf.Manifest, error) {
	transformer := func(u *unstructured.Unstructured) error {
		kedaControllerInstance := &kedav1alpha1.KedaController{}
		if err := scheme.Convert(u, kedaControllerInstance, nil); err != nil {
			return err
		}
		kedaControllerInstance.Spec.HTTPAddon.Enabled = false
		return scheme.Convert(kedaControllerInstance, u, nil)
	}
	return manifest.Transform(transformer)
}

func enableHTTPAddonWithOperatorReplicas(manifest mf.Manifest, scheme *runtime.Scheme, replicas int32) (mf.Manifest, error) {
	transformer := func(u *unstructured.Unstructured) error {
		kedaControllerInstance := &kedav1alpha1.KedaController{}
		if err := scheme.Convert(u, kedaControllerInstance, nil); err != nil {
			return err
		}
		kedaControllerInstance.Spec.HTTPAddon.Enabled = true
		kedaControllerInstance.Spec.HTTPAddon.Operator.Replicas = &replicas
		// Add annotation to track test changes
		if kedaControllerInstance.Spec.HTTPAddon.Operator.PodAnnotations == nil {
			kedaControllerInstance.Spec.HTTPAddon.Operator.PodAnnotations = make(map[string]string)
		}
		kedaControllerInstance.Spec.HTTPAddon.Operator.PodAnnotations["testCase"] = fmt.Sprintf("operator-replicas-%d", replicas)
		return scheme.Convert(kedaControllerInstance, u, nil)
	}
	return manifest.Transform(transformer)
}

func enableHTTPAddonWithScalerReplicas(manifest mf.Manifest, scheme *runtime.Scheme, replicas int32) (mf.Manifest, error) {
	transformer := func(u *unstructured.Unstructured) error {
		kedaControllerInstance := &kedav1alpha1.KedaController{}
		if err := scheme.Convert(u, kedaControllerInstance, nil); err != nil {
			return err
		}
		kedaControllerInstance.Spec.HTTPAddon.Enabled = true
		kedaControllerInstance.Spec.HTTPAddon.Scaler.Replicas = &replicas
		// Add annotation to track test changes
		if kedaControllerInstance.Spec.HTTPAddon.Scaler.PodAnnotations == nil {
			kedaControllerInstance.Spec.HTTPAddon.Scaler.PodAnnotations = make(map[string]string)
		}
		kedaControllerInstance.Spec.HTTPAddon.Scaler.PodAnnotations["testCase"] = fmt.Sprintf("scaler-replicas-%d", replicas)
		return scheme.Convert(kedaControllerInstance, u, nil)
	}
	return manifest.Transform(transformer)
}
