package panels

import (
	"github.com/yottaapp/yotta/internal/panel"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
)

type Service struct{ panels *panel.Service }

func NewService(p *panel.Service) *Service                   { return &Service{panels: p} }
func (s *Service) List() []panel.Source                      { return s.panels.List() }
func (s *Service) Read(id string) (contract.Snapshot, error) { return s.panels.Read(id) }
func (s *Service) Dispatch(id string, event contract.Event) (contract.Result, error) {
	return s.panels.Dispatch(id, event)
}

func (s *Service) Save(d panel.Draft) (panel.Draft, error) { return s.panels.Save(d) }
func (s *Service) Edit(id string) (panel.Draft, error)     { return s.panels.Edit(id) }
func (s *Service) Delete(id string, revision uint64) error { return s.panels.Delete(id, revision) }
func (s *Service) Show(id string) error                    { return s.panels.Show(id) }
func (s *Service) Selected() string                        { return s.panels.Selected() }
