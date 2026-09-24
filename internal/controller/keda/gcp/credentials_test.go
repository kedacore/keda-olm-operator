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

package gcp

import (
	"encoding/json"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func testConfig() *Config {
	return &Config{
		Audience:             testAudience,
		ServiceAccountEmail:  testEmail,
		SubjectTokenAudience: DefaultSubjectTokenAudience,
	}
}

func TestCredentialsJSON(t *testing.T) {
	tests := []struct {
		name           string
		universeDomain string
		wantTokenURL   string
		wantImpersURL  string
		wantUniverse   string // "" means the field must be absent
	}{
		{
			name:           "public cloud when no universe domain is known",
			universeDomain: "",
			wantTokenURL:   "https://sts.googleapis.com/v1/token",
			wantImpersURL:  "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" + testEmail + ":generateAccessToken",
		},
		{
			name:           "public cloud spelled out",
			universeDomain: DefaultUniverseDomain,
			wantTokenURL:   "https://sts.googleapis.com/v1/token",
			wantImpersURL:  "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" + testEmail + ":generateAccessToken",
		},
		{
			name:           "trusted partner cloud universe",
			universeDomain: "apis-tpclp.goog",
			wantTokenURL:   "https://sts.apis-tpclp.goog/v1/token",
			wantImpersURL:  "https://iamcredentials.apis-tpclp.goog/v1/projects/-/serviceAccounts/" + testEmail + ":generateAccessToken",
			wantUniverse:   "apis-tpclp.goog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := testConfig().CredentialsJSON(tt.universeDomain)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("credentials are not valid JSON: %v\n%s", err, data)
			}

			expect := map[string]any{
				"type":                              "external_account",
				"audience":                          testAudience,
				"subject_token_type":                "urn:ietf:params:oauth:token-type:jwt",
				"token_url":                         tt.wantTokenURL,
				"service_account_impersonation_url": tt.wantImpersURL,
			}
			for k, want := range expect {
				if got[k] != want {
					t.Errorf("%s = %v, want %v", k, got[k], want)
				}
			}

			source, ok := got["credential_source"].(map[string]any)
			if !ok {
				t.Fatalf("credential_source missing or malformed: %v", got["credential_source"])
			}
			if source["file"] != SubjectTokenFilePath {
				t.Errorf("credential_source.file = %v, want %v", source["file"], SubjectTokenFilePath)
			}
			if format, _ := source["format"].(map[string]any); format["type"] != "text" {
				t.Errorf("credential_source.format = %v, want type text", source["format"])
			}

			universe, present := got["universe_domain"]
			if tt.wantUniverse == "" && present {
				t.Errorf("universe_domain should be absent for the public cloud, got %v", universe)
			}
			if tt.wantUniverse != "" && universe != tt.wantUniverse {
				t.Errorf("universe_domain = %v, want %v", universe, tt.wantUniverse)
			}
		})
	}
}

func TestCredentialsHash(t *testing.T) {
	a, err := testConfig().CredentialsJSON("")
	if err != nil {
		t.Fatal(err)
	}
	same, err := testConfig().CredentialsJSON("")
	if err != nil {
		t.Fatal(err)
	}
	other := testConfig()
	other.ServiceAccountEmail = "someone-else@my-project.iam.gserviceaccount.com"
	b, err := other.CredentialsJSON("")
	if err != nil {
		t.Fatal(err)
	}

	if CredentialsHash(a) != CredentialsHash(same) {
		t.Error("hash is not deterministic")
	}
	if CredentialsHash(a) == CredentialsHash(b) {
		t.Error("hash does not change with the credentials")
	}
	if len(CredentialsHash(a)) != 64 {
		t.Errorf("expected a hex encoded sha256, got %q", CredentialsHash(a))
	}
}

