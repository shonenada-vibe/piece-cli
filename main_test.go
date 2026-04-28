package main

import "testing"

func TestEffectiveVersionUsesStampedVersion(t *testing.T) {
	original := version
	t.Cleanup(func() {
		version = original
	})

	version = "v1.2.3"
	if got := effectiveVersion(); got != "v1.2.3" {
		t.Fatalf("effectiveVersion() = %q, want %q", got, "v1.2.3")
	}
}

func TestEffectiveVersionFallsBackToDev(t *testing.T) {
	original := version
	t.Cleanup(func() {
		version = original
	})

	version = "dev"
	if got := effectiveVersion(); got == "" {
		t.Fatal("effectiveVersion() returned empty string")
	}
}
