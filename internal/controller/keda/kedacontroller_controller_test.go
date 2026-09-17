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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mf "github.com/manifestival/manifestival"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/internal/controller/keda/gcp"
)

var _ = Describe("Deploying KedaController manifest", func() {
	const (
		olmOperatorName      = "keda-olm-operator"
		operatorName         = "keda-operator"
		kedaManifestFilepath = "../../../config/samples/keda_v1alpha1_kedacontroller.yaml"
		timeout              = time.Second * 60
		interval             = time.Millisecond * 250
		namespace            = "keda"
	)

	var (
		ctx      = context.Background()
		err      error
		scheme   *runtime.Scheme
		manifest mf.Manifest
	)

	BeforeEach(func() {
		Eventually(func() error {
			_, err = getObject(ctx, "Pod", olmOperatorName, namespace, k8sClient)
			return err
		}, timeout, interval).Should(Succeed())

	})

	var _ = Describe("Changing namespace", func() {

		const ()

		BeforeEach(func() {
			scheme = k8sManager.GetScheme()
			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			manifest, err = changeAttribute(manifest, "namespace", namespace, scheme, "")
			Expect(err).To(BeNil())

			Expect(manifest.Delete()).Should(Succeed())

			Eventually(func() error {
				_, err = getObject(ctx, "Pod", operatorName, namespace, k8sClient)
				return err
			}, timeout, interval).ShouldNot(Succeed())
		})

		Context("When deploying in \"keda\" namespace", func() {
			It("Should deploy KedaController", func() {

				Expect(manifest.Apply()).Should(Succeed())

				Eventually(func() error {
					_, err = getObject(ctx, "Pod", operatorName, namespace, k8sClient)
					return err
				}, timeout, interval).Should(Succeed())
			})
		})

		Context("When deploying not in \"keda\" namespace", func() {
			const changedNamespace = "default"

			It("Should not deploy KedaController", func() {

				manifest, err = changeAttribute(manifest, "namespace", changedNamespace, scheme, "")
				Expect(err).To(BeNil())

				Expect(manifest.Apply()).Should(Succeed())

				Eventually(func() error {
					_, err = getObject(ctx, "Pod", operatorName, namespace, k8sClient)
					return err
				}, timeout, interval).ShouldNot(Succeed())
			})
		})
	})
})

