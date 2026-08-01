package util

import (
	"testing"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
)

func TestConvertTLSProfileSpec(t *testing.T) {
	tests := map[string]struct {
		profile        openshiftconfigv1.TLSProfileSpec
		wantMinVersion string
		wantCiphers    []string
	}{
		"converts version and OpenSSL ciphers to IANA": {
			profile: openshiftconfigv1.TLSProfileSpec{
				MinTLSVersion: "VersionTLS12",
				Ciphers:       []string{"ECDHE-ECDSA-AES128-GCM-SHA256", "ECDHE-RSA-AES256-GCM-SHA384"},
			},
			wantMinVersion: "TLS12",
			wantCiphers:    []string{"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
		},
		"passes through IANA cipher names unchanged": {
			profile: openshiftconfigv1.TLSProfileSpec{
				MinTLSVersion: "VersionTLS13",
				Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
			},
			wantMinVersion: "TLS13",
			wantCiphers:    []string{"TLS_AES_128_GCM_SHA256"},
		},
		"handles nil cipher list": {
			profile: openshiftconfigv1.TLSProfileSpec{
				MinTLSVersion: "VersionTLS12",
			},
			wantMinVersion: "TLS12",
			wantCiphers:    nil,
		},
		"drops unknown ciphers": {
			profile: openshiftconfigv1.TLSProfileSpec{
				MinTLSVersion: "VersionTLS12",
				Ciphers:       []string{"ECDHE-ECDSA-AES128-GCM-SHA256", "MADE-UP-CIPHER"},
			},
			wantMinVersion: "TLS12",
			wantCiphers:    []string{"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotMinVersion, gotCiphers := ConvertTLSProfileSpec(tc.profile)
			if gotMinVersion != tc.wantMinVersion {
				t.Errorf("minTLSVersion = %q, want %q", gotMinVersion, tc.wantMinVersion)
			}
			if len(gotCiphers) != len(tc.wantCiphers) {
				t.Fatalf("ciphers length = %d, want %d\ngot:  %v\nwant: %v", len(gotCiphers), len(tc.wantCiphers), gotCiphers, tc.wantCiphers)
			}
			for i := range gotCiphers {
				if gotCiphers[i] != tc.wantCiphers[i] {
					t.Errorf("cipher[%d] = %q, want %q", i, gotCiphers[i], tc.wantCiphers[i])
				}
			}
		})
	}
}
