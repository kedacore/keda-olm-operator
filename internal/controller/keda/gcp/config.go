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
// OpenShift passes the WIF parameters to OLM-managed operators as environment
// variables on the operator pod: the OperatorHub console sets PROJECT_NUMBER,
// POOL_ID, PROVIDER_ID and SERVICE_ACCOUNT_EMAIL through the Subscription, and
// CLI installs set AUDIENCE and SERVICE_ACCOUNT_EMAIL. From these the operator
// renders a GCP external_account credential configuration into a Secret and
// wires it, together with a projected service account token, into the
// keda-operator Deployment. KEDA scalers that use `podIdentity.provider: gcp`
// then authenticate to Google Cloud through Application Default Credentials
// with short-lived tokens and no long-lived key anywhere in the cluster.
package gcp

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
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

// ConfigFromEnv reads the WIF configuration from the process environment. It
// returns (nil, nil) when Workload Identity is not configured at all, and an
// error when it is configured only partially or with invalid values.
func ConfigFromEnv() (*Config, error) {
	return ParseConfig(os.Getenv)
}

// ParseConfig builds the WIF configuration from the given environment lookup.
// See ConfigFromEnv.
func ParseConfig(getenv func(string) string) (*Config, error) {
	email := strings.TrimSpace(getenv(EnvServiceAccountEmail))
	audience := strings.TrimSpace(getenv(EnvAudience))
	projectNumber := strings.TrimSpace(getenv(EnvProjectNumber))
	poolID := strings.TrimSpace(getenv(EnvPoolID))
	providerID := strings.TrimSpace(getenv(EnvProviderID))
	subjectTokenAudience := strings.TrimSpace(getenv(EnvSubjectTokenAudience))

	if email == "" && audience == "" && projectNumber == "" && poolID == "" && providerID == "" && subjectTokenAudience == "" {
		return nil, nil
	}

	var errs []error

	if email == "" {
		errs = append(errs, fmt.Errorf("%s must be set", EnvServiceAccountEmail))
	} else if err := validateServiceAccountEmail(email); err != nil {
		errs = append(errs, err)
	}

	audience, err := resolveAudience(audience, projectNumber, poolID, providerID)
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
		Audience:             audience,
		ServiceAccountEmail:  email,
		ProjectID:            strings.TrimSpace(getenv(EnvProjectID)),
		SubjectTokenAudience: subjectTokenAudience,
	}, nil
}

// resolveAudience returns the STS audience from either AUDIENCE or the
// PROJECT_NUMBER/POOL_ID/PROVIDER_ID triplet. When both are given they have to
// agree, so that an install can't silently end up with a provider the
// administrator didn't intend.
func resolveAudience(audience, projectNumber, poolID, providerID string) (string, error) {
	tripletSet := projectNumber != "" || poolID != "" || providerID != ""

	if audience == "" && !tripletSet {
		return "", fmt.Errorf("either %s or all of %s, %s and %s must be set",
			EnvAudience, EnvProjectNumber, EnvPoolID, EnvProviderID)
	}

	if audience != "" && !audienceRegexp.MatchString(audience) {
		return "", fmt.Errorf("%s %q is not a workload identity provider resource name of the form %s",
			EnvAudience, audience, fmt.Sprintf(audienceFormat, "<project_number>", "<pool_id>", "<provider_id>"))
	}

	if !tripletSet {
		return audience, nil
	}

	var missing []string
	for _, v := range []struct{ name, value string }{
		{EnvProjectNumber, projectNumber},
		{EnvPoolID, poolID},
		{EnvProviderID, providerID},
	} {
		if v.value == "" {
			missing = append(missing, v.name)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("%s must be set together with %s, %s and %s", strings.Join(missing, " and "),
			EnvProjectNumber, EnvPoolID, EnvProviderID)
	}
	if !projectNumberRegexp.MatchString(projectNumber) {
		return "", fmt.Errorf("%s %q must be numeric", EnvProjectNumber, projectNumber)
	}
	if !identifierRegexp.MatchString(poolID) {
		return "", fmt.Errorf("%s %q contains characters not allowed in a workload identity pool ID", EnvPoolID, poolID)
	}
	if !identifierRegexp.MatchString(providerID) {
		return "", fmt.Errorf("%s %q contains characters not allowed in a workload identity provider ID", EnvProviderID, providerID)
	}

	built := fmt.Sprintf(audienceFormat, projectNumber, poolID, providerID)
	if audience != "" && audience != built {
		return "", fmt.Errorf("%s %q does not match the provider %q built from %s, %s and %s; set only one of them",
			EnvAudience, audience, built, EnvProjectNumber, EnvPoolID, EnvProviderID)
	}
	return built, nil
}

// validateServiceAccountEmail catches the typical mistakes (a project number or
// a bare name pasted into the field) without being stricter than Google is
// about the local part or the domain.
func validateServiceAccountEmail(email string) error {
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" || strings.Contains(domain, "@") || !strings.Contains(domain, ".") {
		return fmt.Errorf("%s %q is not a service account email address", EnvServiceAccountEmail, email)
	}
	return nil
}