var _ = Describe("Testing functionality", func() {

	var _ = Describe("Default KedaController and Annotation Creation", func() {
		const (
			timeout                         = time.Second * 60
			interval                        = time.Millisecond * 250
			namespace                       = "keda"
			kedaDefaultControllerAnnotation = "keda-olm-operator/create-default-controller"
			deploymentName                  = "keda-operator"
		)

		var (
			ctx = context.Background()
			err error
		)

		Context("Default KedaController & annotation exist after operator installation", func() {
			It("Should find the correct annotation in the keda namespace", func() {
				kedaNamespace := &corev1.Namespace{}
				Eventually(func(g Gomega) {
					err = k8sClient.Get(ctx, types.NamespacedName{Name: namespace}, kedaNamespace)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(kedaNamespace.GetAnnotations()).To(HaveKeyWithValue(kedaDefaultControllerAnnotation, "true"))
				}, timeout, interval).Should(Succeed())
			})

			It("Should retrieve the default KedaController instance", func() {
				kedaInstance := &kedav1alpha1.KedaController{}
				Eventually(func() error {
					err = k8sClient.Get(ctx, types.NamespacedName{Name: namespace, Namespace: namespace}, kedaInstance)
					return err
				}, timeout, interval).Should(Succeed())
			})
		})
	})

	var _ = Describe("Changing operator parameters", func() {

		const (
			deploymentName       = "keda-operator"
			containerName        = "keda-operator"
			logLevelPrefix       = "--zap-log-level="
			kind                 = "KedaController"
			name                 = "keda"
			namespace            = "keda"
			kedaManifestFilepath = "../../../config/samples/keda_v1alpha1_kedacontroller.yaml"
		)

		var (
			ctx      = context.Background()
			timeout  = time.Second * 60
			interval = time.Millisecond * 250
			scheme   *runtime.Scheme
			manifest mf.Manifest
			err      error
			arg      string
			dep      = &appsv1.Deployment{}
		)

		BeforeEach(func() {
			By("Applying a manifest that defaults operator logLevel back to info")
			scheme = k8sManager.GetScheme()
			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
			manifest, err = changeAttribute(manifest, "logLevel", "info", scheme, "default")
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).Should(Succeed())

			By("Waiting for the operator deployment to reflect the changes")
			Eventually(func() error {
				return deploymentHasRolledOut(deploymentName, namespace, "default")
			}, timeout, interval).Should(Succeed())

			By("Checking to make sure loglevel is set back to info")
			u, err := getObject(ctx, "Deployment", deploymentName, namespace, k8sClient)
			Expect(err).To(BeNil())
			err = scheme.Convert(u, dep, nil)
			Expect(err).To(BeNil())

			arg, err = getDepArg(dep, logLevelPrefix, containerName)
			Expect(err).To(BeNil())
			Expect(arg).To(Equal("info"))
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
				// the default in the sample kedacontroller manifest is "info", so "info" in this list can also mean "make sure it doesn't change",
				// it is supposed to ignore these two cases because the values are invalid.
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
				caseName := fmt.Sprintf("Should change it, initialLoglevel='%s', actualLoglevel='%s'", variant.initialLogLevel, variant.actualLogLevel)
				It(caseName,
					func() {
						By(fmt.Sprintf("Setting operator loglevel to %s in kedaController manifest", variant.initialLogLevel))
						manifest, err = changeAttribute(manifest, "logLevel", variant.initialLogLevel, scheme, caseName)
						Expect(err).To(BeNil())
						err = manifest.Apply()
						Expect(err).To(BeNil())

						By("Waiting for the operator deployment to reflect the changes")
						Eventually(func() error {
							return deploymentHasRolledOut(deploymentName, namespace, caseName)
						}, timeout, interval).Should(Succeed())

						By("Checking to make sure the log level is " + variant.actualLogLevel)
						u, err := getObject(ctx, "Deployment", deploymentName, namespace, k8sClient)
						Expect(err).To(BeNil())
						err = scheme.Convert(u, dep, nil)
						Expect(err).To(BeNil())

						arg, err = getDepArg(dep, logLevelPrefix, containerName)
						Expect(err).To(BeNil())
						Expect(arg).To(Equal(variant.actualLogLevel))

					})
			}

		})
	})

	var _ = Describe("Changing webhook parameters", func() {

		const (
			deploymentName       = "keda-admission"
			containerName        = "keda-admission-webhooks"
			logLevelPrefix       = "--zap-log-level="
			kind                 = "KedaController"
			name                 = "keda"
			namespace            = "keda"
			kedaManifestFilepath = "../../../config/samples/keda_v1alpha1_kedacontroller.yaml"
		)

		var (
			ctx      = context.Background()
			timeout  = time.Second * 60
			interval = time.Millisecond * 250
			scheme   *runtime.Scheme
			manifest mf.Manifest
			err      error
			arg      string
			dep      = &appsv1.Deployment{}
		)

		BeforeEach(func() {
			By("Applying a manifest that defaults operator logLevel back to info")
			scheme = k8sManager.GetScheme()
			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
			manifest, err = changeAttribute(manifest, "logLevel", "info", scheme, "default")
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).Should(Succeed())

			By("Waiting for the operator deployment to reflect the changes")
			Eventually(func() error {
				return deploymentHasRolledOut(deploymentName, namespace, "default")
			}, timeout, interval).Should(Succeed())

			By("Checking to make sure loglevel is set back to info")
			u, err := getObject(ctx, "Deployment", deploymentName, namespace, k8sClient)
			Expect(err).To(BeNil())
			err = scheme.Convert(u, dep, nil)
			Expect(err).To(BeNil())

			arg, err = getDepArg(dep, logLevelPrefix, containerName)
			Expect(err).To(BeNil())
			Expect(arg).To(Equal("info"))
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
				// the default in the sample kedacontroller manifest is "info", so "info" in this list can also mean "make sure it doesn't change",
				// it is supposed to ignore these two cases because the values are invalid.
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
				caseName := fmt.Sprintf("Should change it, initialLoglevel='%s', actualLoglevel='%s'", variant.initialLogLevel, variant.actualLogLevel)

				It(caseName, func() {
					By(fmt.Sprintf("Setting admission loglevel to %s in kedaController manifest", variant.initialLogLevel))
					manifest, err = changeAttribute(manifest, "logLevel-admission", variant.initialLogLevel, scheme, caseName)
					Expect(err).To(BeNil())
					err = manifest.Apply()
					Expect(err).To(BeNil())

					By("Waiting for the admission deployment to reflect the changes")
					Eventually(func() error {
						return deploymentHasRolledOut(deploymentName, namespace, caseName)
					}, timeout, interval).Should(Succeed())

					By("Checking to make sure the log level is " + variant.actualLogLevel)
					u, err := getObject(ctx, "Deployment", deploymentName, namespace, k8sClient)
					Expect(err).To(BeNil())
					err = scheme.Convert(u, dep, nil)
					Expect(err).To(BeNil())

					arg, err = getDepArg(dep, logLevelPrefix, containerName)
					Expect(err).To(BeNil())
					Expect(arg).To(Equal(variant.actualLogLevel))
				})
			}
		})
	})

	var _ = Describe("Setting and removing component environment variables", func() {

		const (
			kind                 = "KedaController"
			name                 = "keda"
			namespace            = "keda"
			kedaManifestFilepath = "../../../config/samples/keda_v1alpha1_kedacontroller.yaml"
		)

		var (
			ctx      = context.Background()
			timeout  = time.Second * 60
			interval = time.Millisecond * 250
			scheme   *runtime.Scheme
			manifest mf.Manifest
			err      error
			dep      = &appsv1.Deployment{}
		)

		BeforeEach(func() {
			scheme = k8sManager.GetScheme()
			dep = &appsv1.Deployment{}

			// The operator creates a default KedaController at start-up. Wait for it to reach the
			// client cache, otherwise Apply races it and tries to create a second one.
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: namespace, Namespace: namespace}, &kedav1alpha1.KedaController{})
			}, timeout, interval).Should(Succeed())

			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
		})

		setEnv := func(instance *kedav1alpha1.KedaController, component string, env []corev1.EnvVar) error {
			switch component {
			case "operator":
				instance.Spec.Operator.Env = env
			case "metricsServer":
				instance.Spec.MetricsServer.Env = env
			case "admissionWebhooks":
				instance.Spec.AdmissionWebhooks.Env = env
			default:
				return errors.New("Not a valid component: " + component)
			}
			return nil
		}

		// KEDA_HTTP_DEFAULT_TIMEOUT ships on all three containers, so it exercises the override
		// path, while POD_NAMESPACE is a valueFrom variable the operator never touches.
		variants := []struct {
			component      string
			deploymentName string
			containerName  string
		}{
			{
				component:      "operator",
				deploymentName: "keda-operator",
				containerName:  "keda-operator",
			},
			{
				component:      "metricsServer",
				deploymentName: "keda-metrics-apiserver",
				containerName:  "keda-metrics-apiserver",
			},
			{
				component:      "admissionWebhooks",
				deploymentName: "keda-admission",
				containerName:  "keda-admission-webhooks",
			},
		}

		for _, variant := range variants {
			caseName := fmt.Sprintf("Should set them on the '%s' container and drop them again once deleted", variant.containerName)
			It(caseName, func() {
				By(fmt.Sprintf("Setting %s.env in the kedaController manifest", variant.component))
				manifest, err = mutateKedaController(manifest, scheme, caseName+" (set)", func(instance *kedav1alpha1.KedaController) error {
					return setEnv(instance, variant.component, []corev1.EnvVar{
						{Name: "KEDA_HTTP_DEFAULT_TIMEOUT", Value: "10000"},
						{Name: "KEDA_OLM_OPERATOR_TEST", Value: "example"},
					})
				})
				Expect(err).To(BeNil())
				Expect(manifest.Apply()).To(Succeed())

				By("Waiting for the deployment to reflect the changes")
				Eventually(func() error {
					return deploymentHasRolledOut(variant.deploymentName, namespace, caseName+" (set)")
				}, timeout, interval).Should(Succeed())

				u, err := getObject(ctx, "Deployment", variant.deploymentName, namespace, k8sClient)
				Expect(err).To(BeNil())
				Expect(scheme.Convert(u, dep, nil)).To(Succeed())

				By("Checking that a variable already present on the container was overridden")
				overridden, err := getDepEnv(dep, "KEDA_HTTP_DEFAULT_TIMEOUT", variant.containerName)
				Expect(err).To(BeNil())
				Expect(overridden.Value).To(Equal("10000"))

				By("Checking that a variable absent from the container was appended")
				appended, err := getDepEnv(dep, "KEDA_OLM_OPERATOR_TEST", variant.containerName)
				Expect(err).To(BeNil())
				Expect(appended.Value).To(Equal("example"))

				By("Checking that an unrelated valueFrom variable was left intact")
				untouched, err := getDepEnv(dep, "POD_NAMESPACE", variant.containerName)
				Expect(err).To(BeNil())
				Expect(untouched.ValueFrom).ToNot(BeNil())

				By(fmt.Sprintf("Deleting %s.env from the kedaController manifest", variant.component))
				manifest, err = mutateKedaController(manifest, scheme, caseName+" (cleared)", func(instance *kedav1alpha1.KedaController) error {
					return setEnv(instance, variant.component, nil)
				})
				Expect(err).To(BeNil())
				Expect(manifest.Apply()).To(Succeed())

				By("Waiting for the deployment to reflect the removal")
				Eventually(func() error {
					return deploymentHasRolledOut(variant.deploymentName, namespace, caseName+" (cleared)")
				}, timeout, interval).Should(Succeed())

				u, err = getObject(ctx, "Deployment", variant.deploymentName, namespace, k8sClient)
				Expect(err).To(BeNil())
				dep = &appsv1.Deployment{}
				Expect(scheme.Convert(u, dep, nil)).To(Succeed())

				By("Checking that the appended variable is gone")
				_, err = getDepEnv(dep, "KEDA_OLM_OPERATOR_TEST", variant.containerName)
				Expect(err).To(HaveOccurred())

				By("Checking that the overridden variable is back to the value from the operand manifest")
				restored, err := getDepEnv(dep, "KEDA_HTTP_DEFAULT_TIMEOUT", variant.containerName)
				Expect(err).To(BeNil())
				Expect(restored.Value).To(BeEmpty())
			})
		}
	})

	var _ = Describe("GCP Workload Identity Federation", func() {

		const (
			namespace            = "keda"
			operatorDeployment   = "keda-operator"
			operatorContainer    = "keda-operator"
			kedaManifestFilepath = "../../../config/samples/keda_v1alpha1_kedacontroller.yaml"
			serviceAccountEmail  = "keda@my-project.iam.gserviceaccount.com"
			audience             = "//iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/my-pool/providers/my-provider"
		)

		var (
			ctx      = context.Background()
			timeout  = time.Second * 60
			interval = time.Millisecond * 250
			scheme   *runtime.Scheme
			manifest mf.Manifest
			err      error
		)

		wifEnvVars := []string{
			gcp.EnvServiceAccountEmail, gcp.EnvAudience, gcp.EnvProjectNumber, gcp.EnvPoolID, gcp.EnvProviderID,
			gcp.EnvProjectID, gcp.EnvSubjectTokenAudience,
		}
		clearWIFEnv := func() {
			for _, name := range wifEnvVars {
				Expect(os.Unsetenv(name)).To(Succeed())
			}
		}
		secretKey := types.NamespacedName{Name: gcp.CredentialsSecretName, Namespace: namespace}

		// reconcile pushes the KedaController through the reconciler (stamping testCase on the
		// Deployments) and waits until the keda-operator Deployment reflects that run.
		reconcile := func(testCase string, mutate func(*kedav1alpha1.KedaController) error) {
			var err error
			manifest, err = mutateKedaController(manifest, scheme, testCase, mutate)
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).To(Succeed())
			Eventually(func() error {
				return deploymentHasRolledOut(operatorDeployment, namespace, testCase)
			}, timeout, interval).Should(Succeed())
		}
		noop := func(*kedav1alpha1.KedaController) error { return nil }

		getOperatorDeployment := func() *appsv1.Deployment {
			u, err := getObject(ctx, "Deployment", operatorDeployment, namespace, k8sClient)
			Expect(err).To(BeNil())
			dep := &appsv1.Deployment{}
			Expect(scheme.Convert(u, dep, nil)).To(Succeed())
			return dep
		}
		volumeNames := func(dep *appsv1.Deployment) []string {
			var names []string
			for _, v := range dep.Spec.Template.Spec.Volumes {
				names = append(names, v.Name)
			}
			return names
		}
		volumeMountNames := func(dep *appsv1.Deployment) []string {
			var names []string
			for _, c := range dep.Spec.Template.Spec.Containers {
				if c.Name == operatorContainer {
					for _, m := range c.VolumeMounts {
						names = append(names, m.Name)
					}
				}
			}
			return names
		}

		BeforeEach(func() {
			scheme = k8sManager.GetScheme()

			// The operator creates a default KedaController at start-up. Wait for it to reach the
			// client cache, otherwise Apply races it and tries to create a second one.
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: namespace, Namespace: namespace}, &kedav1alpha1.KedaController{})
			}, timeout, interval).Should(Succeed())

			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
			clearWIFEnv()
		})

		AfterEach(func() {
			// Leave the operand the way the other specs expect it: no WIF variables, and a
			// reconcile that has dropped the Secret and the Deployment wiring again.
			clearWIFEnv()
			reconcile(CurrentSpecReport().LeafNodeText+" (cleanup)", noop)
			Eventually(func() error {
				err := k8sClient.Get(ctx, secretKey, &corev1.Secret{})
				if err == nil {
					return errors.New("GCP credentials Secret still exists")
				}
				if k8serrors.IsNotFound(err) {
					return nil
				}
				return err
			}, timeout, interval).Should(Succeed())
		})

		It("Should create the credentials Secret and wire it into keda-operator from the console variables", func() {
			caseName := CurrentSpecReport().LeafNodeText

			By("Configuring WIF the way the OperatorHub console does through the Subscription")
			Expect(os.Setenv(gcp.EnvProjectNumber, "123456789012")).To(Succeed())
			Expect(os.Setenv(gcp.EnvPoolID, "my-pool")).To(Succeed())
			Expect(os.Setenv(gcp.EnvProviderID, "my-provider")).To(Succeed())
			Expect(os.Setenv(gcp.EnvServiceAccountEmail, serviceAccountEmail)).To(Succeed())
			Expect(os.Setenv(gcp.EnvProjectID, "my-project")).To(Succeed())
			reconcile(caseName, noop)

			By("Checking the credentials Secret")
			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, secretKey, secret)
			}, timeout, interval).Should(Succeed())
			Expect(secret.Data).To(HaveKey(gcp.CredentialsSecretKey))
			var creds map[string]any
			Expect(json.Unmarshal(secret.Data[gcp.CredentialsSecretKey], &creds)).To(Succeed())
			Expect(creds).To(HaveKeyWithValue("type", "external_account"))
			Expect(creds).To(HaveKeyWithValue("audience", audience))
			Expect(creds).To(HaveKeyWithValue("token_url", "https://sts.googleapis.com/v1/token"))
			Expect(creds).To(HaveKeyWithValue("service_account_impersonation_url",
				"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/"+serviceAccountEmail+":generateAccessToken"))
			Expect(creds).To(HaveKeyWithValue("credential_source", HaveKeyWithValue("file", gcp.SubjectTokenFilePath)))
			Expect(creds).ToNot(HaveKey("universe_domain"))

			By("Checking that the Secret is owned by the KedaController")
			kedaController := &kedav1alpha1.KedaController{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: namespace, Namespace: namespace}, kedaController)).To(Succeed())
			Expect(metav1.IsControlledBy(secret, kedaController)).To(BeTrue())

			By("Checking the keda-operator Deployment wiring")
			dep := getOperatorDeployment()
			Expect(volumeNames(dep)).To(ContainElements("bound-sa-token", "gcp-credentials"))
			Expect(volumeMountNames(dep)).To(ContainElements("bound-sa-token", "gcp-credentials"))
			for _, v := range dep.Spec.Template.Spec.Volumes {
				if v.Name == "bound-sa-token" {
					Expect(v.Projected).ToNot(BeNil())
					Expect(v.Projected.Sources).To(HaveLen(1))
					Expect(v.Projected.Sources[0].ServiceAccountToken).ToNot(BeNil())
					Expect(v.Projected.Sources[0].ServiceAccountToken.Audience).To(Equal(gcp.DefaultSubjectTokenAudience))
				}
				if v.Name == "gcp-credentials" {
					Expect(v.Secret).ToNot(BeNil())
					Expect(v.Secret.SecretName).To(Equal(gcp.CredentialsSecretName))
				}
			}
			appCreds, err := getDepEnv(dep, "GOOGLE_APPLICATION_CREDENTIALS", operatorContainer)
			Expect(err).To(BeNil())
			Expect(appCreds.Value).To(Equal(gcp.CredentialsFilePath))
			project, err := getDepEnv(dep, "CLOUDSDK_CORE_PROJECT", operatorContainer)
			Expect(err).To(BeNil())
			Expect(project.Value).To(Equal("my-project"))
			Expect(dep.Spec.Template.Annotations).To(HaveKeyWithValue(gcp.CredentialsHashAnnotation, gcp.CredentialsHash(secret.Data[gcp.CredentialsSecretKey])))
		})

		It("Should honor the CLI variables, a custom token audience and spec.operator.env overrides", func() {
			caseName := CurrentSpecReport().LeafNodeText

			By("Configuring WIF the way a CLI install does through the Subscription")
			Expect(os.Setenv(gcp.EnvAudience, audience)).To(Succeed())
			Expect(os.Setenv(gcp.EnvServiceAccountEmail, serviceAccountEmail)).To(Succeed())
			Expect(os.Setenv(gcp.EnvProjectID, "my-project")).To(Succeed())
			Expect(os.Setenv(gcp.EnvSubjectTokenAudience, "sts.googleapis.com")).To(Succeed())
			reconcile(caseName, func(instance *kedav1alpha1.KedaController) error {
				instance.Spec.Operator.Env = []corev1.EnvVar{{Name: "CLOUDSDK_CORE_PROJECT", Value: "other-project"}}
				return nil
			})

			dep := getOperatorDeployment()
			By("Checking that the token audience from the environment is used")
			for _, v := range dep.Spec.Template.Spec.Volumes {
				if v.Name == "bound-sa-token" {
					Expect(v.Projected.Sources[0].ServiceAccountToken.Audience).To(Equal("sts.googleapis.com"))
				}
			}
			By("Checking that the user-defined variable wins over the one derived from the environment")
			project, err := getDepEnv(dep, "CLOUDSDK_CORE_PROJECT", operatorContainer)
			Expect(err).To(BeNil())
			Expect(project.Value).To(Equal("other-project"))
			appCreds, err := getDepEnv(dep, "GOOGLE_APPLICATION_CREDENTIALS", operatorContainer)
			Expect(err).To(BeNil())
			Expect(appCreds.Value).To(Equal(gcp.CredentialsFilePath))

			By("Checking the Secret carries the audience from the environment")
			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, secretKey, secret)
			}, timeout, interval).Should(Succeed())
			var creds map[string]any
			Expect(json.Unmarshal(secret.Data[gcp.CredentialsSecretKey], &creds)).To(Succeed())
			Expect(creds).To(HaveKeyWithValue("audience", audience))

			By("Dropping the user-defined variable again")
			reconcile(caseName+" (env cleared)", func(instance *kedav1alpha1.KedaController) error {
				instance.Spec.Operator.Env = nil
				return nil
			})
			project, err = getDepEnv(getOperatorDeployment(), "CLOUDSDK_CORE_PROJECT", operatorContainer)
			Expect(err).To(BeNil())
			Expect(project.Value).To(Equal("my-project"))
		})

		It("Should remove the Secret and the wiring once the variables are gone", func() {
			caseName := CurrentSpecReport().LeafNodeText

			By("Enabling WIF")
			Expect(os.Setenv(gcp.EnvAudience, audience)).To(Succeed())
			Expect(os.Setenv(gcp.EnvServiceAccountEmail, serviceAccountEmail)).To(Succeed())
			reconcile(caseName+" (enabled)", noop)
			Eventually(func() error {
				return k8sClient.Get(ctx, secretKey, &corev1.Secret{})
			}, timeout, interval).Should(Succeed())
			Expect(volumeNames(getOperatorDeployment())).To(ContainElement("gcp-credentials"))

			By("Removing the variables, as if the Subscription had been edited")
			clearWIFEnv()
			reconcile(caseName+" (disabled)", noop)

			By("Checking that the Secret is gone")
			Eventually(func() error {
				err := k8sClient.Get(ctx, secretKey, &corev1.Secret{})
				if err == nil {
					return errors.New("GCP credentials Secret still exists")
				}
				if k8serrors.IsNotFound(err) {
					return nil
				}
				return err
			}, timeout, interval).Should(Succeed())

			By("Checking that the Deployment wiring is gone")
			dep := getOperatorDeployment()
			Expect(volumeNames(dep)).ToNot(ContainElements("bound-sa-token", "gcp-credentials"))
			Expect(volumeMountNames(dep)).ToNot(ContainElements("bound-sa-token", "gcp-credentials"))
			_, err = getDepEnv(dep, "GOOGLE_APPLICATION_CREDENTIALS", operatorContainer)
			Expect(err).To(HaveOccurred())
			_, err = getDepEnv(dep, "CLOUDSDK_CORE_PROJECT", operatorContainer)
			Expect(err).To(HaveOccurred())
			Expect(dep.Spec.Template.Annotations).ToNot(HaveKey(gcp.CredentialsHashAnnotation))
		})

		// Secrets of our name that the operator must not take over: one controlled by
		// another object, and an unowned one of another type (the type is immutable, so
		// it can't be turned into the credential Secret in place).
		isController := true
		foreignSecrets := []struct {
			description string
			secret      corev1.Secret
		}{
			{
				description: "it is controlled by someone else",
				secret: corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion: "v1", Kind: "ConfigMap", Name: "someone-else", UID: "00000000-0000-0000-0000-000000000001",
							Controller: &isController,
						}},
					},
					Data: map[string][]byte{"foreign": []byte("data")},
				},
			},
			{
				description: "it is of another type",
				secret: corev1.Secret{
					Type: "example.com/foreign",
					Data: map[string][]byte{"foreign": []byte("data")},
				},
			},
		}

		for _, variant := range foreignSecrets {
			It("Should leave a Secret of the same name alone when "+variant.description, func() {
				caseName := CurrentSpecReport().LeafNodeText

				By("Creating the foreign Secret")
				foreign := variant.secret.DeepCopy()
				foreign.Name = secretKey.Name
				foreign.Namespace = secretKey.Namespace
				Expect(k8sClient.Create(ctx, foreign)).To(Succeed())

				By("Enabling WIF")
				Expect(os.Setenv(gcp.EnvAudience, audience)).To(Succeed())
				Expect(os.Setenv(gcp.EnvServiceAccountEmail, serviceAccountEmail)).To(Succeed())
				manifest, err = mutateKedaController(manifest, scheme, caseName, noop)
				Expect(err).To(BeNil())
				Expect(manifest.Apply()).To(Succeed())

				By("Checking that the KedaController reports the conflict and the Secret is untouched")
				kedaController := &kedav1alpha1.KedaController{}
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: namespace, Namespace: namespace}, kedaController)).To(Succeed())
					g.Expect(kedaController.Status.Phase).To(Equal(kedav1alpha1.PhaseFailed))
				}, timeout, interval).Should(Succeed())
				current := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, secretKey, current)).To(Succeed())
				Expect(current.Data).To(HaveKey("foreign"))
				Expect(current.Data).ToNot(HaveKey(gcp.CredentialsSecretKey))
				Expect(current.Type).To(Equal(foreign.Type))
				Expect(metav1.IsControlledBy(current, kedaController)).To(BeFalse())

				By("Removing the foreign Secret so the reconcile can recover")
				Expect(k8sClient.Delete(ctx, current)).To(Succeed())
				Eventually(func() error {
					err := k8sClient.Get(ctx, secretKey, &corev1.Secret{})
					if err == nil {
						return errors.New("foreign Secret still exists")
					}
					if k8serrors.IsNotFound(err) {
						return nil
					}
					return err
				}, timeout, interval).Should(Succeed())
				reconcile(caseName+" (recovered)", noop)
				Eventually(func(g Gomega) {
					secret := &corev1.Secret{}
					g.Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
					g.Expect(secret.Data).To(HaveKey(gcp.CredentialsSecretKey))
				}, timeout, interval).Should(Succeed())
			})
		}

		It("Should mark the KedaController as failed when the configuration is incomplete", func() {
			caseName := CurrentSpecReport().LeafNodeText

			By("Setting only the service account email")
			Expect(os.Setenv(gcp.EnvServiceAccountEmail, serviceAccountEmail)).To(Succeed())
			manifest, err = mutateKedaController(manifest, scheme, caseName, noop)
			Expect(err).To(BeNil())
			Expect(manifest.Apply()).To(Succeed())

			By("Checking the KedaController status")
			Eventually(func(g Gomega) {
				kedaController := &kedav1alpha1.KedaController{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: namespace, Namespace: namespace}, kedaController)).To(Succeed())
				g.Expect(kedaController.Status.Phase).To(Equal(kedav1alpha1.PhaseFailed))
				g.Expect(kedaController.Status.Reason).To(ContainSubstring("Invalid GCP Workload Identity configuration"))
				g.Expect(kedaController.Status.Reason).To(ContainSubstring("AUDIENCE"))
			}, timeout, interval).Should(Succeed())

			By("Checking that no Secret was created")
			Expect(k8serrors.IsNotFound(k8sClient.Get(ctx, secretKey, &corev1.Secret{}))).To(BeTrue())

			By("Fixing the configuration")
			clearWIFEnv()
			reconcile(caseName+" (fixed)", noop)
			Eventually(func(g Gomega) {
				kedaController := &kedav1alpha1.KedaController{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: namespace, Namespace: namespace}, kedaController)).To(Succeed())
				g.Expect(kedaController.Status.Phase).To(Equal(kedav1alpha1.PhaseInstallSucceeded))
			}, timeout, interval).Should(Succeed())
		})
	})

})

