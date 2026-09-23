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

// Package gcp implements GCP Workload Identity Federation (WIF) support for the
// keda-operator Deployment managed by this operator.
//
// The WIF parameters come from spec.operator.gcpWorkloadIdentity of the
// KedaController or, when that is not set, from environment variables on the
// operator pod. The latter is how OpenShift passes them to OLM-managed operators:
// the OperatorHub console sets PROJECT_NUMBER, POOL_ID, PROVIDER_ID and
// SERVICE_ACCOUNT_EMAIL through the Subscription, and CLI installs set AUDIENCE
// and SERVICE_ACCOUNT_EMAIL. From these the operator renders a GCP
// external_account credential configuration into a Secret and wires it,
// together with a projected service account token, into the keda-operator
// Deployment. KEDA scalers that use `podIdentity.provider: gcp` then
// authenticate to Google Cloud through Application Default Credentials with
// short-lived tokens and no long-lived key anywhere in the cluster.
package gcp

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
)

// Environment variables read from the operator pod. On OpenShift they are set on
// the OLM Subscription (spec.config.env), either by the OperatorHub console or by
// the administrator.
const (
	// EnvServiceAccountEmail is the GCP service account that keda-operator impersonates.
	EnvServiceAccountEmail = "SERVICE_ACCOUNT_EMAIL"
	// EnvAudience is the full workload identity provider resource name, in the form
	// //iam.googleapis.com/projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>.
	// It is what CLI installs set; the console sets the three components below instead.
	EnvAudience = "AUDIENCE"
	// EnvProjectNumber, EnvPoolID and EnvProviderID are the components of the audience
	// as collected by the OperatorHub console install form.
	EnvProjectNumber = "PROJECT_NUMBER"
	EnvPoolID        = "POOL_ID"
	EnvProviderID    = "PROVIDER_ID"
	// EnvProjectID optionally sets the GCP project keda-operator uses by default
	// (passed through as CLOUDSDK_CORE_PROJECT). When unset, the project of the
	// OpenShift cluster is used.
	EnvProjectID = "CLOUDSDK_CORE_PROJECT"
	// EnvSubjectTokenAudience optionally overrides the audience of the projected
	// Kubernetes service account token that keda-operator exchanges at the GCP STS
	// endpoint. It has to be one of the allowed audiences of the workload identity
	// provider. Defaults to DefaultSubjectTokenAudience, which is what ccoctl configures.
	EnvSubjectTokenAudience = "SUBJECT_TOKEN_AUDIENCE"

	// DefaultSubjectTokenAudience is the allowed audience ccoctl configures on the
	// workload identity provider of an OpenShift cluster.
	DefaultSubjectTokenAudience = "openshift"

	audienceFormat = "//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s"
)

