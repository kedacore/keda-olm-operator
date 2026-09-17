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
	"reflect"
	"strings"
	"testing"
)

const (
	testEmail    = "keda@my-project.iam.gserviceaccount.com"
	testAudience = "//iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/my-pool/providers/my-provider"
)

func envLookup(env map[string]string) func(string) string {
	return func(name string) string { return env[name] }
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    *Config
		wantErr []string // substrings that have to appear in the error
	}{
		{
			name: "not configured",
			env:  map[string]string{},
			want: nil,
		},
		{
			name: "unrelated variables only",
			env:  map[string]string{EnvProjectID: "some-project", "KEDA_OPERATOR_IMAGE": "img"},
			want: nil,
		},
		{
			name: "CLI install with AUDIENCE",
			env: map[string]string{
				EnvAudience:            testAudience,
				EnvServiceAccountEmail: testEmail,
			},
			want: &Config{
				Audience:             testAudience,
				ServiceAccountEmail:  testEmail,
				SubjectTokenAudience: DefaultSubjectTokenAudience,
			},
		},
		{
			name: "console install with provider components",
			env: map[string]string{
				EnvProjectNumber:       "123456789012",
				EnvPoolID:              "my-pool",
				EnvProviderID:          "my-provider",
				EnvServiceAccountEmail: testEmail,
			},
			want: &Config{
				Audience:             testAudience,
				ServiceAccountEmail:  testEmail,
				SubjectTokenAudience: DefaultSubjectTokenAudience,
			},
		},
		{
			name: "optional project and token audience, surrounding whitespace trimmed",
			env: map[string]string{
				EnvAudience:             " " + testAudience + "\n",
				EnvServiceAccountEmail:  testEmail + " ",
				EnvProjectID:            " other-project ",
				EnvSubjectTokenAudience: "sts.googleapis.com",
			},
			want: &Config{
				Audience:             testAudience,
				ServiceAccountEmail:  testEmail,
				ProjectID:            "other-project",
				SubjectTokenAudience: "sts.googleapis.com",
			},
		},
		{
			name: "AUDIENCE and matching components are accepted",
			env: map[string]string{
				EnvAudience:            testAudience,
				EnvProjectNumber:       "123456789012",
				EnvPoolID:              "my-pool",
				EnvProviderID:          "my-provider",
				EnvServiceAccountEmail: testEmail,
			},
			want: &Config{
				Audience:             testAudience,
				ServiceAccountEmail:  testEmail,
				SubjectTokenAudience: DefaultSubjectTokenAudience,
			},
		},
		{
			name:    "service account email alone",
			env:     map[string]string{EnvServiceAccountEmail: testEmail},
			wantErr: []string{"either AUDIENCE or all of PROJECT_NUMBER, POOL_ID and PROVIDER_ID must be set"},
		},
		{
			name:    "audience alone",
			env:     map[string]string{EnvAudience: testAudience},
			wantErr: []string{"SERVICE_ACCOUNT_EMAIL must be set"},
		},
		{
			name:    "token audience alone",
			env:     map[string]string{EnvSubjectTokenAudience: "openshift"},
			wantErr: []string{"SERVICE_ACCOUNT_EMAIL must be set", "either AUDIENCE or all of"},
		},
		{
			name: "incomplete provider components",
			env: map[string]string{
				EnvProjectNumber:       "123456789012",
				EnvProviderID:          "my-provider",
				EnvServiceAccountEmail: testEmail,
			},
			wantErr: []string{"POOL_ID must be set together with"},
		},
		{
			name: "incomplete provider components next to AUDIENCE",
			env: map[string]string{
				EnvAudience:            testAudience,
				EnvPoolID:              "my-pool",
				EnvServiceAccountEmail: testEmail,
			},
			wantErr: []string{"PROJECT_NUMBER and PROVIDER_ID must be set together with"},
		},
		{
			name: "AUDIENCE contradicting the provider components",
			env: map[string]string{
				EnvAudience:            testAudience,
				EnvProjectNumber:       "123456789012",
				EnvPoolID:              "another-pool",
				EnvProviderID:          "my-provider",
				EnvServiceAccountEmail: testEmail,
			},
			wantErr: []string{"does not match the provider", "another-pool"},
		},
		{
			name: "non-numeric project number",
			env: map[string]string{
				EnvProjectNumber:       "my-project",
				EnvPoolID:              "my-pool",
				EnvProviderID:          "my-provider",
				EnvServiceAccountEmail: testEmail,
			},
			wantErr: []string{`PROJECT_NUMBER "my-project" must be numeric`},
		},
		{
			name: "provider ID with a slash",
			env: map[string]string{
				EnvProjectNumber:       "123456789012",
				EnvPoolID:              "my-pool",
				EnvProviderID:          "providers/my-provider",
				EnvServiceAccountEmail: testEmail,
			},
			wantErr: []string{"PROVIDER_ID", "not allowed"},
		},
		{
			name: "malformed AUDIENCE",
			env: map[string]string{
				EnvAudience:            "projects/123456789012/locations/global/workloadIdentityPools/my-pool/providers/my-provider",
				EnvServiceAccountEmail: testEmail,
			},
			wantErr: []string{"is not a workload identity provider resource name"},
		},
		{
			name: "project number instead of an email",
			env: map[string]string{
				EnvAudience:            testAudience,
				EnvServiceAccountEmail: "123456789012",
			},
			wantErr: []string{`SERVICE_ACCOUNT_EMAIL "123456789012" is not a service account email address`},
		},
		{
			name: "all problems are reported at once",
			env: map[string]string{
				EnvProjectNumber:       "abc",
				EnvPoolID:              "my-pool",
				EnvProviderID:          "my-provider",
				EnvServiceAccountEmail: "not-an-email",
			},
			wantErr: []string{"SERVICE_ACCOUNT_EMAIL", "PROJECT_NUMBER"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfig(envLookup(tt.env))
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatalf("expected an error, got config %+v", got)
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err.Error(), want)
					}
				}
				if got != nil {
					t.Errorf("expected nil config on error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv(EnvAudience, testAudience)
	t.Setenv(EnvServiceAccountEmail, testEmail)

	got, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Audience != testAudience || got.ServiceAccountEmail != testEmail {
		t.Errorf("unexpected config %+v", got)
	}
}
