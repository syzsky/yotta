package tools

import "github.com/yottaapp/yotta/internal/apperr"

func (s *Service) OpenPanels() error {
	presenter := s.windowPresenter()
	if presenter == nil {
		return apperr.New(apperr.CodeWailsNotReady, nil)
	}
	w, opened, err := s.openWindow(presenter, &s.panels, WindowRequest{Kind: WindowPanels}, nil)
	if err != nil {
		return toolError("tools.window_open_failed", apperr.CategoryInfrastructure, map[string]any{"window": "panels"}, true, err)
	}
	if !opened && w != nil {
		w.Show()
		w.Focus()
	}
	presenter.Emit("panels:visibility", true)
	return nil
}
func (s *Service) HidePanels() error {
	if p := s.windowPresenter(); p != nil {
		p.Emit("panels:visibility", false)
	}
	if w := s.currentWindow(&s.panels); w != nil {
		w.Hide()
	}
	return nil
}
func (s *Service) SetPanelsAlwaysOnTop(on bool) error {
	if w := s.currentWindow(&s.panels); w != nil {
		w.SetAlwaysOnTop(on)
	}
	return nil
}
