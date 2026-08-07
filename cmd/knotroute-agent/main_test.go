package main

import "testing"

func TestTargetValidation(t *testing.T) {
	for _, v := range []string{"app:8080", "api-1:443"} {
		if !validTarget(v) {
			t.Fatalf("expected valid %q", v)
		}
	}
	for _, v := range []string{"/tmp:80", "app", "app:99999"} {
		if validTarget(v) {
			t.Fatalf("expected invalid %q", v)
		}
	}
}
