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
		w.SetIgnoreMouseEvents(false)
		w.Show()
		w.Focus()
	}
	presenter.Emit("panels:visibility", true)
	presenter.Emit("panels:click-through", false)
	return nil
}
func (s *Service) SetPanelsSize(width, height int) error {
	if w := s.currentWindow(&s.panels); w != nil {
		w.SetSize(max(240, min(width, 3840)), max(160, min(height, 2160)))
	}
	return nil
}
func (s *Service) SetPanelsClickThrough(on bool) error {
	if w := s.currentWindow(&s.panels); w != nil {
		w.SetIgnoreMouseEvents(on)
	}
	if p := s.windowPresenter(); p != nil {
		p.Emit("panels:click-through", on)
	}
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
