package registryclient

import (
	"regexp"
	"strings"
)

var workflowReleaseVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

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
