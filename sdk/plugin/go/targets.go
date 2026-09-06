package pluginsdk

import (
	"encoding/json"
	"github.com/yottaapp/yotta/internal/artifact"
)

const ConfiguredTargetsHostFeatureID = "https://schemas.yotta.dev/host-features/plugin-configured-targets/v1"

// OpenTarget opens a ConfiguredTargetSpec declared by the Node Contract. Its
// local slot is selected from the node config by the host, not by the guest.
func (guest *Guest) OpenTarget(requestID, targetID, kind string, operations []string, config json.RawMessage) (*HostOpenResponse, error) {
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	raw, err := artifact.Marshal(struct {
		Kind   string          `json:"kind"`
		Config json.RawMessage `json:"config"`
	}{kind, config})
	if err != nil {
		return nil, err
	}
	return guest.Open(requestID, targetID, operations, raw)
}
