package main

import (
	"bufio"
	"debug/buildinfo"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/yottaapp/yotta/internal/serviceconfig"
)

func readOnlineProfile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	values := map[string]string{}
	allowed := map[string]bool{}
	for _, key := range serviceconfig.Keys {
		allowed[key] = true
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !allowed[key] {
			return nil, fmt.Errorf("unknown online profile field %q", key)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate online profile field %q", key)
		}
		values[key] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for _, key := range serviceconfig.Keys {
		value, exists := values[key]
		if !exists {
			return nil, fmt.Errorf("online profile is missing %s", key)
		}
		if strings.HasSuffix(key, "_URL") || strings.HasSuffix(key, "_ENDPOINT") {
			u, err := url.Parse(value)
			if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" {
				return nil, fmt.Errorf("%s must be a public HTTPS endpoint", key)
			}
		}
		if strings.HasSuffix(key, "_ALLOW_INSECURE_HTTP") && value != "false" {
			return nil, fmt.Errorf("%s must be false for releases", key)
		}
	}
	return values, nil
}

func embeddedServices(path string) (map[string]string, error) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return nil, err
	}
	const prefix = "github.com/yottaapp/yotta/internal/serviceconfig.Encoded="
	for _, setting := range info.Settings {
		if setting.Key != "-ldflags" {
			continue
		}
		for _, field := range strings.Fields(setting.Value) {
			if !strings.HasPrefix(field, prefix) {
				continue
			}
			raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(field, prefix))
			if err != nil {
				return nil, err
			}
			var values map[string]string
			if err := json.Unmarshal(raw, &values); err != nil {
				return nil, err
			}
			return values, nil
		}
	}
	return nil, fmt.Errorf("executable has no embedded public service configuration")
}

func verifyOnline(profile, binary string) error {
	expected, err := readOnlineProfile(profile)
	if err != nil {
		return err
	}
	actual := serviceconfig.Values()
	if binary != "" {
		actual, err = embeddedServices(binary)
		if err != nil {
			return err
		}
	}
	return compareServices(actual, expected)
}

func compareServices(actual, expected map[string]string) error {
	for _, key := range serviceconfig.Keys {
		value, exists := actual[key]
		// An omitted optional audience and an explicit empty audience both ask
		// Identity for the public client's registered audiences.
		if (!exists && expected[key] != "") || value != expected[key] {
			return fmt.Errorf("release service %s does not match the online profile", key)
		}
	}
	return nil
}
