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

	"github.com/kedacore/keda-olm-operator/controllers/transform"
	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Keda OLM operator", func() {
	const (
		olmOperatorName      = "keda-olm-operator"
		operatorName         = "keda-operator"
		kedaManifestFilepath = "../config/samples/keda_v1alpha1_kedacontroller.yaml"
		timeout              = time.Second * 60
		interval             = time.Millisecond * 250
	)

	var (
		ctx      = context.Background()
		err      error
		scheme   *runtime.Scheme
		manifest mf.Manifest
	)

	Describe("Deploying KedaController manifest", func() {
		BeforeEach(func() {
			scheme = k8sManager.GetScheme()
			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			manifest, err = changeNamespace(manifest, namespace, scheme)
			Expect(err).To(BeNil())

			Expect(manifest.Delete()).Should(Succeed())

			Eventually(func() error {
				_, err = getPod(operatorName, namespace, k8sClient, ctx)
				return err
			}, timeout, interval).ShouldNot(Succeed())
		})

		Context("When deploying in \"keda\" namespace", func() {
			It("Should deploy KedaController", func() {

				Expect(manifest.Apply()).Should(Succeed())

				Eventually(func() error {
					_, err = getPod(operatorName, namespace, k8sClient, ctx)
					return err
				}, timeout, interval).Should(Succeed())
			})
		})

		Context("When deploying not in \"keda\" namespace", func() {
			const changedNamespace = "default"

			It("Should not deploy KedaController", func() {

				manifest, err = changeNamespace(manifest, changedNamespace, scheme)
				Expect(err).To(BeNil())

				Expect(manifest.Apply()).Should(Succeed())

				Eventually(func() error {
					_, err = getPod(operatorName, namespace, k8sClient, ctx)
					return err
				}, timeout, interval).ShouldNot(Succeed())
			})
		})
	})

	Describe("Changing parameters", func() {
		const (
			containerName   = "keda-operator"
			logLevelPrefix  = "--zap-log-level="
			defaultLogLevel = "info"
		)

		var (
			arg string
			pod = &corev1.Pod{}
			log = ctrl.Log.WithName("test")
		)

		BeforeEach(func() {
			scheme = k8sManager.GetScheme()
			Expect(deployManifest(kedaManifestFilepath, k8sClient)).Should(Succeed())
			Eventually(func() error {
				_, err = getPod(operatorName, namespace, k8sClient, ctx)
				return err
			}, timeout, interval).Should(Succeed())

			pod, err = getPod(operatorName, namespace, k8sClient, ctx)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			Expect(manifest.Delete()).Should(Succeed())

			Eventually(func() error {
				_, err = getPod(operatorName, namespace, k8sClient, ctx)
				return err
			}, timeout, interval).ShouldNot(Succeed())
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

					transforms := []mf.Transformer{transform.ReplaceKedaOperatorLogLevel(variant.initialLogLevel, scheme, log)}
					manifest, err = manifest.Transform(transforms...)
					Expect(err).To(BeNil())
					manifest.Apply()

					arg, err = getPodArg(pod, logLevelPrefix, containerName)
					Expect(err).To(BeNil())
					Expect(arg).To(Equal(variant.actualLogLevel))
				})
			}
		})
	})
})

func getPodArg(pod *corev1.Pod, prefix string, containerName string) (string, error) {
	for _, container := range pod.Spec.Containers {
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