var _ = Describe("Testing audit flags", func() {
	const (
		metricsServerName    = "keda-metrics-apiserver"
		kind                 = "KedaController"
		name                 = "keda"
		namespace            = "keda"
		kedaManifestFilepath = "../../../config/samples/keda_v1alpha1_kedacontroller.yaml"
	)

	var (
		ctx      = context.Background()
		timeout  = time.Second * 60
		interval = time.Millisecond * 250
		scheme   *runtime.Scheme
		manifest mf.Manifest
		err      error
		dep      = &appsv1.Deployment{}
	)

	When("Manipulating parameters", func() {

		// TODO(jkyros): come back to refactor this test, it doesn't proeprly reset between test cases, and
		// if it did, it would be failing, because it's currently impossible to deconfigure audit because of
		// how our manifestival code only operates if the settings are different from default/empty.
		BeforeEach(func() {
			scheme = k8sManager.GetScheme()
			dep = &appsv1.Deployment{}
			manifest, err = createManifest(kedaManifestFilepath, k8sClient)
			Expect(err).To(BeNil())
		})

		Context("to add audit configuration flags", func() {
			vars := []struct {
				argument string
				prefix   string
				value    string
			}{
				{
					argument: "auditLogFormat",
					prefix:   "--audit-log-format=",
					value:    "json",
				},
				{
					argument: "auditMaxAge",
					prefix:   "--audit-log-maxage=",
					value:    "1",
				},
				{
					argument: "auditMaxBackup",
					prefix:   "--audit-log-maxbackup=",
					value:    "2",
				},
				{
					argument: "auditLogMaxSize",
					prefix:   "--audit-log-maxsize=",
					value:    "3",
				},
			}
			for _, variant := range vars {
				caseName := fmt.Sprintf("adds '%s' with value '%s'", variant.argument, variant.value)
				It(caseName, func() {
					manifest, err := changeAttribute(manifest, variant.argument, variant.value, scheme, caseName)
					Expect(err).To(BeNil())

					Expect(manifest.Apply()).To(Succeed())
					Eventually(func() error {
						return deploymentHasRolledOut(metricsServerName, namespace, caseName)

					}, timeout, interval).Should(Succeed())
					u, err := getObject(ctx, "Deployment", metricsServerName, namespace, k8sClient)
					Expect(err).To(BeNil())

					Expect(scheme.Convert(u, dep, nil)).To(Succeed())
					arg, err := getDepArg(dep, variant.prefix, metricsServerName)
					Expect(err).To(BeNil())
					Expect(arg).To(Equal(variant.value))
				})
			}

		})
	})
})

