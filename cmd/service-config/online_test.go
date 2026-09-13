package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yottaapp/yotta/internal/serviceconfig"
)

func TestOnlineProfileRejectsDevelopmentAndMissingSettings(t *testing.T) {
	profile := filepath.Join("..", "..", ".env.online.example")
	expected, err := readOnlineProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := compareServices(expected, expected); err != nil {
		t.Fatal(err)
	}
	for _, key := range serviceconfig.Keys {
		t.Run(key, func(t *testing.T) {
			actual := map[string]string{}
			for k, v := range expected {
				actual[k] = v
			}
			delete(actual, key)
			if err := compareServices(actual, expected); (err == nil) != (expected[key] == "") {
				t.Fatalf("missing setting result: %v", err)
			}
			actual[key] = "http://127.0.0.1:8090"
			if err := compareServices(actual, expected); err == nil {
				t.Fatal("development override accepted")
			}
		})
	}
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.ReplaceAll(string(raw), "https://api.yuelili.com/yotta/hub", "http://127.0.0.1:8094"),
		string(raw) + "\nYOTTA_CLIENT_SECRET=forbidden-field\n",
		string(raw) + "\nYOTTA_HUB_URL=https://example.test\n",
		strings.ReplaceAll(string(raw), "YOTTA_HUB_ALLOW_INSECURE_HTTP=false", "YOTTA_HUB_ALLOW_INSECURE_HTTP=true"),
	} {
		path := filepath.Join(t.TempDir(), "profile.env")
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readOnlineProfile(path); err == nil {
			t.Fatal("invalid release profile accepted")
		}
	}
}

func TestVerifyPackagedServicesReadsBinaryNotEnvironment(t *testing.T) {
	profile := filepath.Join("..", "..", ".env.online.example")
	expected, err := readOnlineProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "service-config.exe")
	flags := "-s -w -X github.com/yottaapp/yotta/internal/serviceconfig.Encoded=" + base64.RawURLEncoding.EncodeToString(raw)
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", flags, "-o", exe, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, output)
	}
	t.Setenv("YOTTA_HUB_URL", "http://127.0.0.1:8094")
	if err := verifyOnline(profile, exe); err != nil {
		t.Fatal(err)
	}
	if err := verifyOnline(profile, ""); err == nil {
		t.Fatal("environment validation accepted development endpoints")
	}
	if _, err := embeddedServices(profile); err == nil {
		t.Fatal("non-executable accepted")
	}
}
