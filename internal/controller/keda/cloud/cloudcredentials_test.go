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

package cloud

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	Expect(corev1.AddToScheme(s)).To(Succeed())
	Expect(kedav1alpha1.AddToScheme(s)).To(Succeed())
	Expect(configv1.Install(s)).To(Succeed())
	return s
}

var _ = Describe("Cloud credential provider registry", func() {
	Context("Providers()", func() {
		It("Should return at least the GCP credential provider", func() {
			providers := Providers()
			Expect(providers).NotTo(BeEmpty())
			Expect(providers[0].Name()).To(Equal("GCP WIF"))
		})
	})
})

var _ = Describe("Bound SA token helpers", func() {
	Context("BoundSATokenVolume", func() {
		It("Should return a projected volume with correct audience and path", func() {
			vol := BoundSATokenVolume()
			Expect(vol.Name).To(Equal("bound-sa-token"))
			Expect(vol.Projected).NotTo(BeNil())
			Expect(vol.Projected.Sources).To(HaveLen(1))

			sat := vol.Projected.Sources[0].ServiceAccountToken
			Expect(sat).NotTo(BeNil())
			Expect(sat.Audience).To(Equal("openshift"))
			Expect(sat.Path).To(Equal("token"))
		})
	})

	Context("BoundSATokenVolumeMount", func() {
		It("Should return a read-only mount at the bound SA token path", func() {
			mount := BoundSATokenVolumeMount()
			Expect(mount.Name).To(Equal("bound-sa-token"))
			Expect(mount.MountPath).To(Equal(BoundSATokenPath))
			Expect(mount.ReadOnly).To(BeTrue())
		})
	})

	Context("Volume and mount compatibility", func() {
		It("Should have matching names between volume and mount", func() {
			vol := BoundSATokenVolume()
			mount := BoundSATokenVolumeMount()
			Expect(vol.Name).To(Equal(mount.Name))
		})
	})
})

var _ = Describe("Credential Secret volume helpers", func() {
	Context("CredentialSecretVolume", func() {
		It("Should return a Secret-backed volume", func() {
			vol := CredentialSecretVolume("my-vol", "my-secret")
			Expect(vol.Name).To(Equal("my-vol"))
			Expect(vol.Secret).NotTo(BeNil())
			Expect(vol.Secret.SecretName).To(Equal("my-secret"))
		})
	})

	Context("When using GCP credential constants", func() {
		It("Should produce a volume and mount with matching names", func() {
			vol := CredentialSecretVolume(gcpCredVolumeName, gcpSecretName)
			mount := corev1.VolumeMount{
				Name:      gcpCredVolumeName,
				MountPath: gcpCredMountPath,
				ReadOnly:  true,
			}
			Expect(vol.Name).To(Equal(mount.Name))
			Expect(vol.Secret.SecretName).To(Equal(gcpSecretName))
		})
	})
})

var _ = Describe("EnsureCredentialSecret", func() {
	var (
		scheme *runtime.Scheme
		owner  *kedav1alpha1.KedaController
		p      *gcpProvider
	)

	BeforeEach(func() {
		scheme = newTestScheme()
		owner = &kedav1alpha1.KedaController{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "keda",
				Namespace: "keda",
				UID:       types.UID("test-uid"),
			},
		}
		p = &gcpProvider{}
	})

	Context("When the Secret does not exist", func() {
		BeforeEach(func() {
			setGCPEnv(map[string]string{
				"SERVICE_ACCOUNT_EMAIL": "sa@proj.iam.gserviceaccount.com",
				"AUDIENCE":              "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
			})
		})

		It("Should create the Secret with correct labels, owner reference, and credential data", func() {
			c := fake.NewClientBuilder().WithScheme(scheme).Build()

			result, err := EnsureCredentialSecret(context.Background(), c, p, "keda", owner, scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(controllerutil.OperationResultCreated))

			secret := &corev1.Secret{}
			Expect(c.Get(context.Background(), types.NamespacedName{Name: gcpSecretName, Namespace: "keda"}, secret)).To(Succeed())

			Expect(secret.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "keda-operator"))

			Expect(secret.OwnerReferences).To(HaveLen(1))
			Expect(secret.OwnerReferences[0].UID).To(Equal(owner.UID))

			raw, ok := secret.Data[gcpSecretKey]
			Expect(ok).To(BeTrue(), "expected key %q in Secret data", gcpSecretKey)

			var cred gcpExternalAccountCredential
			Expect(json.Unmarshal(raw, &cred)).To(Succeed())
			Expect(cred.Type).To(Equal("external_account"))
			Expect(cred.Audience).To(ContainSubstring("workloadIdentityPools"))
		})
	})

	Context("When the Secret already exists with stale data", func() {
		BeforeEach(func() {
			setGCPEnv(map[string]string{
				"SERVICE_ACCOUNT_EMAIL": "sa@proj.iam.gserviceaccount.com",
				"AUDIENCE":              "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
			})
		})

		It("Should update the Secret and replace the stale data", func() {
			existing := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gcpSecretName,
					Namespace: "keda",
				},
				Data: map[string][]byte{"stale": []byte("data")},
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

			result, err := EnsureCredentialSecret(context.Background(), c, p, "keda", owner, scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(controllerutil.OperationResultUpdated))

			secret := &corev1.Secret{}
			Expect(c.Get(context.Background(), types.NamespacedName{Name: gcpSecretName, Namespace: "keda"}, secret)).To(Succeed())

			Expect(secret.Data).NotTo(HaveKey("stale"))
			Expect(secret.Data).To(HaveKey(gcpSecretKey))
		})
	})
})
