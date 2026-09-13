package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, "--print-build-service-config")
	if runtime.GOOS == "windows" {
		// This read-only entry never initializes the desktop or input drivers.
		// Run it at the verifier's existing privilege level without a UAC prompt.
		command.Env = append(os.Environ(), "__COMPAT_LAYER=RunAsInvoker")
	}
	raw, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read executable public configuration: %w", err)
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
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
