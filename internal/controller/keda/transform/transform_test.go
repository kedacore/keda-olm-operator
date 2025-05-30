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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

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

			By("Setting up schemes")
			// Set up the scheme, the transformer uses this convert from unstructured, so we need this
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
		})
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
