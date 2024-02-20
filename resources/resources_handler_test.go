package resources

import "testing"

// TestResourceHandler tests the GetResourcesManifest function
func TestResourceHandler(t *testing.T) {
	_, err := GetResourcesManifest("")
	if err != nil {
		t.Errorf("GetResourcesManifest failed: %v", err)
	}
	_, err = GetResourcesManifest("keda-not-released")
	if err == nil {
		t.Errorf("GetResourcesManifest failed: %v", err)
	}
}

// TestValidateSemanticVersion tests the validateSemanticVersion function
func TestValidateSemanticVersion(t *testing.T) {
	err := validateSemanticVersion("v2.0")
	if err != nil {
		t.Errorf("validateSemanticVersion failed: %v", err)
	}
	err = validateSemanticVersion("v2.0.0")
	if err != nil {
		t.Errorf("validateSemanticVersion failed: %v", err)
	}
	err = validateSemanticVersion("")
	if err != nil {
		t.Errorf("validateSemanticVersion failed: %v", err)
	}
}
