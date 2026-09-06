package pluginsdk

import "github.com/yottaapp/yotta/internal/artifact"

// MarshalJSON encodes protocol payloads using the host's canonical JSON rules.
func MarshalJSON(value any) ([]byte, error) { return artifact.Marshal(value) }
