package serviceconfig

import (
	"encoding/base64"
	"os"
	"testing"
)

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
