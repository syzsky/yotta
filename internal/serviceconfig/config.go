// Package serviceconfig resolves public service endpoints for both packaged and development apps.
package serviceconfig

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
)

// Encoded is populated by task build. It contains public endpoints only, never credentials.
var Encoded string

var Keys = []string{"YOTTA_ACCOUNT_URL", "YOTTA_HUB_URL", "YOTTA_HUB_ALLOW_INSECURE_HTTP", "YOTTA_REGISTRY_URL", "YOTTA_REGISTRY_ALLOW_INSECURE_HTTP", "YOTTA_OIDC_AUTHORIZATION_ENDPOINT", "YOTTA_OIDC_TOKEN_ENDPOINT", "YOTTA_OIDC_USERINFO_ENDPOINT", "YOTTA_OIDC_CLIENT_ID", "YOTTA_OIDC_CALLBACK_ADDRESS", "YOTTA_REGISTRY_AUDIENCE"}

func Values() map[string]string {
	values := map[string]string{}
	if raw, err := base64.RawURLEncoding.DecodeString(Encoded); err == nil {
		_ = json.Unmarshal(raw, &values)
	}
	for _, key := range Keys {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = strings.TrimSpace(value)
		}
	}
	return values
}
