package registryclient

import (
	"regexp"
	"strings"
)

var workflowReleaseVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var environmentVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

func ValidWorkflowReleaseVersion(value string) bool {
	return len(value) <= 128 && workflowReleaseVersion.MatchString(value)
}

func IsNewerWorkflowVersion(candidate, installed string) bool {
	if !ValidWorkflowReleaseVersion(candidate) || !ValidWorkflowReleaseVersion(installed) {
		return false
	}
	left, right := strings.Split(candidate, "."), strings.Split(installed, ".")
	for i := 0; i < 3; i++ {
		if len(left[i]) != len(right[i]) {
			return len(left[i]) > len(right[i])
		}
		if left[i] != right[i] {
			return left[i] > right[i]
		}
	}
	return false
}

func NormalizeEnvironmentVersion(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if environmentVersion.MatchString(value) {
		return value
	}
	return "0.0.0-dev"
}
