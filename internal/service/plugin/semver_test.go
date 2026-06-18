package plugin

import "testing"

func TestCompareSemver_DevSuffix(t *testing.T) {
	if compareSemver("1.0.0-dev", "1.0.0") != 0 {
		t.Fatal("1.0.0-dev should match 1.0.0 for compatibility")
	}
	if compareSemver("1.0.0", "1.0.0-dev") != 0 {
		t.Fatal("1.0.0 should match 1.0.0-dev for compatibility")
	}
	if compareSemver("1.0.0-dev", "1.1.0") >= 0 {
		t.Fatal("1.0.0-dev should be less than 1.1.0")
	}
	if compareSemver("1.4.0", "1.0.0-dev") <= 0 {
		t.Fatal("1.4.0 should be greater than 1.0.0-dev")
	}
}
