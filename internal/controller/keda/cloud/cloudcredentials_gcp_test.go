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
	"os"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func setGCPEnv(env map[string]string) {
	for k, v := range env {
		os.Setenv(k, v)
	}
}

func clearGCPEnv() {
	for _, k := range []string{"SERVICE_ACCOUNT_EMAIL", "AUDIENCE", "PROJECT_NUMBER", "POOL_ID", "PROVIDER_ID", "CLOUDSDK_CORE_PROJECT"} {
		os.Unsetenv(k)
	}
}

var _ = Describe("GCP WIF integration", func() {

	BeforeEach(func() {
		clearGCPEnv()
	})

	AfterEach(func() {
		clearGCPEnv()
	})

	Context("Enabled()", func() {
		variants := []struct {
			name string
			env  map[string]string
			want bool
		}{
			{
				name: "no env vars",
				env:  map[string]string{},
				want: false,
			},
			{
				name: "SERVICE_ACCOUNT_EMAIL only",
				env:  map[string]string{"SERVICE_ACCOUNT_EMAIL": "sa@proj.iam.gserviceaccount.com"},
				want: false,
			},
			{
				name: "AUDIENCE + SERVICE_ACCOUNT_EMAIL",
				env: map[string]string{
					"AUDIENCE":              "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
					"SERVICE_ACCOUNT_EMAIL": "sa@proj.iam.gserviceaccount.com",
				},
				want: true,
			},
			{
				name: "component env vars (console install)",
				env: map[string]string{
					"PROJECT_NUMBER":        "123456",
					"POOL_ID":               "my-pool",
					"PROVIDER_ID":           "my-provider",
					"SERVICE_ACCOUNT_EMAIL": "sa@proj.iam.gserviceaccount.com",
				},
				want: true,
			},
			{
				name: "partial component env vars missing PROVIDER_ID",
				env: map[string]string{
					"PROJECT_NUMBER":        "123456",
					"POOL_ID":               "my-pool",
					"SERVICE_ACCOUNT_EMAIL": "sa@proj.iam.gserviceaccount.com",
				},
				want: false,
			},
			{
				name: "AUDIENCE without SERVICE_ACCOUNT_EMAIL",
				env: map[string]string{
					"AUDIENCE": "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
				},
				want: false,
			},
		}

		for _, tt := range variants {
			It("Should return "+strconv.FormatBool(tt.want)+" when "+tt.name, func() {
				setGCPEnv(tt.env)
				p := &gcpProvider{}
				Expect(p.Enabled()).To(Equal(tt.want))
			})
		}
	})

	Context("audience()", func() {
		variants := []struct {
			name string
			env  map[string]string
			want string
		}{
			{
				name: "AUDIENCE env var takes precedence",
				env: map[string]string{
					"AUDIENCE":       "//iam.googleapis.com/projects/999/locations/global/workloadIdentityPools/p/providers/pr",
					"PROJECT_NUMBER": "123",
					"POOL_ID":        "pool",
					"PROVIDER_ID":    "prov",
				},
				want: "//iam.googleapis.com/projects/999/locations/global/workloadIdentityPools/p/providers/pr",
			},
			{
				name: "constructed from components",
				env: map[string]string{
					"PROJECT_NUMBER": "123456",
					"POOL_ID":        "my-pool",
					"PROVIDER_ID":    "my-provider",
				},
				want: "//iam.googleapis.com/projects/123456/locations/global/workloadIdentityPools/my-pool/providers/my-provider",
			},
		}

		for _, tt := range variants {
			It("Should resolve correctly when "+tt.name, func() {
				setGCPEnv(tt.env)
				p := &gcpProvider{}
				Expect(p.audience()).To(Equal(tt.want))
			})
		}
	})

	Context("BuildCredentialSecret()", func() {
		It("Should produce valid external_account JSON with AUDIENCE env var", func() {
			setGCPEnv(map[string]string{
				"AUDIENCE":              "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
				"SERVICE_ACCOUNT_EMAIL": "sa@proj.iam.gserviceaccount.com",
			})

			p := &gcpProvider{}
			data, err := p.BuildCredentialSecret()
			Expect(err).NotTo(HaveOccurred())

			raw, ok := data[gcpSecretKey]
			Expect(ok).To(BeTrue(), "missing key %q in secret data", gcpSecretKey)

			var cred gcpExternalAccountCredential
			Expect(json.Unmarshal(raw, &cred)).To(Succeed())

			Expect(cred.Type).To(Equal("external_account"))
			Expect(cred.Audience).To(Equal("//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/prov"))
			Expect(cred.TokenURL).To(Equal(gcpSTSTokenURL))
			Expect(cred.CredentialSource.File).To(Equal(CloudTokenPath))

			wantImpersonation := "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/sa@proj.iam.gserviceaccount.com:generateAccessToken"
			Expect(cred.ServiceAccountImpersonationURL).To(Equal(wantImpersonation))
		})

		It("Should use component env vars when AUDIENCE is absent", func() {
			setGCPEnv(map[string]string{
				"PROJECT_NUMBER":        "999",
				"POOL_ID":               "p",
				"PROVIDER_ID":           "pr",
				"SERVICE_ACCOUNT_EMAIL": "sa@proj.iam.gserviceaccount.com",
			})

			p := &gcpProvider{}
			data, err := p.BuildCredentialSecret()
			Expect(err).NotTo(HaveOccurred())

			var cred gcpExternalAccountCredential
			Expect(json.Unmarshal(data[gcpSecretKey], &cred)).To(Succeed())

			wantAudience := "//iam.googleapis.com/projects/999/locations/global/workloadIdentityPools/p/providers/pr"
			Expect(cred.Audience).To(Equal(wantAudience))
		})
	})

	Context("SecretName()", func() {
		It("Should return the expected GCP Secret name", func() {
			p := &gcpProvider{}
			Expect(p.SecretName()).To(Equal(gcpSecretName))
		})
	})

	Context("resolveProjectID()", func() {
		It("Should prefer CLOUDSDK_CORE_PROJECT env var over Infrastructure CR", func() {
			os.Setenv("CLOUDSDK_CORE_PROJECT", "env-project")

			p := &gcpProvider{}
			id := p.resolveProjectID(context.Background(), fake.NewClientBuilder().Build())
			Expect(id).To(Equal("env-project"))
		})

		It("Should fall back to Infrastructure CR when CLOUDSDK_CORE_PROJECT is not set", func() {
			s := newTestScheme()
			infra := &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Status: configv1.InfrastructureStatus{
					PlatformStatus: &configv1.PlatformStatus{
						GCP: &configv1.GCPPlatformStatus{ProjectID: "infra-project"},
					},
				},
			}
			c := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(infra).WithObjects(infra).Build()

			p := &gcpProvider{}
			id := p.resolveProjectID(context.Background(), c)
			Expect(id).To(Equal("infra-project"))
		})

		It("Should return empty when neither source provides a project ID", func() {
			p := &gcpProvider{}
			id := p.resolveProjectID(context.Background(), fake.NewClientBuilder().Build())
			Expect(id).To(BeEmpty())
		})
	})
})
