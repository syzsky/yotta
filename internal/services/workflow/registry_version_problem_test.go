package workflow

import (
	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/registryclient"
	"testing"
)

func TestNonIncreasingVersionHasActionableCorrelatedProblem(t *testing.T) {
	const operation = "8968a822-0ce3-4c91-9a55-104a1f75b7bc"
	err := registryError("publish", registryclient.Problem{Code: "registry.version_not_increasing", OperationID: operation})
	problem := apperr.From(err)
	if problem.ID != "workflow.registry.version_not_increasing" || problem.Retryable || problem.OperationID != operation || problem.Category != apperr.CategoryValidation {
		t.Fatalf("problem=%#v", problem)
	}
}
