package registryclient

import "regexp"

var workflowReleaseVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func ValidWorkflowReleaseVersion(value string) bool {
	return len(value) <= 128 && workflowReleaseVersion.MatchString(value)
}
