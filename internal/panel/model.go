package panel

import (
	"crypto/sha256"
	"fmt"
)

type Interaction struct {
	Sequence    int64  `json:"sequence"`
	ComponentID string `json:"componentId"`
	EventID     string `json:"eventId"`
	Name        string `json:"name"`
	Value       any    `json:"value"`
}

// RunError keeps recovery semantics distinct without exposing internal causes.
type RunError string

func (e RunError) Error() string { return string(e) }

const (
	ErrPanelEnded              RunError = "panel_ended"
	ErrPanelMissing            RunError = "panel_missing"
	ErrComponentMissing        RunError = "component_missing"
	ErrComponentConflict       RunError = "component_conflict"
	ErrComponentReadOnly       RunError = "component_read_only"
	ErrComponentNoValue        RunError = "component_no_value"
	ErrComponentNotInteractive RunError = "component_not_interactive"
	ErrInvalidValue            RunError = "invalid_value"
	ErrInvalidDefinition       RunError = "invalid_definition"
	ErrPanelCapacity           RunError = "capacity"
	ErrPanelQueueFull          RunError = "queue_full"
	ErrPanelUnavailable        RunError = "unavailable"
)

// PluginSourceID fits the existing Workflow Target slot grammar while retaining
// stable plugin ownership and definition identity across package updates.
func PluginSourceID(owner, id string) string {
	return fmt.Sprintf("plugin-panel-%x", sha256.Sum256([]byte(owner+"\x00"+id)))
}
