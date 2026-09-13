package installed

import (
	"context"
	"errors"
	"testing"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
)

func TestCooperativeInputOperations(t *testing.T) {
	cooperative := inputcoord.WithOwner(context.Background(), inputcoord.NewCooperatingOwner(inputcoord.NewOwner()))
	allowed := map[string]bool{
		OperationPressKeys: true, OperationHoldKeys: true, OperationReleaseHeld: true,
		OperationPointerPosition: true, OperationGetWindowState: true,
		OperationWaitWindow: true, OperationWaitWindowGone: true,
		OperationCapture: true, OperationReadCapture: true,
	}
	for _, operation := range append(Operations(), "future-operation") {
		t.Run(operation, func(t *testing.T) {
			err := checkCooperativeInput(cooperative, operation)
			if allowed[operation] {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var classified *Failure
				if !errors.Is(err, ErrCooperativeCameraInput) || !errors.As(err, &classified) || classified.Code != CodeInvalidRequest {
					t.Fatalf("exclusive effect lost shared error contract: %v", err)
				}
			}
			// Independent paused/recovery scopes retain their normal operation path.
			if err := checkCooperativeInput(inputcoord.WithOwner(cooperative, inputcoord.NewOwner()), operation); err != nil {
				t.Fatal(err)
			}
		})
	}
}
