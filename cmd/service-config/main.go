// service-config emits a shell-safe encoding of the public build-time endpoints.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/yottaapp/yotta/internal/serviceconfig"
)

func main() {
	raw, err := json.Marshal(serviceconfig.Values())
	if err != nil {
		panic(err)
	}
	fmt.Print(base64.RawURLEncoding.EncodeToString(raw))
}
