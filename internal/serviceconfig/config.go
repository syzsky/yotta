// Package serviceconfig resolves public service endpoints for both packaged and development apps.
package serviceconfig

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// Encoded is populated by task build. It contains public endpoints only, never credentials.
var Encoded string

var Keys = []string{"YOTTA_ACCOUNT_URL", "YOTTA_HUB_URL", "YOTTA_HUB_ALLOW_INSECURE_HTTP", "YOTTA_REGISTRY_URL", "YOTTA_REGISTRY_ALLOW_INSECURE_HTTP", "YOTTA_OIDC_AUTHORIZATION_ENDPOINT", "YOTTA_OIDC_TOKEN_ENDPOINT", "YOTTA_OIDC_USERINFO_ENDPOINT", "YOTTA_OIDC_CLIENT_ID", "YOTTA_OIDC_CALLBACK_ADDRESS", "YOTTA_REGISTRY_AUDIENCE"}

func BuildValues() map[string]string {
	values := map[string]string{}
	if raw, err := base64.RawURLEncoding.DecodeString(Encoded); err == nil {
		_ = json.Unmarshal(raw, &values)
	}
	return values
}

// WriteBuildValues reports only the embedded public fields, without applying
// environment overrides or initializing any application services.
func WriteBuildValues(out io.Writer) error {
	values := BuildValues()
	public := map[string]string{}
	for _, key := range Keys {
		if value, ok := values[key]; ok {
			public[key] = value
		}
	}
	return json.NewEncoder(out).Encode(public)
}

func Values() map[string]string {
	values := BuildValues()
	for _, key := range Keys {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = strings.TrimSpace(value)
		}
	}
	return values
}