func TestPlatformFromInfrastructure(t *testing.T) {
	tests := []struct {
		name         string
		infra        *configv1.Infrastructure
		wantProject  string
		wantUniverse string
	}{
		{name: "nil"},
		{name: "no platform status", infra: &configv1.Infrastructure{}},
		{
			name: "not on GCP",
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				PlatformStatus: &configv1.PlatformStatus{Type: configv1.AWSPlatformType, AWS: &configv1.AWSPlatformStatus{Region: "us-east-1"}},
			}},
		},
		{
			name: "on GCP",
			infra: &configv1.Infrastructure{Status: configv1.InfrastructureStatus{
				PlatformStatus: &configv1.PlatformStatus{Type: configv1.GCPPlatformType, GCP: &configv1.GCPPlatformStatus{ProjectID: "my-project", UniverseDomain: "apis-tpclp.goog"}},
			}},
			wantProject:  "my-project",
			wantUniverse: "apis-tpclp.goog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, universe := PlatformFromInfrastructure(tt.infra)
			if project != tt.wantProject || universe != tt.wantUniverse {
				t.Errorf("got (%q, %q), want (%q, %q)", project, universe, tt.wantProject, tt.wantUniverse)
			}
		})
	}
}

// kedaOperatorDeployment mimics the relevant parts of the keda-operator Deployment
// from the operand manifest: a keda-operator container with pre-existing volumes,
// mounts and env, next to a sidecar that must not be touched.
func kedaOperatorDeployment(t *testing.T, scheme *runtime.Scheme) *unstructured.Unstructured {
	t.Helper()
	deploy := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "keda-operator", Namespace: "keda"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"existing": "annotation"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         "keda-operator",
							Env:          []corev1.EnvVar{{Name: "WATCH_NAMESPACE", Value: ""}},
							VolumeMounts: []corev1.VolumeMount{{Name: "certificates", MountPath: "/certs"}},
						},
						{Name: "sidecar"},
					},
					Volumes: []corev1.Volume{{Name: "certificates"}},
				},
			},
		},
	}
	u := &unstructured.Unstructured{}
	if err := scheme.Convert(deploy, u, nil); err != nil {
		t.Fatal(err)
	}
	return u
}

func applyTransforms(t *testing.T, scheme *runtime.Scheme, u *unstructured.Unstructured, cfg *Config, projectID, hash string) *appsv1.Deployment {
	t.Helper()
	for _, tr := range DeploymentTransforms(cfg, projectID, hash, scheme) {
		if err := tr(u); err != nil {
			t.Fatal(err)
		}
	}
	deploy := &appsv1.Deployment{}
	if err := scheme.Convert(u, deploy, nil); err != nil {
		t.Fatal(err)
	}
	return deploy
}

func findContainer(t *testing.T, deploy *appsv1.Deployment, name string) *corev1.Container {
	t.Helper()
	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == name {
			return &deploy.Spec.Template.Spec.Containers[i]
		}
	}
	t.Fatalf("container %s not found", name)
	return nil
}

func envValue(env []corev1.EnvVar, name string) (string, bool) {
	for _, e := range env {
		if e.Name == name {
			return e.Value, true
		}
	}
	return "", false
}

