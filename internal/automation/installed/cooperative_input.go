package installed

import (
	"context"
	"errors"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
)

// ErrCooperativeCameraInput identifies input requiring an exclusive navigation
// branch. Keep this sentinel stable for the runtime's shared reason mapping.
var ErrCooperativeCameraInput = errors.New("this action cannot run alongside active navigation; use a paused marker branch for pointer input, replay, text entry, or window changes")

func checkCooperativeInput(ctx context.Context, operation string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !inputcoord.IsCooperative(ctx) {
		return nil
	}
	// Cooperating branches compose physical key claims. Other effects need a
	// paused marker; observations and cleanup do not disturb navigation.
	switch operation {
	case OperationPressKeys, OperationHoldKeys,
		OperationPointerPosition, OperationGetWindowState, OperationWaitWindow, OperationWaitWindowGone,
		OperationCapture, OperationReadCapture, OperationReleaseHeld:
		return nil
	default:
		return failure(CodeInvalidRequest, ErrCooperativeCameraInput)
	}
}
