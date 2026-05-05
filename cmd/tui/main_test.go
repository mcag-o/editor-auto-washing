package main

import "testing"

func TestDefaultAPIURLUsesWebControlPlanePort8123(t *testing.T) {
	if defaultAPIURL != "http://localhost:8123" {
		t.Fatalf("defaultAPIURL = %q, want %q", defaultAPIURL, "http://localhost:8123")
	}
}