func TestDeploymentTransforms(t *testing.T) {
	scheme := clientgoscheme.Scheme
	cfg := testConfig()
	cfg.SubjectTokenAudience = "custom-audience"

	deploy := applyTransforms(t, scheme, kedaOperatorDeployment(t, scheme), cfg, "my-project", "abc123")

	// volumes: existing ones are kept, the token and the Secret are added
	volumes := map[string]corev1.Volume{}
	for _, v := range deploy.Spec.Template.Spec.Volumes {
		volumes[v.Name] = v
	}
	if len(volumes) != 3 {
		t.Errorf("expected 3 volumes, got %v", deploy.Spec.Template.Spec.Volumes)
	}
	token, ok := volumes[boundSATokenVolumeName]
	if !ok || token.Projected == nil || len(token.Projected.Sources) != 1 || token.Projected.Sources[0].ServiceAccountToken == nil {
		t.Fatalf("projected token volume missing or malformed: %+v", token)
	}
	if sat := token.Projected.Sources[0].ServiceAccountToken; sat.Audience != "custom-audience" || sat.Path != boundSATokenFileName {
		t.Errorf("unexpected service account token projection %+v", sat)
	}
	if creds, ok := volumes[credentialsVolumeName]; !ok || creds.Secret == nil || creds.Secret.SecretName != CredentialsSecretName {
		t.Errorf("credentials Secret volume missing or malformed: %+v", creds)
	}

	// keda-operator container: mounts and env added to what was there
	operator := findContainer(t, deploy, "keda-operator")
	mounts := map[string]corev1.VolumeMount{}
	for _, m := range operator.VolumeMounts {
		mounts[m.Name] = m
	}
	if len(mounts) != 3 {
		t.Errorf("expected 3 volume mounts, got %v", operator.VolumeMounts)
	}
	if m := mounts[boundSATokenVolumeName]; m.MountPath != boundSATokenMountPath || !m.ReadOnly {
		t.Errorf("unexpected token mount %+v", m)
	}
	if m := mounts[credentialsVolumeName]; m.MountPath != credentialsMountPath || !m.ReadOnly {
		t.Errorf("unexpected credentials mount %+v", m)
	}
	if v, ok := envValue(operator.Env, envGoogleApplicationCredentials); !ok || v != CredentialsFilePath {
		t.Errorf("%s = %q, %v; want %q", envGoogleApplicationCredentials, v, ok, CredentialsFilePath)
	}
	if v, ok := envValue(operator.Env, envCloudSDKCoreProject); !ok || v != "my-project" {
		t.Errorf("%s = %q, %v; want my-project", envCloudSDKCoreProject, v, ok)
	}
	if _, ok := envValue(operator.Env, "WATCH_NAMESPACE"); !ok {
		t.Error("pre-existing env var was dropped")
	}

	// the sidecar is left alone
	sidecar := findContainer(t, deploy, "sidecar")
	if len(sidecar.VolumeMounts) != 0 || len(sidecar.Env) != 0 {
		t.Errorf("sidecar must not be modified, got %+v", sidecar)
	}

	// pod template annotation
	annotations := deploy.Spec.Template.Annotations
	if annotations[CredentialsHashAnnotation] != "abc123" || annotations["existing"] != "annotation" {
		t.Errorf("unexpected pod template annotations %v", annotations)
	}
}

func TestDeploymentTransformsWithoutProject(t *testing.T) {
	scheme := clientgoscheme.Scheme
	deploy := applyTransforms(t, scheme, kedaOperatorDeployment(t, scheme), testConfig(), "", "abc123")

	operator := findContainer(t, deploy, "keda-operator")
	if _, ok := envValue(operator.Env, envCloudSDKCoreProject); ok {
		t.Errorf("%s must not be set when the project is unknown", envCloudSDKCoreProject)
	}
	if _, ok := envValue(operator.Env, envGoogleApplicationCredentials); !ok {
		t.Errorf("%s must be set regardless of the project", envGoogleApplicationCredentials)
	}
}

func TestDeploymentTransformsAreIdempotent(t *testing.T) {
	scheme := clientgoscheme.Scheme
	u := kedaOperatorDeployment(t, scheme)
	applyTransforms(t, scheme, u, testConfig(), "my-project", "first")
	deploy := applyTransforms(t, scheme, u, testConfig(), "other-project", "second")

	if n := len(deploy.Spec.Template.Spec.Volumes); n != 3 {
		t.Errorf("volumes were duplicated: %d", n)
	}
	operator := findContainer(t, deploy, "keda-operator")
	if n := len(operator.VolumeMounts); n != 3 {
		t.Errorf("volume mounts were duplicated: %d", n)
	}
	if n := len(operator.Env); n != 3 {
		t.Errorf("env vars were duplicated: %v", operator.Env)
	}
	if v, _ := envValue(operator.Env, envCloudSDKCoreProject); v != "other-project" {
		t.Errorf("%s was not replaced, got %q", envCloudSDKCoreProject, v)
	}
	if deploy.Spec.Template.Annotations[CredentialsHashAnnotation] != "second" {
		t.Errorf("hash annotation was not replaced: %v", deploy.Spec.Template.Annotations)
	}
}
