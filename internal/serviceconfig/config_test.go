package serviceconfig

import (
	"bytes"
	"encoding/base64"
	"os"
	"testing"
)

func TestBuildReportIgnoresOverridesAndNonPublicFields(t *testing.T) {
	old := Encoded
	t.Cleanup(func() { Encoded = old })
	Encoded = base64.RawURLEncoding.EncodeToString([]byte(`{"YOTTA_HUB_URL":"https://online.example","internal-note":"not-public"}`))
	t.Setenv("YOTTA_HUB_URL", "http://127.0.0.1:8094")
	var out bytes.Buffer
	if err := WriteBuildValues(&out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "{\"YOTTA_HUB_URL\":\"https://online.example\"}\n"; got != want {
		t.Fatalf("report = %q, want %q", got, want)
	}
}

func TestBuildDefaultsAndRuntimeOverride(t *testing.T) {
	value, present := os.LookupEnv("YOTTA_REGISTRY_URL")
	_ = os.Unsetenv("YOTTA_REGISTRY_URL")
	t.Cleanup(func() {
		if present {
			_ = os.Setenv("YOTTA_REGISTRY_URL", value)
		} else {
			_ = os.Unsetenv("YOTTA_REGISTRY_URL")
		}
	})
	old := Encoded
	t.Cleanup(func() { Encoded = old })
	Encoded = base64.RawURLEncoding.EncodeToString([]byte(`{"YOTTA_REGISTRY_URL":"https://registry.example"}`))
	if value := Values()["YOTTA_REGISTRY_URL"]; value != "https://registry.example" {
		t.Fatal(value)
	}
	t.Setenv("YOTTA_REGISTRY_URL", "http://127.0.0.1:8090")
	if value := Values()["YOTTA_REGISTRY_URL"]; value != "http://127.0.0.1:8090" {
		t.Fatal(value)
	}
	t.Setenv("YOTTA_REGISTRY_URL", "")
	if value := Values()["YOTTA_REGISTRY_URL"]; value != "" {
		t.Fatal(value)
	}
}
