package workflow

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/registryclient"
)

func TestRegistryErrorPreservesRecoverySemantics(t *testing.T) {
	tests := []struct {
		name      string
		cause     error
		id        string
		category  string
		retryable bool
	}{
		{"daily submission limit", registryclient.Problem{Code: "registry.submission.daily_limit", Status: 429}, "workflow.registry.submission_daily_limit", apperr.CategoryPolicy, false},
		{"conflict", registryclient.Problem{Code: "registry.release_version_conflict"}, "workflow.registry.release_version_conflict", apperr.CategoryDomain, false},
		{"ownership", registryclient.Problem{Code: "registry.workflow_not_owner"}, "workflow.registry.not_owner", apperr.CategoryPolicy, false},
		{"purchase", registryclient.Problem{Code: "registry.purchase_required"}, "workflow.registry.purchase_required", apperr.CategoryPolicy, false},
		{"cancel unsupported", registryclient.Problem{Code: "commerce.payment_cancel_unsupported"}, "workflow.checkout.cancel_unsupported", apperr.CategoryPolicy, false},
		{"payment expired", registryclient.Problem{Code: "commerce.payment_session_expired"}, "workflow.checkout.session_expired", apperr.CategoryDomain, true},
		{"order changed", registryclient.Problem{Code: "commerce.order_invalid_state"}, "workflow.checkout.order_changed", apperr.CategoryDomain, true},
		{"authentication", registryclient.Problem{Code: "registry.authentication_required"}, "workflow.registry.authentication_required", apperr.CategoryPolicy, false},
		{"rejected", registryclient.Problem{Code: "registry.bundle_required"}, "workflow.registry.workflow_rejected", apperr.CategoryDomain, false},
		{"capacity", registryclient.Problem{Code: "registry.bundle_too_large"}, "workflow.registry.bundle_too_large", apperr.CategoryValidation, false},
		{"invalid search", registryclient.Problem{Code: "registry.invalid_search"}, "workflow.registry.invalid_search", apperr.CategoryValidation, false},
		{"cancelled", context.Canceled, "workflow.registry.cancelled", apperr.CategoryInfrastructure, false},
		{"timeout", context.DeadlineExceeded, "workflow.registry.timeout", apperr.CategoryInfrastructure, true},
		{"unavailable", errors.New("dial secret.internal:1234"), "workflow.registry.unavailable", apperr.CategoryInfrastructure, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := registryError("publish", test.cause)
			envelope := apperr.From(err)
			if envelope.ID != test.id || envelope.Category != test.category || envelope.Retryable != test.retryable {
				t.Fatalf("envelope = %#v", envelope)
			}
			encoded := apperr.Marshal(err)
			if bytes.Contains(encoded, []byte("secret.internal")) {
				t.Fatalf("raw cause leaked into envelope: %s", encoded)
			}
		})
	}
}

func TestRegistryErrorPreservesServerOperationID(t *testing.T) {
	cause := registryclient.Problem{Code: "registry.invalid_summary", OperationID: "8968a822-0ce3-4c91-9a55-104a1f75b7bc"}
	result := apperr.From(registryError("publish", cause))
	if result.ID != "workflow.registry.invalid_summary" || result.OperationID != cause.OperationID || result.Retryable {
		t.Fatalf("envelope=%#v", result)
	}
}
