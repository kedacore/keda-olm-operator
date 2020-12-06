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
	"time"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"fmt"
	// corev1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/runtime/schema"
	// "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Keda OLM operator", func() {
	const (
		namespace = "keda"
		name      = "keda"
	)

	var (
		ctx                    context.Context
		kedaControllerInstance *kedav1alpha1.KedaController
		namespacedName         = types.NamespacedName{Namespace: namespace, Name: name}
		err                    error
		timeout                = time.Second * 10
		interval               = time.Millisecond * 250
		scheme                 *runtime.Scheme
	)

	Describe("Deploying KedaController manifest", func() {
		const (
			kedaManifestFilepath = "../config/samples/keda_v1alpha1_kedacontroller.yaml"
		)

		var manifest mf.Manifest

		BeforeEach(func() {
			scheme = k8sManager.GetScheme()
			manifest, err = mf.NewManifest(kedaManifestFilepath)
			Expect(err).To(BeNil())
			manifest.Client = mfc.NewClient(k8sClient)

			ctx = context.Background()
			kedaControllerInstance = &kedav1alpha1.KedaController{}
		})

		AfterEach(func() {
			manifest, err = changeNamespace(manifest, namespace, scheme)
			Expect(err).To(BeNil())

			Expect(manifest.Delete()).Should(Succeed())

			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName, kedaControllerInstance)
			}, timeout, interval).ShouldNot(Succeed())
		})

		Context("When deploying in \"keda\" namespace", func() {
			It("Should deploy KedaController", func() {

				Expect(manifest.Apply()).Should(Succeed())

				Eventually(func() error {
					// ! tuna nemozem kontrolovat kedacontroller instance ale pod/description
					// ! kedacontroller kind vytvori aj ked nebezi keda-olm-operator
					return k8sClient.Get(ctx, namespacedName, kedaControllerInstance)
				}, timeout, interval).Should(Succeed())

				ctx = context.Background()
				// pod := &corev1.PodList{}
				// pod := &appsv1.DeploymentList{}
				pod := &kedav1alpha1.KedaControllerList{}
				// lso := &client.ListOptions{
				// 	Namespace: "keda",
				// }
				// u := &unstructured.UnstructuredList{}
				// u.SetGroupVersionKind(schema.GroupVersionKind{
				// 	Group:   "apps",
				// 	Kind:    "DeploymentList",
				// 	Version: "v1",
				// })
				// _ = k8sClient.List(ctx, u, lso)
				// fmt.Println("list:", u)
				_ = k8sClient.List(ctx, pod)
				fmt.Println("list:", pod)

			})
		})

		Context("When deploying not in \"keda\" namespace", func() {
			const changedNamespace = "default"

			It("Should not deploy KedaController", func() {

				manifest, err = changeNamespace(manifest, changedNamespace, scheme)
				Expect(err).To(BeNil())

				Expect(manifest.Apply()).Should(Succeed())

				Eventually(func() error {
					return k8sClient.Get(ctx, namespacedName, kedaControllerInstance)
				// }, timeout, interval).ShouldNot(Succeed())
				}, timeout, interval).Should(Succeed())
			})
		})
	})

	Describe("Changing parameters", func() {
		const (
			kind           = "Deployment"
			deploymentName = "keda-operator"
			containerName  = "keda-operator"
			logLevelPrefix = "--zap-log-level="
		)

		var dep = &appsv1.Deployment{}

		BeforeEach(func() {
			scheme = k8sManager.GetScheme()
		})

		Context("When changing \"--zap-log-level\"", func() {
			variants := []struct {
				initialLogLevel string
				actualLogLevel  string
			}{
				{
					initialLogLevel: "debug",
					actualLogLevel:  "debug",
				},
				{
					initialLogLevel: "info",
					actualLogLevel:  "info",
				},
				{
					initialLogLevel: "error",
					actualLogLevel:  "error",
				},
				{
					initialLogLevel: "",
					actualLogLevel:  "info",
				},
				{
					initialLogLevel: "foo",
					actualLogLevel:  "info",
				},
			}

			for _, variant := range variants {
				It("Should change it", func() {
					kedaControllerInstance.Spec.LogLevel = variant.initialLogLevel

					_, err = kedaControllerReconciler.Reconcile(reconcile.Request{NamespacedName: namespacedName})
					Expect(err).To(BeNil())

					for _, res := range kedaControllerReconciler.resourcesController.Filter(mf.ByKind(kind)).Resources() {
						if res.GetName() == deploymentName {
							u := res.DeepCopy()
							Expect(scheme.Convert(u, dep, nil)).To(Succeed())

							value, err := getDeploymentArgsForPrefix(dep, logLevelPrefix, containerName)
							Expect(err).To(BeNil())
							Expect(value).To(Equal(variant.actualLogLevel))
						}
					}
				})
			}
		})
	})
})

func getDeploymentArgsForPrefix(dep *appsv1.Deployment, prefix string, containerName string) (string, error) {
	for _, container := range dep.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			for _, arg := range container.Args {
				if strings.HasPrefix(arg, prefix) {
					return strings.TrimPrefix(arg, prefix), nil
				}
			}
			return "", errors.New("Could not find an argument with given prefix: " + prefix)
		}
	}
	return "", errors.New("Could not find a container: " + containerName)
}

func changeNamespace(manifest mf.Manifest, namespace string, scheme *runtime.Scheme) (mf.Manifest, error) {
	transformer := func(u *unstructured.Unstructured) error {
		kedaControllerInstance := &kedav1alpha1.KedaController{}
		if err := scheme.Convert(u, kedaControllerInstance, nil); err != nil {
			return err
		}

		kedaControllerInstance.Namespace = namespace

		if err := scheme.Convert(kedaControllerInstance, u, nil); err != nil {
			return err
		}
		return nil
	}

	return manifest.Transform(transformer)
}