func getDepArg(dep *appsv1.Deployment, prefix string, containerName string) (string, error) {
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

func getDepEnv(dep *appsv1.Deployment, name string, containerName string) (*corev1.EnvVar, error) {
	for _, container := range dep.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			for i, env := range container.Env {
				if env.Name == name {
					return &container.Env[i], nil
				}
			}
			return nil, errors.New("Could not find an environment variable named: " + name)
		}
	}
	return nil, errors.New("Could not find a container: " + containerName)
}

// mutateKedaController applies mutate to the KedaController in the manifest, stamping the test case
// annotation on every component so deploymentHasRolledOut can tell our change apart from a previous one.
func mutateKedaController(manifest mf.Manifest, scheme *runtime.Scheme, annotation string, mutate func(*kedav1alpha1.KedaController) error) (mf.Manifest, error) {
	transformer := func(u *unstructured.Unstructured) error {
		kedaControllerInstance := &kedav1alpha1.KedaController{}
		if err := scheme.Convert(u, kedaControllerInstance, nil); err != nil {
			return err
		}

		// Annotations might be nil, so we need to make sure we account for that
		if kedaControllerInstance.Spec.Operator.DeploymentAnnotations == nil {
			kedaControllerInstance.Spec.Operator.DeploymentAnnotations = make(map[string]string)
		}

		if kedaControllerInstance.Spec.AdmissionWebhooks.DeploymentAnnotations == nil {
			kedaControllerInstance.Spec.AdmissionWebhooks.DeploymentAnnotations = make(map[string]string)
		}
		if kedaControllerInstance.Spec.MetricsServer.DeploymentAnnotations == nil {
			kedaControllerInstance.Spec.MetricsServer.DeploymentAnnotations = make(map[string]string)
		}
		// When we push through an attribute change, we also set an annotation matching the test case
		// so we can be sure the controller reacted to our kedaControllerInstance updates and we have "our"
		// changes, not just some changes that might be from a previous test case
		kedaControllerInstance.Spec.Operator.DeploymentAnnotations["testCase"] = annotation
		kedaControllerInstance.Spec.AdmissionWebhooks.DeploymentAnnotations["testCase"] = annotation
		kedaControllerInstance.Spec.MetricsServer.DeploymentAnnotations["testCase"] = annotation

		if err := mutate(kedaControllerInstance); err != nil {
			return err
		}
		return scheme.Convert(kedaControllerInstance, u, nil)
	}

	return manifest.Transform(transformer)
}

