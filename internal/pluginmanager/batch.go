package pluginmanager

import "github.com/yottaapp/yotta/internal/apperr"

type BatchResult struct {
	ID        string           `json:"id"`
	Succeeded bool             `json:"succeeded"`
	Problem   *apperr.Envelope `json:"problem,omitempty"`
}

// Batch applies the existing lifecycle boundary to each distinct package.
// One failure does not hide or discard the results of other selections.
func (m *Manager) Batch(action string, ids []string) ([]BatchResult, error) {
	if (action != "enable" && action != "disable" && action != "uninstall") || len(ids) == 0 || len(ids) > 4096 {
		return nil, apperr.New("plugins.batch_invalid", nil)
	}
	for _, id := range ids {
		if id == "" {
			return nil, apperr.New("plugins.batch_invalid", nil)
		}
	}
	results := make([]BatchResult, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		var err error
		switch action {
		case "enable":
			err = m.SetEnabled(id, true)
		case "disable":
			err = m.SetEnabled(id, false)
		case "uninstall":
			err = m.Uninstall(id)
		}
		result := BatchResult{ID: id, Succeeded: err == nil}
		if err != nil {
			envelope := apperr.Project(err)
			result.Problem = &envelope
		}
		results = append(results, result)
	}
	return results, nil
}
