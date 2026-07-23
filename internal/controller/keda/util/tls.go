package util

import (
	"strings"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
)

// ConvertTLSProfileSpec converts a TLSProfileSpec into env-var-ready values.
// MinTLSVersion is converted from "VersionTLS12" to "TLS12".
// Ciphers are converted from OpenSSL names to Go/IANA names; unknown ciphers are dropped.
func ConvertTLSProfileSpec(profile openshiftconfigv1.TLSProfileSpec) (string, []string) {
	minVer := strings.TrimPrefix(string(profile.MinTLSVersion), "Version")
	ianaCiphers := libgocrypto.OpenSSLToIANACipherSuites(profile.Ciphers)
	return minVer, ianaCiphers
}