var (
	audienceRegexp      = regexp.MustCompile(`^//iam\.[^/]+/projects/[^/]+/locations/[^/]+/workloadIdentityPools/[^/]+/providers/[^/]+$`)
	projectNumberRegexp = regexp.MustCompile(`^[0-9]+$`)
	identifierRegexp    = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// Config holds the validated GCP Workload Identity Federation parameters.
type Config struct {
	// Source tells where the parameters come from.
	Source kedav1alpha1.GCPWorkloadIdentitySource
	// Audience is the STS audience, i.e. the workload identity provider resource name.
	Audience string
	// ServiceAccountEmail is the GCP service account to impersonate.
	ServiceAccountEmail string
	// ProjectID is the explicitly configured GCP project for keda-operator, or empty
	// to use the project of the cluster.
	ProjectID string
	// SubjectTokenAudience is the audience of the projected Kubernetes service account token.
	SubjectTokenAudience string
}

// specFieldNames maps each parameter to its field in spec.operator.gcpWorkloadIdentity,
// so that validation errors name what the administrator actually set.
var specFieldNames = map[string]string{
	EnvServiceAccountEmail:  "serviceAccountEmail",
	EnvAudience:             "audience",
	EnvProjectNumber:        "projectNumber",
	EnvPoolID:               "poolID",
	EnvProviderID:           "providerID",
	EnvProjectID:            "projectID",
	EnvSubjectTokenAudience: "subjectTokenAudience",
}

// ConfigFromEnv reads the WIF configuration from the process environment. It
// returns (nil, nil) when Workload Identity is not configured at all, and an
// error when it is configured only partially or with invalid values.
func ConfigFromEnv() (*Config, error) {
	return ParseConfig(os.Getenv)
}

// ParseConfig builds the WIF configuration from the given environment lookup.
// See ConfigFromEnv.
func ParseConfig(getenv func(string) string) (*Config, error) {
	return parseConfig(getenv, func(env string) string { return env }, false,
		kedav1alpha1.GCPWorkloadIdentitySourceOperatorEnvironment)
}

// ConfigFromSpec builds the WIF configuration from spec.operator.gcpWorkloadIdentity.
// The CRD already rejects most invalid values at admission; the same validation runs
// here as well, because a KedaController may have been created before the CRD carried
// those rules.
func ConfigFromSpec(spec *kedav1alpha1.GCPWorkloadIdentitySpec) (*Config, error) {
	values := map[string]string{
		EnvServiceAccountEmail:  spec.ServiceAccountEmail,
		EnvAudience:             spec.Audience,
		EnvProjectNumber:        spec.ProjectNumber,
		EnvPoolID:               spec.PoolID,
		EnvProviderID:           spec.ProviderID,
		EnvProjectID:            spec.ProjectID,
		EnvSubjectTokenAudience: spec.SubjectTokenAudience,
	}
	return parseConfig(func(key string) string { return values[key] },
		func(env string) string { return specFieldNames[env] }, true,
		kedav1alpha1.GCPWorkloadIdentitySourceKedaController)
}

// parseConfig validates the parameters read through lookup, which is keyed by the
// environment variable names. name turns such a key into the name the administrator
// knows it by. Unless required is set, no parameters at all means that Workload
// Identity is not configured, which is reported as (nil, nil).
func parseConfig(lookup, name func(string) string, required bool, source kedav1alpha1.GCPWorkloadIdentitySource) (*Config, error) {
	email := strings.TrimSpace(lookup(EnvServiceAccountEmail))
	audience := strings.TrimSpace(lookup(EnvAudience))
	projectNumber := strings.TrimSpace(lookup(EnvProjectNumber))
	poolID := strings.TrimSpace(lookup(EnvPoolID))
	providerID := strings.TrimSpace(lookup(EnvProviderID))
	subjectTokenAudience := strings.TrimSpace(lookup(EnvSubjectTokenAudience))

	if !required && email == "" && audience == "" && projectNumber == "" && poolID == "" && providerID == "" && subjectTokenAudience == "" {
		return nil, nil
	}

	var errs []error

	if email == "" {
		errs = append(errs, fmt.Errorf("%s must be set", name(EnvServiceAccountEmail)))
	} else if err := validateServiceAccountEmail(email, name); err != nil {
		errs = append(errs, err)
	}

	audience, err := resolveAudience(audience, projectNumber, poolID, providerID, name)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	if subjectTokenAudience == "" {
		subjectTokenAudience = DefaultSubjectTokenAudience
	}

	return &Config{
		Source:               source,
		Audience:             audience,
		ServiceAccountEmail:  email,
		ProjectID:            strings.TrimSpace(lookup(EnvProjectID)),
		SubjectTokenAudience: subjectTokenAudience,
	}, nil
}

// resolveAudience returns the STS audience from either the audience itself or the
// project number, pool ID and provider ID. When both are given they have to agree,
// so that an install can't silently end up with a provider the administrator
// didn't intend.
func resolveAudience(audience, projectNumber, poolID, providerID string, name func(string) string) (string, error) {
	tripletSet := projectNumber != "" || poolID != "" || providerID != ""

	if audience == "" && !tripletSet {
		return "", fmt.Errorf("either %s or all of %s, %s and %s must be set",
			name(EnvAudience), name(EnvProjectNumber), name(EnvPoolID), name(EnvProviderID))
	}

	if audience != "" && !audienceRegexp.MatchString(audience) {
		return "", fmt.Errorf("%s %q is not a workload identity provider resource name of the form %s",
			name(EnvAudience), audience, fmt.Sprintf(audienceFormat, "<project_number>", "<pool_id>", "<provider_id>"))
	}

	if !tripletSet {
		return audience, nil
	}

	var missing []string
	for _, v := range []struct{ key, value string }{
		{EnvProjectNumber, projectNumber},
		{EnvPoolID, poolID},
		{EnvProviderID, providerID},
	} {
		if v.value == "" {
			missing = append(missing, name(v.key))
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("%s must be set together with %s, %s and %s", strings.Join(missing, " and "),
			name(EnvProjectNumber), name(EnvPoolID), name(EnvProviderID))
	}
	if !projectNumberRegexp.MatchString(projectNumber) {
		return "", fmt.Errorf("%s %q must be numeric", name(EnvProjectNumber), projectNumber)
	}
	if !identifierRegexp.MatchString(poolID) {
		return "", fmt.Errorf("%s %q contains characters not allowed in a workload identity pool ID", name(EnvPoolID), poolID)
	}
	if !identifierRegexp.MatchString(providerID) {
		return "", fmt.Errorf("%s %q contains characters not allowed in a workload identity provider ID", name(EnvProviderID), providerID)
	}

	built := fmt.Sprintf(audienceFormat, projectNumber, poolID, providerID)
	if audience != "" && audience != built {
		return "", fmt.Errorf("%s %q does not match the provider %q built from %s, %s and %s; set only one of them",
			name(EnvAudience), audience, built, name(EnvProjectNumber), name(EnvPoolID), name(EnvProviderID))
	}
	return built, nil
}

// validateServiceAccountEmail catches the typical mistakes (a project number or
// a bare name pasted into the field) without being stricter than Google is
// about the local part or the domain.
func validateServiceAccountEmail(email string, name func(string) string) error {
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" || strings.Contains(domain, "@") || !strings.Contains(domain, ".") {
		return fmt.Errorf("%s %q is not a service account email address", name(EnvServiceAccountEmail), email)
	}
	return nil
}
