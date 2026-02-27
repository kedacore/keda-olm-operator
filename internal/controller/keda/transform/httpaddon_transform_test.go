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

package transform_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kedacore/keda-olm-operator/internal/controller/keda/transform"
)

var _ = Describe("HTTP Add-on Transform Functions", func() {
	var (
		scheme *runtime.Scheme
		logger = zap.New(zap.UseDevMode(true))
	)

	BeforeEach(func() {
		if testType != "unit" {
			Skip("test.type isn't 'unit'")
		}
		scheme = runtime.NewScheme()
		_ = appsv1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)
	})

	Describe("ReplaceHTTPAddonOperatorImage", func() {
		It("should replace the operator image", func() {
			deploy := createHTTPAddonDeployment("keda-http-add-on-operator", "keda-http-add-on-operator", "ghcr.io/kedacore/http-add-on-operator:0.11.0")
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			transformer := transform.ReplaceHTTPAddonOperatorImage("my-registry/my-operator:v1.0.0", scheme)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())
			Expect(resultDeploy.Spec.Template.Spec.Containers[0].Image).To(Equal("my-registry/my-operator:v1.0.0"))
		})

		It("should not modify other deployments", func() {
			deploy := createHTTPAddonDeployment("other-deployment", "other-container", "original-image:v1")
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			transformer := transform.ReplaceHTTPAddonOperatorImage("my-registry/my-operator:v1.0.0", scheme)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())
			Expect(resultDeploy.Spec.Template.Spec.Containers[0].Image).To(Equal("original-image:v1"))
		})
	})

	Describe("ReplaceHTTPAddonOperatorReplicas", func() {
		It("should replace the operator replicas", func() {
			deploy := createHTTPAddonDeployment("keda-http-add-on-operator", "keda-http-add-on-operator", "image:v1")
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			var newReplicas int32 = 3
			transformer := transform.ReplaceHTTPAddonOperatorReplicas(newReplicas, scheme)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())
			Expect(*resultDeploy.Spec.Replicas).To(Equal(newReplicas))
		})
	})

	Describe("ReplaceHTTPAddonOperatorLogLevel", func() {
		It("should replace the operator log level", func() {
			deploy := createHTTPAddonDeploymentWithArgs("keda-http-add-on-operator", "keda-http-add-on-operator", []string{"--zap-log-level=info"})
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			transformer := transform.ReplaceHTTPAddonOperatorLogLevel("debug", scheme, logger)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())
			Expect(resultDeploy.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--zap-log-level=debug"))
		})

		It("should add the log level arg if not present", func() {
			deploy := createHTTPAddonDeploymentWithArgs("keda-http-add-on-operator", "keda-http-add-on-operator", []string{})
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			transformer := transform.ReplaceHTTPAddonOperatorLogLevel("error", scheme, logger)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())
			Expect(resultDeploy.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--zap-log-level=error"))
		})
	})

	Describe("ReplaceHTTPAddonOperatorResources", func() {
		It("should replace the operator resources", func() {
			deploy := createHTTPAddonDeployment("keda-http-add-on-operator", "keda-http-add-on-operator", "image:v1")
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			resources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}
			transformer := transform.ReplaceHTTPAddonOperatorResources(resources, scheme)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())
			Expect(resultDeploy.Spec.Template.Spec.Containers[0].Resources).To(Equal(resources))
		})
	})

	Describe("ReplaceHTTPAddonOperatorNodeSelector", func() {
		It("should merge node selectors", func() {
			deploy := createHTTPAddonDeployment("keda-http-add-on-operator", "keda-http-add-on-operator", "image:v1")
			deploy.Spec.Template.Spec.NodeSelector = map[string]string{
				"kubernetes.io/os": "linux",
			}
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			nodeSelector := map[string]string{
				"custom-label": "custom-value",
			}
			transformer := transform.ReplaceHTTPAddonOperatorNodeSelector(nodeSelector, scheme)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())
			Expect(resultDeploy.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("kubernetes.io/os", "linux"))
			Expect(resultDeploy.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("custom-label", "custom-value"))
		})
	})

	Describe("ReplaceHTTPAddonInterceptorEnv", func() {
		It("should replace existing environment variable", func() {
			deploy := createHTTPAddonDeploymentWithEnv("keda-http-add-on-interceptor", "keda-http-add-on-interceptor", []corev1.EnvVar{
				{Name: "KEDA_HTTP_CONNECT_TIMEOUT", Value: "500ms"},
			})
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			transformer := transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_CONNECT_TIMEOUT", "1s", scheme)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())

			var foundEnv corev1.EnvVar
			for _, env := range resultDeploy.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "KEDA_HTTP_CONNECT_TIMEOUT" {
					foundEnv = env
					break
				}
			}
			Expect(foundEnv.Value).To(Equal("1s"))
		})

		It("should add new environment variable if not present", func() {
			deploy := createHTTPAddonDeploymentWithEnv("keda-http-add-on-interceptor", "keda-http-add-on-interceptor", []corev1.EnvVar{})
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			transformer := transform.ReplaceHTTPAddonInterceptorEnv("NEW_ENV_VAR", "new-value", scheme)
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())

			var foundEnv corev1.EnvVar
			for _, env := range resultDeploy.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "NEW_ENV_VAR" {
					foundEnv = env
					break
				}
			}
			Expect(foundEnv.Value).To(Equal("new-value"))
		})
	})

	Describe("ReplaceHTTPAddonAllNamespaces", func() {
		It("should replace namespace in deployment metadata", func() {
			deploy := createHTTPAddonDeployment("keda-http-add-on-operator", "keda-http-add-on-operator", "image:v1")
			deploy.Namespace = "keda"
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			transformer := transform.ReplaceHTTPAddonAllNamespaces("custom-namespace")
			Expect(transformer(u)).To(Succeed())

			Expect(u.GetNamespace()).To(Equal("custom-namespace"))
		})

		It("should replace namespace in environment variables", func() {
			deploy := createHTTPAddonDeploymentWithEnv("keda-http-add-on-operator", "keda-http-add-on-operator", []corev1.EnvVar{
				{Name: "KEDA_HTTP_OPERATOR_NAMESPACE", Value: "keda"},
			})
			deploy.Namespace = "keda"
			u := &unstructured.Unstructured{}
			Expect(scheme.Convert(deploy, u, nil)).To(Succeed())

			transformer := transform.ReplaceHTTPAddonAllNamespaces("custom-namespace")
			Expect(transformer(u)).To(Succeed())

			resultDeploy := &appsv1.Deployment{}
			Expect(scheme.Convert(u, resultDeploy, nil)).To(Succeed())

			var foundEnv corev1.EnvVar
			for _, env := range resultDeploy.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "KEDA_HTTP_OPERATOR_NAMESPACE" {
					foundEnv = env
					break
				}
			}
			Expect(foundEnv.Value).To(Equal("custom-namespace"))
		})
	})
})

func createHTTPAddonDeployment(name, containerName, image string) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "keda",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: image,
						},
					},
				},
			},
		},
	}
}

func createHTTPAddonDeploymentWithArgs(name, containerName string, args []string) *appsv1.Deployment {
	deploy := createHTTPAddonDeployment(name, containerName, "image:v1")
	deploy.Spec.Template.Spec.Containers[0].Args = args
	return deploy
}

func createHTTPAddonDeploymentWithEnv(name, containerName string, env []corev1.EnvVar) *appsv1.Deployment {
	deploy := createHTTPAddonDeployment(name, containerName, "image:v1")
	deploy.Spec.Template.Spec.Containers[0].Env = env
	return deploy
}
