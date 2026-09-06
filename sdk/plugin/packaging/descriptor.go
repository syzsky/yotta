// Package packaging builds complete, signed plugin archives for desktop import.
package packaging

const DescriptorPath = "yotta-plugin.json"

// Descriptor is a signed payload inside the existing Node Package format.
// Messages are flat locale keys owned by this package's node contracts.
type Descriptor struct {
	Format      string                       `json:"format"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	PublicKey   string                       `json:"publicKey"`
	Messages    map[string]map[string]string `json:"messages,omitempty"`
	Companions  []Companion                  `json:"companions,omitempty"`
}

// Companion declares a cooperative local background service and its configured
// application/network slots. Arguments beginning with ${package}/ resolve
// against the verified installation, never the caller's working directory.
type Companion struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Executable      string   `json:"executable"`
	Arguments       []string `json:"arguments"`
	ApplicationSlot string   `json:"applicationSlot"`
	NetworkSlot     string   `json:"networkSlot"`
	Origin          string   `json:"origin"`
	HealthPath      string   `json:"healthPath"`
	StopPath        string   `json:"stopPath"`
	Protocol        string   `json:"protocol"`
}
