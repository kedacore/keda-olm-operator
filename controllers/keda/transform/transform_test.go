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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kedacore/keda-olm-operator/controllers/keda/transform"
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
