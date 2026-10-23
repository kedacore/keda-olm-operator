/*
Copyright 2026 The KEDA Authors

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
	"slices"
	"testing"

	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kedacore/keda-olm-operator/resources"
)

func newResource(kind, name string) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetKind(kind)
	u.SetName(name)
	return u
}

func resourceNames(objs []unstructured.Unstructured) []string {
	names := make([]string, 0, len(objs))
	for _, r := range objs {
		names = append(names, r.GetKind()+"/"+r.GetName())
	}
	return names
}

func TestParseManifestsFromFileKeepsEveryResource(t *testing.T) {
	manifest, err := resources.GetResourcesManifest()
	if err != nil {
		t.Fatal(err)
	}
	general, controller, metrics, webhooks, monitoring, err := parseManifestsFromFile(manifest, nil)
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]int{}
	for _, m := range []mf.Manifest{general, controller, metrics, webhooks, monitoring} {
		for _, r := range m.Resources() {
			got[r.GetKind()+"/"+r.GetNamespace()+"/"+r.GetName()]++
		}
	}

	for _, r := range manifest.Resources() {
		// OLM installs the CRDs from the bundle and the namespace is a prerequisite of the install
		if kind := r.GetKind(); kind == "CustomResourceDefinition" || kind == "Namespace" {
			continue
		}
		key := r.GetKind() + "/" + r.GetNamespace() + "/" + r.GetName()
		if got[key] != 1 {
			t.Errorf("%s is in %d operand manifests, want 1", key, got[key])
		}
		delete(got, key)
	}
	for key := range got {
		t.Errorf("%s is in an operand manifest but not in the embedded resources", key)
	}
}

func TestParseManifestsFromFileAggregatedClusterRoles(t *testing.T) {
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{
		newResource("ClusterRole", "keda-external-metrics-reader"),
		newResource("ClusterRole", "keda-operator"),
		newResource("ClusterRole", "keda:edit"),
		newResource("ClusterRole", "keda:view"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	general, controller, metrics, webhooks, monitoring, err := parseManifestsFromFile(manifest, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		group    string
		manifest mf.Manifest
		want     []string
	}{
		{"general", general, nil},
		{"controller", controller, []string{"ClusterRole/keda-operator", "ClusterRole/keda:edit", "ClusterRole/keda:view"}},
		{"metrics", metrics, []string{"ClusterRole/keda-external-metrics-reader"}},
		{"webhooks", webhooks, nil},
		{"monitoring", monitoring, nil},
	} {
		if got := resourceNames(tt.manifest.Resources()); !slices.Equal(got, tt.want) {
			t.Errorf("%s resources = %v, want %v", tt.group, got, tt.want)
		}
	}
}

func TestSortMetricsResources(t *testing.T) {
	in := []unstructured.Unstructured{
		newResource("APIService", "v1beta1.external.metrics.k8s.io"),
		newResource("Deployment", "keda-metrics-apiserver"),
		newResource("ClusterRoleBinding", "keda-system-auth-delegator"),
		newResource("ClusterRole", "keda-external-metrics-reader"),
		newResource("Service", "keda-metrics-apiserver"),
		newResource("ClusterRoleBinding", "keda-hpa-controller-external-metrics"),
		newResource("RoleBinding", "keda-auth-reader"),
		newResource("ClusterRole", "another-metrics-reader"),
	}
	want := []string{
		"ClusterRole/keda-external-metrics-reader",
		"ClusterRole/another-metrics-reader",
		"RoleBinding/keda-auth-reader",
		"ClusterRoleBinding/keda-hpa-controller-external-metrics",
		"ClusterRoleBinding/keda-system-auth-delegator",
		"Service/keda-metrics-apiserver",
		"Deployment/keda-metrics-apiserver",
		"APIService/v1beta1.external.metrics.k8s.io",
	}

	if got := resourceNames(sortMetricsResources(&in)); !slices.Equal(got, want) {
		t.Errorf("sortMetricsResources() = %v, want %v", got, want)
	}
}
