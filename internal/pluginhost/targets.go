package pluginhost

import (
	"encoding/json"
	"errors"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/pluginprotocol"
	"github.com/yottaapp/yotta/internal/targetruntime"
)

const ConfiguredTargetsHostFeatureID = "https://schemas.yotta.dev/host-features/plugin-configured-targets/v1"

func (session *processSession) configuredTarget(id string) *nodecontract.ConfiguredTargetSpec {
	for i := range session.targetSpecs {
		if session.targetSpecs[i].ID == id {
			return &session.targetSpecs[i]
		}
	}
	return nil
}

// The existing resource exchange carries a configured target's declared ID.
// Device configuration remains in the host; no native path or credential is
// sent to the guest, and no capability grant is constructed for the target.
func (session *processSession) openTarget(request *pluginprotocol.HostOpenRequest) ([]byte, error) {
	spec := session.configuredTarget(request.RequirementId)
	if spec == nil || session.invocation.Targets == nil {
		return nil, errors.New("configured target is unavailable")
	}
	slot, _ := session.invocation.Config[spec.SlotConfigKey].(string)
	var config struct {
		Kind   string          `json:"kind"`
		Config json.RawMessage `json:"config"`
	}
	if err := decodeCanonical(request.ConfigJson, &config); err != nil {
		return nil, err
	}
	handle, err := session.invocation.Targets.Open(session.ctx, targetruntime.OpenRequest{Slot: slot, Kind: config.Kind, Operations: request.Operations, Config: config.Config})
	if err != nil {
		return nil, err
	}
	return artifact.Marshal(handle)
}
