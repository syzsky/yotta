package tools

import "github.com/yottaapp/yotta/internal/apperr"

// OpenPathEditor reuses one recording workbench so F6 and samples have one owner.
func (s *Service) OpenPathEditor(guid string) error {
	p := s.windowPresenter()
	if p == nil {
		return apperr.New(apperr.CodeWailsNotReady, nil)
	}
	w, opened, err := s.openWindow(p, &s.pathEditor, WindowRequest{Kind: WindowPathEditor, GUID: guid}, s.onPathEditorClose)
	if err != nil {
		return toolError("tools.window_open_failed", apperr.CategoryInfrastructure, map[string]any{"window": "path_editor"}, true, err)
	}
	if !opened && w != nil {
		w.Show()
		w.Focus()
		p.Emit("path-editor:open", guid)
	}
	return nil
}
func (s *Service) ClosePathEditor() error {
	if w := s.invalidateWindow(&s.pathEditor); w != nil {
		if s.onPathEditorClose != nil {
			s.onPathEditorClose()
		}
		w.Close()
	}
	return nil
}
func (s *Service) SetPathEditorAlwaysOnTop(on bool) error {
	if w := s.currentWindow(&s.pathEditor); w != nil {
		w.SetAlwaysOnTop(on)
	}
	return nil
}

// OpenPathSettings keeps the recording draft in its own window.
func (s *Service) OpenPathSettings(section string) error {
	p := s.windowPresenter()
	if p == nil {
		return apperr.New(apperr.CodeWailsNotReady, nil)
	}
	if err := p.ShowMain(); err != nil {
		return toolError("tools.window_open_failed", apperr.CategoryInfrastructure, map[string]any{"window": "main"}, true, err)
	}
	if section != "hotkeys" {
		section = "plugins"
	}
	p.Emit("main:navigate", map[string]string{"path": "/settings", "section": section})
	return nil
}
