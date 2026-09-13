// service-config emits a shell-safe encoding of the public build-time endpoints.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/yottaapp/yotta/internal/serviceconfig"
)

func main() {
	verify := flag.Bool("verify-online", false, "verify public release endpoints against the online profile")
	profile := flag.String("profile", ".env.online.example", "public online service profile")
	binary := flag.String("binary", "", "verify embedded endpoints in this executable instead of the environment")
	printBuild := flag.Bool("print-build-service-config", false, "print embedded public service configuration")
	flag.Parse()
	if *printBuild {
		if err := serviceconfig.WriteBuildValues(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *verify {
		if err := verifyOnline(*profile, *binary); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("online release services verified")
		return
	}
	raw, err := json.Marshal(serviceconfig.Values())
	if err != nil {
		panic(err)
	}
	fmt.Print(base64.RawURLEncoding.EncodeToString(raw))
}
