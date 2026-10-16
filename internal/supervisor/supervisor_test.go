package supervisor

import (
	"strings"
	"testing"
)

func TestDeploymentEnvironmentCannotOverrideLocalInstance(t *testing.T) {
	input := []string{"PATH=/bin", "PGSTORE_DSN=private", "home_jwt=private", "MANAGEMENT_PASSWORD=private", "WRITABLE_PATH=wrong", "MANAGEMENT_STATIC_PATH=wrong", "CPA_FIXTURE_EXIT=1"}
	out := strings.Join(ChildEnvironment(input, "data path", "static path"), "\n")
	for _, removed := range []string{"private", "wrong"} {
		if strings.Contains(out, removed) {
			t.Fatal("deployment override was inherited")
		}
	}
	for _, retained := range []string{"PATH=/bin", "WRITABLE_PATH=data path", "MANAGEMENT_STATIC_PATH=static path", "CPA_FIXTURE_EXIT=1"} {
		if !strings.Contains(out, retained) {
			t.Fatal("missing child environment entry:", retained)
		}
	}
}
