package application

// WithPluginChange serializes package mutations against new Run preparation.
// Running jobs are checked by the caller and keep their existing resources.
func (a *Application) WithPluginChange(change func() error) error {
	a.pluginMu.Lock()
	defer a.pluginMu.Unlock()
	return change()
}

func (a *Application) SetPluginValidator(validate func([]byte) ([]string, error)) {
	a.pluginMu.Lock()
	defer a.pluginMu.Unlock()
	a.validatePlugins = validate
}

// PluginInUse uses the immutable package set captured when the Run was queued,
// even if its editable Source has since removed the plugin node.
func (a *Application) PluginInUse(id string) bool {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	for _, job := range a.jobs {
		for _, p := range job.packageIDs {
			if p == id {
				return true
			}
		}
	}
	return false
}
