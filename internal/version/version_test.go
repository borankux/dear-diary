package version

import "testing"

func TestServerVersionUsesCLIVersion(t *testing.T) {
	if CLI == "" {
		t.Fatal("CLI version must not be empty")
	}
	if Server != CLI+"-server" {
		t.Fatalf("Server version = %q, want %q", Server, CLI+"-server")
	}
}