func changeAttribute(manifest mf.Manifest, attr string, value string, scheme *runtime.Scheme, annotation string) (mf.Manifest, error) {
	return mutateKedaController(manifest, scheme, annotation, func(kedaControllerInstance *kedav1alpha1.KedaController) error {
		switch attr {
		case "namespace":
			kedaControllerInstance.Namespace = value
		case "logLevel":
			kedaControllerInstance.Spec.Operator.LogLevel = value
		// TODO(jkyros): this breaks pattern with the rest of these cases but multiple operands have
		// the same field, but we kind of bolted the admission tests on here without doing a refactor and
		// this makes it work for now
		case "logLevel-admission":
			kedaControllerInstance.Spec.AdmissionWebhooks.LogLevel = value
		// metricsServer audit arguments
		case "auditLogFormat":
			kedaControllerInstance.Spec.MetricsServer.LogFormat = value
		case "auditMaxAge":
			kedaControllerInstance.Spec.MetricsServer.MaxAge = value
		case "auditMaxBackup":
			kedaControllerInstance.Spec.MetricsServer.MaxBackup = value
		case "auditLogMaxSize":
			kedaControllerInstance.Spec.MetricsServer.MaxSize = value
		default:
			return errors.New("Not a valid attribute")
		}
		return nil
	})
}

// deploymentHasRolledOut waits for the specified deployment to possess the specified annotation
//
//nolint:unparam
func deploymentHasRolledOut(deploymentName string, namespace string, deploymentAnnotation string) error {
	u, err := getObject(ctx, "Deployment", deploymentName, namespace, k8sClient)
	if err != nil {
		return err
	}
	// The default manifest has no annotation, so I'm not checking whether it's present, only the value,
	// because the default case will not have the annotation, and we want that to be okay in the case where we
	// apply the default.
	testcase := u.GetAnnotations()["testCase"]
	if deploymentAnnotation == testcase {
		By("Observing that the test case annotation is now: " + testcase)
		return nil
	}
	return fmt.Errorf("Deployment has not rolled out, annotation is still '%s'", testcase)
}
