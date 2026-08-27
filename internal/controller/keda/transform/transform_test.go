/*
Copyright 2023 The KEDA Authors

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
	"strings"

	mf "github.com/manifestival/manifestival"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kedacore/keda-olm-operator/internal/controller/keda/transform"
)

var _ = Describe("Transforming all resource namespaces", func() {
	var _ = Describe("Changing namespace", func() {
		Context("When transforming a ServiceAccount", func() {

			yamlData := `---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app.kubernetes.io/name: keda-operator
    app.kubernetes.io/part-of: keda-operator
    app.kubernetes.io/version: 2.10.1
  name: keda-operator
  namespace: keda
`
			It("Should be able to change the object's metadata.namespace field", func() {

				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				testNs := "default"
				transforms := []mf.Transformer{transform.ReplaceAllNamespaces(testNs)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())

				r := newManifest.Resources()
				Expect(len(r)).To(Equal(1))
				Expect(r[0].GetNamespace()).To(Equal(testNs))
			})
		})

		Context("When transforming an APIService", func() {

			yamlData := `---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  labels:
    app.kubernetes.io/name: v1beta1.external.metrics.k8s.io
    app.kubernetes.io/part-of: keda-operator
    app.kubernetes.io/version: 2.10.1
  name: v1beta1.external.metrics.k8s.io
spec:
  group: external.metrics.k8s.io
  groupPriorityMinimum: 100
  service:
    name: keda-metrics-apiserver
    namespace: keda
  version: v1beta1
  versionPriority: 100
`
			It("Should be able to change the namespace in the object's spec.service.namespace field", func() {
				if testType != "unit" {
					Skip("test.type isn't 'unit'")
				}

				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				testNs := "default"
				transforms := []mf.Transformer{transform.ReplaceAllNamespaces(testNs)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())

				r := newManifest.Resources()
				Expect(len(r)).To(Equal(1))
				// get spec.service.namespace from the result
				ns, found, err := unstructured.NestedString(r[0].UnstructuredContent(), "spec", "service", "namespace")
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
				Expect(ns).To(Equal(testNs))
			})
		})
	})
})

// TODO(jkyros): test the volume/mount injection, make sure it doesn't regress audit volume stuff
var _ = Describe("Transforming deployment spec for volumes", func() {
	var _ = Describe("Overriding volumes", func() {
		BeforeEach(func() {
			if testType != "unit" {
				Skip("test.type isn't 'unit'")
			}
		})
		Context("When transforming a Deployment", func() {

			// Set up the scheme, the transformer uses this to convert from unstructured, so we need this
			scheme := runtime.NewScheme()
			_ = appsv1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			yamlData := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-keda-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: keda
  template:
    metadata:
      labels:
        app: keda
    spec:
      containers:
        - name: keda-operator
          image: keda:latest
          volumeMounts:
            - name: example-volume
              mountPath: /example
      volumes:
        - name: example-volume
          configMap:
            name: example`

			// This is testing: https://keda.sh/docs/2.14/scalers/apache-kafka/#your-kafka-cluster-turns-on-saslgssapi-auth-without-tls since the OLM operator
			// manages it, users need a way to add the volumes
			It("Should be able to add a volume to the template.spec.volumes field", func() {

				By("Adding a volume to a deployment")
				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				var desiredVolumes = []corev1.Volume{{Name: "temp-kerberos-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}}}
				transforms := []mf.Transformer{transform.ReplaceDeploymentVolumes(desiredVolumes, scheme)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())

				// make sure we got a manifest back
				r := newManifest.Resources()
				Expect(len(r)).To(Equal(1))

				By("Making sure the volumes are correct")
				// get spec.service.namespace from the result
				volumes, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "volumes")
				Expect(found).To(BeTrue(), "spec.volumes should exist")
				Expect(volumes).To(ContainElement(structuredToMap(desiredVolumes[0])))
				Expect(err).To(BeNil())
			})

			It("Should be able to add a volumeMount to the spec.containers.volumeMounts field", func() {

				By("Adding a volume to a deployment container")
				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				var desiredVolumeMounts = []corev1.VolumeMount{{Name: "temp-kerberos-vol", MountPath: "/tmp/kerberos", ReadOnly: false}}
				transforms := []mf.Transformer{transform.ReplaceDeploymentVolumeMounts(desiredVolumeMounts, scheme)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())

				// make sure we got a manifest back
				r := newManifest.Resources()
				Expect(len(r)).To(Equal(1))

				By("Making sure the volume mounts are correct")
				// grab the list of containers in the deployment template
				containers, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "containers")
				Expect(found).To(BeTrue(), "spec.template.spec.containers should exist")
				Expect(err).To(BeNil())

				// grab the list of volume mounts
				volumeMounts, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "volumeMounts")
				Expect(found).To(BeTrue(), "spec.template.spec.containers.volumeMounts should exist")
				Expect(err).To(BeNil())
				// make sure our element is in the list
				Expect(volumeMounts).To(ContainElement(structuredToMap(desiredVolumeMounts[0])))
			})

			It("Should be able to replace a volume in the spec.volumes field", func() {

				By("Adding a volume to a deployment")
				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				var desiredVolumes = []corev1.Volume{{Name: "temp-kerberos-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}}}
				transforms := []mf.Transformer{transform.ReplaceDeploymentVolumes(desiredVolumes, scheme)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())

				// make sure we got a manifest back
				r := newManifest.Resources()
				Expect(len(r)).To(Equal(1))

				By("Replacing volume in deployment")
				var replaceVolumes = []corev1.Volume{{Name: "temp-kerberos-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumHugePages}}}}
				transforms = []mf.Transformer{transform.ReplaceDeploymentVolumes(replaceVolumes, scheme)}
				newManifest, err = manifest.Transform(transforms...)
				Expect(err).To(BeNil())

				// make sure we got a manifest back
				r = newManifest.Resources()
				Expect(len(r)).To(Equal(1))

				By("Making sure the deployment's volumes are correct")
				// get spec.service.namespace from the result
				volumes, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "volumes")
				Expect(found).To(BeTrue(), "spec.volumes should exist")
				Expect(volumes).To(ContainElement(structuredToMap(replaceVolumes[0])))
				Expect(volumes).NotTo(ContainElement(structuredToMap(desiredVolumes[0])))
				Expect(err).To(BeNil())
			})

			It("Should be able to replace a volumeMount in the spec.containers.VolumeMounts field", func() {

				By("Adding a volume mount to a deployment container")
				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				var desiredVolumeMounts = []corev1.VolumeMount{{Name: "temp-kerberos-vol", MountPath: "/tmp/kerberos", ReadOnly: false}}
				transforms := []mf.Transformer{transform.ReplaceDeploymentVolumeMounts(desiredVolumeMounts, scheme)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())
				r := newManifest.Resources()
				Expect(len(r)).To(Equal(1))

				By("Replacing the volume mount of a deployment container")
				var replaceVolumeMounts = []corev1.VolumeMount{{Name: "temp-kerberos-vol", MountPath: "/tmp/kerberosreplaced", ReadOnly: false}}
				transforms = []mf.Transformer{transform.ReplaceDeploymentVolumeMounts(replaceVolumeMounts, scheme)}
				newManifest, err = manifest.Transform(transforms...)
				Expect(err).To(BeNil())
				r = newManifest.Resources()
				Expect(len(r)).To(Equal(1))

				By("Checking to see if the volume mounts are correct")
				// grab the list of containers
				containers, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "containers")
				Expect(found).To(BeTrue(), "spec.template.spec.containers should exist")
				Expect(err).To(BeNil())

				// grab the list of volume mounts for the first container
				By("Making sure the deployment's volumes are correct")
				volumeMounts, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "volumeMounts")
				Expect(found).To(BeTrue(), "spec.template.spec.containers.volumeMounts should exist")
				Expect(err).To(BeNil())

				// make sure we have the replacement, not the original
				Expect(volumeMounts).To(ContainElement(structuredToMap(replaceVolumeMounts[0])))
				Expect(volumeMounts).NotTo(ContainElement(structuredToMap(desiredVolumeMounts[0])))
			})

			// Test to verify pre-existing volumeMounts are preserved when adding new ones
			It("Should preserve pre-existing volumeMounts when adding new ones (merge, not replace)", func() {

				By("Starting with a deployment that has a pre-existing volumeMount")
				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				// First, verify the pre-existing mount exists
				containers, found, err := unstructured.NestedSlice(manifest.Resources()[0].UnstructuredContent(), "spec", "template", "spec", "containers")
				Expect(found).To(BeTrue(), "spec.template.spec.containers should exist")
				Expect(err).To(BeNil())

				originalVolumeMounts, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "volumeMounts")
				Expect(found).To(BeTrue(), "original volumeMounts should exist")
				Expect(err).To(BeNil())
				originalCount := len(originalVolumeMounts)
				Expect(originalCount).To(BeNumerically(">", 0), "deployment should have at least one pre-existing volumeMount")

				preExistingMount := corev1.VolumeMount{Name: "example-volume", MountPath: "/example"}
				Expect(originalVolumeMounts).To(ContainElement(structuredToMap(preExistingMount)), "pre-existing mount should exist before transformation")

				By("Adding a NEW volumeMount to the deployment")
				var desiredVolumeMounts = []corev1.VolumeMount{{Name: "temp-kerberos-vol", MountPath: "/tmp/kerberos", ReadOnly: false}}
				transforms := []mf.Transformer{transform.ReplaceDeploymentVolumeMounts(desiredVolumeMounts, scheme)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())
				r := newManifest.Resources()
				Expect(len(r)).To(Equal(1))

				By("Verifying BOTH the pre-existing AND new volumeMounts exist (merge behavior)")
				containers, found, err = unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "containers")
				Expect(found).To(BeTrue(), "spec.template.spec.containers should exist")
				Expect(err).To(BeNil())

				volumeMounts, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "volumeMounts")
				Expect(found).To(BeTrue(), "volumeMounts should exist after transformation")
				Expect(err).To(BeNil())

				Expect(volumeMounts).To(HaveLen(originalCount+1), "volumeMount count should increase by 1, not be replaced")
				Expect(volumeMounts).To(ContainElement(structuredToMap(preExistingMount)), "pre-existing volumeMount MUST be preserved")
				Expect(volumeMounts).To(ContainElement(structuredToMap(desiredVolumeMounts[0])), "new volumeMount should be added")
			})

			// Test to verify multiple additions preserve all mounts
			It("Should preserve all pre-existing volumeMounts when adding multiple new ones", func() {

				By("Starting with a deployment that has a pre-existing volumeMount")
				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				By("Adding multiple NEW volumeMounts")
				var desiredVolumeMounts = []corev1.VolumeMount{
					{Name: "mount-1", MountPath: "/mount1", ReadOnly: false},
					{Name: "mount-2", MountPath: "/mount2", ReadOnly: true},
					{Name: "mount-3", MountPath: "/mount3", ReadOnly: false},
				}
				transforms := []mf.Transformer{transform.ReplaceDeploymentVolumeMounts(desiredVolumeMounts, scheme)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())
				r := newManifest.Resources()

				By("Verifying all volumeMounts exist")
				containers, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "containers")
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())

				volumeMounts, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "volumeMounts")
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())

				// Should have: 1 pre-existing + 3 new = 4 total
				Expect(volumeMounts).To(HaveLen(4), "should have pre-existing + all new mounts")

				// Verify pre-existing is preserved
				preExistingMount := corev1.VolumeMount{Name: "example-volume", MountPath: "/example"}
				Expect(volumeMounts).To(ContainElement(structuredToMap(preExistingMount)), "pre-existing mount must be preserved")

				// Verify all new mounts are added
				for _, mount := range desiredVolumeMounts {
					Expect(volumeMounts).To(ContainElement(structuredToMap(mount)), "new mount %s should be added", mount.Name)
				}
			})

			// Test replacement behavior while preserving other mounts
			It("Should preserve other volumeMounts when replacing a specific mount", func() {

				By("Adding two volumeMounts first")
				manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
				Expect(err).To(BeNil())

				var initialMounts = []corev1.VolumeMount{
					{Name: "mount-1", MountPath: "/mount1", ReadOnly: false},
					{Name: "mount-2", MountPath: "/mount2", ReadOnly: false},
				}
				transforms := []mf.Transformer{transform.ReplaceDeploymentVolumeMounts(initialMounts, scheme)}
				newManifest, err := manifest.Transform(transforms...)
				Expect(err).To(BeNil())

				By("Replacing mount-1 with a different path")
				var replacementMount = []corev1.VolumeMount{
					{Name: "mount-1", MountPath: "/mount1-replaced", ReadOnly: true}, // same name, different path
				}
				transforms = []mf.Transformer{transform.ReplaceDeploymentVolumeMounts(replacementMount, scheme)}
				newManifest, err = newManifest.Transform(transforms...)
				Expect(err).To(BeNil())
				r := newManifest.Resources()

				By("Verifying mount-1 is replaced but mount-2 and example-volume are preserved")
				containers, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "containers")
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())

				volumeMounts, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "volumeMounts")
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())

				// Should have: example-volume + mount-1 (replaced) + mount-2 = 3 total
				Expect(volumeMounts).To(HaveLen(3), "should have pre-existing + replaced + preserved mounts")

				// Verify mount-1 is replaced
				Expect(volumeMounts).To(ContainElement(structuredToMap(replacementMount[0])), "mount-1 should be replaced")
				Expect(volumeMounts).NotTo(ContainElement(structuredToMap(initialMounts[0])), "old mount-1 should not exist")

				// Verify mount-2 is preserved
				Expect(volumeMounts).To(ContainElement(structuredToMap(initialMounts[1])), "mount-2 should be preserved")

				// Verify pre-existing example-volume is preserved
				preExistingMount := corev1.VolumeMount{Name: "example-volume", MountPath: "/example"}
				Expect(volumeMounts).To(ContainElement(structuredToMap(preExistingMount)), "pre-existing mount should be preserved")
			})
		})
	})
})

var _ = Describe("Updating a NetworkPolicy", func() {
	Context("When transforming the keda-allow-egress-to-all policy", func() {

		yamlData := `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  namespace: keda
  labels:
    app.kubernetes.io/name: keda
  name: keda-allow-egress-to-all
spec:
  egress:
  - ports:
    - port: 1
      endPort: 65535
      protocol: TCP
    - port: 1
      endPort: 65535
      protocol: UDP
  podSelector:
    matchExpressions:
    - {key: app, operator: In, values: [keda-operator, keda-metrics-apiserver]}
  policyTypes:
  - Egress
`
		logger := ctrl.Log.WithName("test")
		// Set up the scheme; the transformer uses this to convert from unstructured, so we need this
		scheme := runtime.NewScheme()
		_ = networkingv1.AddToScheme(scheme)
		It("Should be able to remove the pod selectors", func() {
			manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
			Expect(err).To(BeNil())

			transforms := []mf.Transformer{
				transform.RemoveNetworkPolicyPodSelectorFromKedaOperator(scheme, logger),
				transform.RemoveNetworkPolicyPodSelectorFromMetricsServer(scheme, logger),
			}
			newManifest, err := manifest.Transform(transforms...)
			Expect(err).To(BeNil())

			r := newManifest.Resources()
			Expect(len(r)).To(Equal(1))
			_, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "podSelector", "matchExpressions")
			Expect(found).To(BeFalse(), "podSelector.matchExpressions shouldn't exist")
			Expect(err).To(BeNil())
		})
	})
	Context("When transforming the keda-allow-egress-to-dns policy", func() {

		yamlData := `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  namespace: keda
  labels:
    app.kubernetes.io/name: keda
  name: keda-allow-egress-to-dns
spec:
  egress:
  - ports:
    - port: 5353
      protocol: TCP
    - port: 5353
      protocol: UDP
  podSelector:
    matchExpressions:
    - {key: app, operator: In, values: [keda-operator, keda-admission-webhooks, keda-metrics-apiserver]}
  policyTypes:
  - Egress
`
		// Set up the scheme; the transformer uses this to convert from unstructured, so we need this
		scheme := runtime.NewScheme()
		_ = networkingv1.AddToScheme(scheme)
		It("Should be able to remove the pod selectors", func() {
			manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
			Expect(err).To(BeNil())

			newManifest, err := manifest.Transform(transform.AddOpenShiftPodToDNSNetworkPolicy(scheme))
			Expect(err).To(BeNil())

			r := newManifest.Resources()
			Expect(len(r)).To(Equal(1))
			egress, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "egress")
			Expect(found).To(BeTrue(), "spec.egress should exist")
			Expect(len(egress)).To(Equal(1))
			Expect(err).To(BeNil())
			peers, found, err := unstructured.NestedSlice(egress[0].(map[string]interface{}), "to")
			Expect(found).To(BeTrue(), "spec.egress[0].to should exist")
			Expect(len(peers)).To(Equal(1))
			desiredPeer := &networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "openshift-dns"}},
				PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"dns.operator.openshift.io/daemonset-dns": "default"}},
			}
			Expect(peers).To(ContainElement(structuredToMap(desiredPeer)))
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("Transforming operator deployment for CA certs", func() {
	Context("When transforming a KEDA operator Deployment", func() {

		scheme := runtime.NewScheme()
		_ = appsv1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)

		logger := ctrl.Log.WithName("test")

		yamlData := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: keda-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: keda-operator
  template:
    metadata:
      labels:
        app: keda-operator
    spec:
      containers:
        - name: keda-operator
          image: keda:latest
          volumeMounts:
            - name: example-volume
              mountPath: /example
      volumes:
        - name: example-volume
          configMap:
            name: example`

		It("Should replace stale CA bundle args/volumes/mounts when the config map list changes", func() {
			manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
			Expect(err).To(BeNil())

			By("Reconciling with a single config map")
			firstTransforms := transform.EnsureCACertsForOperatorDeployment([]string{"corporate-ca-0"}, scheme, logger)
			manifest, err = manifest.Transform(firstTransforms...)
			Expect(err).To(BeNil())

			By("Reconciling again with two different config maps")
			secondTransforms := transform.EnsureCACertsForOperatorDeployment([]string{"corporate-ca-1", "corporate-ca-2"}, scheme, logger)
			newManifest, err := manifest.Transform(secondTransforms...)
			Expect(err).To(BeNil())

			r := newManifest.Resources()
			Expect(len(r)).To(Equal(1))

			By("Checking only the new CA bundle volumes are present")
			volumes, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "volumes")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			expectCABundleVolumes(volumes)

			By("Checking the operator container has the new --ca-dir args and the CA bundle mounts")
			containers, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "containers")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())

			args, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "args")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(args).To(ConsistOf("--ca-dir=/custom/ca0", "--ca-dir=/custom/ca1"))

			volumeMounts, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "volumeMounts")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			expectCABundleVolumeMounts(volumeMounts)
		})
	})
})

var _ = Describe("Transforming interceptor deployment for CA certs", func() {
	Context("When transforming an HTTP Add-on interceptor Deployment", func() {

		scheme := runtime.NewScheme()
		_ = appsv1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)

		yamlData := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: keda-add-ons-http-interceptor
spec:
  replicas: 1
  selector:
    matchLabels:
      app: keda-add-ons-http-interceptor
  template:
    metadata:
      labels:
        app: keda-add-ons-http-interceptor
    spec:
      containers:
        - name: interceptor
          image: interceptor:latest
          env:
            - name: KEDA_HTTP_PROXY_PORT
              value: "8080"
          volumeMounts:
            - name: example-volume
              mountPath: /example
      volumes:
        - name: example-volume
          configMap:
            name: example`

		It("Should replace stale CA bundle volumes, mounts, and env var when the config map list changes", func() {
			manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
			Expect(err).To(BeNil())

			By("Reconciling with a single config map")
			firstTransforms := transform.EnsureCACertsForInterceptorDeployment([]string{"corporate-ca-0"}, "interceptor", scheme)
			manifest, err = manifest.Transform(firstTransforms...)
			Expect(err).To(BeNil())

			By("Reconciling again with two different config maps")
			secondTransforms := transform.EnsureCACertsForInterceptorDeployment([]string{"corporate-ca-1", "corporate-ca-2"}, "interceptor", scheme)
			newManifest, err := manifest.Transform(secondTransforms...)
			Expect(err).To(BeNil())

			r := newManifest.Resources()
			Expect(len(r)).To(Equal(1))

			By("Checking only the new CA bundle volumes are present")
			volumes, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "volumes")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			expectCABundleVolumes(volumes)

			By("Checking the interceptor container has the new CA bundle mounts and env var")
			containers, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "containers")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())

			volumeMounts, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "volumeMounts")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			expectCABundleVolumeMounts(volumeMounts)

			env, found, err := unstructured.NestedSlice(structuredToMap(containers[0]), "env")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(env).To(ContainElement(structuredToMap(corev1.EnvVar{Name: "KEDA_HTTP_TLS_CA_DIRS", Value: "/custom/ca0,/custom/ca1"})))
			Expect(env).To(ContainElement(structuredToMap(corev1.EnvVar{Name: "KEDA_HTTP_PROXY_PORT", Value: "8080"})))
		})
	})
})

var _ = Describe("ReplaceContainerEnv", func() {
	BeforeEach(func() {
		if testType != "unit" {
			Skip("test.type isn't 'unit'")
		}
	})

	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	yamlData := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: keda-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: keda-operator
  template:
    metadata:
      labels:
        app: keda-operator
    spec:
      containers:
        - name: keda-operator
          image: keda:latest
          env:
            - name: WATCH_NAMESPACE
              value: keda
            - name: KEDA_HTTP_DEFAULT_TIMEOUT
              value: "3000"
        - name: sidecar
          image: sidecar:latest
          env:
            - name: KEDA_HTTP_DEFAULT_TIMEOUT
              value: "3000"`

	// containerEnv returns the env slice of the named container in the first resource of the manifest.
	containerEnv := func(manifest mf.Manifest, containerName string) []interface{} {
		r := manifest.Resources()
		Expect(r).To(HaveLen(1))

		containers, found, err := unstructured.NestedSlice(r[0].UnstructuredContent(), "spec", "template", "spec", "containers")
		Expect(found).To(BeTrue())
		Expect(err).To(BeNil())

		for _, c := range containers {
			name, found, err := unstructured.NestedString(c.(map[string]interface{}), "name")
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			if name == containerName {
				env, found, err := unstructured.NestedSlice(c.(map[string]interface{}), "env")
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
				return env
			}
		}

		Fail("Could not find a container named " + containerName)
		return nil
	}

	secretRef := corev1.EnvVar{
		Name: "KEDA_HTTP_MIN_TLS_VERSION",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tls-config"},
				Key:                  "minTLSVersion",
			},
		},
	}

	It("Should overwrite matching variables in place, append new ones, and preserve the rest", func() {
		manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
		Expect(err).To(BeNil())

		newManifest, err := manifest.Transform(transform.ReplaceContainerEnv(
			[]corev1.EnvVar{
				{Name: "KEDA_HTTP_DEFAULT_TIMEOUT", Value: "10000"},
				secretRef,
			}, "keda-operator", scheme))
		Expect(err).To(BeNil())

		env := containerEnv(newManifest, "keda-operator")
		Expect(env).To(HaveLen(3))

		By("Checking the untouched variable kept its position and value")
		Expect(env[0]).To(Equal(structuredToMap(corev1.EnvVar{Name: "WATCH_NAMESPACE", Value: "keda"})))

		By("Checking the matching variable was overwritten in place rather than appended")
		Expect(env[1]).To(Equal(structuredToMap(corev1.EnvVar{Name: "KEDA_HTTP_DEFAULT_TIMEOUT", Value: "10000"})))

		By("Checking the new valueFrom variable was appended")
		Expect(env[2]).To(Equal(structuredToMap(secretRef)))
	})

	It("Should leave containers with a different name untouched", func() {
		manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(yamlData)))
		Expect(err).To(BeNil())

		newManifest, err := manifest.Transform(transform.ReplaceContainerEnv(
			[]corev1.EnvVar{{Name: "KEDA_HTTP_DEFAULT_TIMEOUT", Value: "10000"}}, "keda-operator", scheme))
		Expect(err).To(BeNil())

		env := containerEnv(newManifest, "sidecar")
		Expect(env).To(HaveLen(1))
		Expect(env[0]).To(Equal(structuredToMap(corev1.EnvVar{Name: "KEDA_HTTP_DEFAULT_TIMEOUT", Value: "3000"})))
	})
})

var _ = Describe("EnsureCertSecretVolume", func() {
	BeforeEach(func() {
		if testType != "unit" {
			Skip("test.type isn't 'unit'")
		}
	})

	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	deployYAML := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: main
          image: main:latest
        - name: sidecar
          image: sidecar:latest`

	It("should add volume and mount to the target container and update idempotently", func() {
		manifest, err := mf.ManifestFrom(mf.Reader(strings.NewReader(deployYAML)))
		Expect(err).To(BeNil())

		newManifest, err := manifest.Transform(
			transform.EnsureCertSecretVolume("main", "my-certs-secret", scheme),
		)
		Expect(err).To(BeNil())

		deploy := &appsv1.Deployment{}
		Expect(scheme.Convert(&newManifest.Resources()[0], deploy, nil)).To(Succeed())

		Expect(deploy.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
			Name: "main-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "my-certs-secret"},
			},
		}))

		main := deploy.Spec.Template.Spec.Containers[0]
		Expect(main.VolumeMounts).To(ContainElement(corev1.VolumeMount{
			Name: "main-certs", MountPath: "/certs", ReadOnly: true,
		}))

		sidecar := deploy.Spec.Template.Spec.Containers[1]
		Expect(sidecar.VolumeMounts).To(BeEmpty())

		// Apply again with a different secret — should replace, not duplicate.
		updatedManifest, err := newManifest.Transform(
			transform.EnsureCertSecretVolume("main", "updated-secret", scheme),
		)
		Expect(err).To(BeNil())

		updatedDeploy := &appsv1.Deployment{}
		Expect(scheme.Convert(&updatedManifest.Resources()[0], updatedDeploy, nil)).To(Succeed())

		Expect(updatedDeploy.Spec.Template.Spec.Volumes).To(HaveLen(1))
		Expect(updatedDeploy.Spec.Template.Spec.Volumes[0]).To(Equal(corev1.Volume{
			Name: "main-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "updated-secret"},
			},
		}))

		updatedMain := updatedDeploy.Spec.Template.Spec.Containers[0]
		Expect(updatedMain.VolumeMounts).To(HaveLen(1))
		Expect(updatedMain.VolumeMounts[0]).To(Equal(corev1.VolumeMount{
			Name: "main-certs", MountPath: "/certs", ReadOnly: true,
		}))
	})
})

// structuredToMap converts a strongly typed volume object to unstructured so we can do a
// containElement comparison against the unstructured object that comes back from unstructured.NestedSlice
// and have them actually match
func structuredToMap(thing interface{}) map[string]interface{} {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&thing)
	if err != nil {
		panic(err)
	}
	return objMap
}

// expectCABundleVolumes asserts that volumes contains only the cabundle0/cabundle1 volumes for
// corporate-ca-1/corporate-ca-2 and the pre-existing example-volume.
func expectCABundleVolumes(volumes []interface{}) {
	Expect(volumes).To(HaveLen(3), "example-volume + 2 new cabundle volumes, stale one replaced")
	Expect(volumes).To(ContainElement(structuredToMap(corev1.Volume{
		Name:         "cabundle0",
		VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "corporate-ca-1"}}},
	})))
	Expect(volumes).To(ContainElement(structuredToMap(corev1.Volume{
		Name:         "cabundle1",
		VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "corporate-ca-2"}}},
	})))
	Expect(volumes).To(ContainElement(structuredToMap(corev1.Volume{
		Name:         "example-volume",
		VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "example"}}},
	})))
}

// expectCABundleVolumeMounts asserts that volumeMounts contains only the cabundle0/cabundle1 mounts
// and the pre-existing example-volume mount.
func expectCABundleVolumeMounts(volumeMounts []interface{}) {
	Expect(volumeMounts).To(HaveLen(3))
	Expect(volumeMounts).To(ContainElement(structuredToMap(corev1.VolumeMount{Name: "cabundle0", MountPath: "/custom/ca0"})))
	Expect(volumeMounts).To(ContainElement(structuredToMap(corev1.VolumeMount{Name: "cabundle1", MountPath: "/custom/ca1"})))
	Expect(volumeMounts).To(ContainElement(structuredToMap(corev1.VolumeMount{Name: "example-volume", MountPath: "/example"})))
}
