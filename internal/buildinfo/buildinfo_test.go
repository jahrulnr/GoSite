package buildinfo

import "testing"

func TestIsDev(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"1.0.0-dev", true},
		{"1.4.0-dev", true},
		{"1.0.0", false},
		{"v1.0.0", false},
	}
	for _, tc := range cases {
		Version = tc.version
		if got := IsDev(); got != tc.want {
			t.Fatalf("IsDev(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}
