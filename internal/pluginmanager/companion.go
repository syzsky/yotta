package pluginmanager

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/sdk/plugin/packaging"
)

var companionHTTP = &http.Client{Timeout: 750 * time.Millisecond, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func health(c packaging.Companion) (bool, string) {
	response, err := companionHTTP.Get(c.Origin + c.HealthPath)
	if err != nil {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return false, "unresponsive"
		}
		return false, "stopped"
	}
	defer response.Body.Close()
	var value struct {
		Protocol string `json:"protocol"`
		Status   string `json:"status"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 65537)).Decode(&value) != nil || value.Protocol != c.Protocol {
		return false, "conflict"
	}
	return true, value.Status
}
func stop(c packaging.Companion) error {
	running, status := health(c)
	if !running {
		if status != "stopped" {
			return errors.New("companion endpoint belongs to another service")
		}
		return nil
	}
	response, err := companionHTTP.Post(c.Origin+c.StopPath, "application/json", nil)
	if err != nil {
		return err
	}
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("companion rejected shutdown")
	}
	for i := 0; i < 30; i++ {
		running, status = health(c)
		if !running && status == "stopped" {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("companion shutdown timed out")
}
func (m *Manager) Control(id, companionID string, start bool) error {
	return m.change(func() error {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.controlLocked(context.Background(), id, companionID, start)
	})
}

// controlLocked is shared by explicit controls and Run preparation; the caller owns m.mu.
func (m *Manager) controlLocked(ctx context.Context, id, companionID string, start bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r, ok := m.records[id]
	if !ok {
		return apperr.New("plugins.not_found", nil)
	}
	if !start {
		if err := m.idle(id); err != nil {
			return err
		}
	}
	for _, c := range r.descriptor.Companions {
		if c.ID != companionID {
			continue
		}
		if !start {
			return wrapIf("plugins.stop_failed", stop(c))
		}
		enabled := false
		for _, p := range m.store.List() {
			if p.PackageID == id {
				enabled = p.Enabled
			}
		}
		if !enabled {
			return apperr.New("plugins.disabled", nil)
		}
		running, status := health(c)
		if running {
			return nil
		}
		if status != "stopped" {
			return apperr.New("plugins.endpoint_conflict", nil)
		}
		// Reverify packaged executable/resource bytes once before process launch.
		if _, _, err := m.store.InstalledManifest(context.Background(), id); err != nil {
			return problem("plugins.invalid_package", err)
		}
		exe, args := companionPaths(c, r.root)
		cmd := exec.Command(exe, args...)
		hideProcess(cmd)
		if err := cmd.Start(); err != nil {
			return problem("plugins.start_failed", err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		for i := 0; i < 60; i++ {
			running, _ = health(c)
			if running {
				return nil
			}
			select {
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				<-done
				return ctx.Err()
			case err := <-done:
				return problem("plugins.start_failed", err)
			case <-time.After(100 * time.Millisecond):
			}
		}
		_ = cmd.Process.Kill()
		<-done
		return apperr.New("plugins.start_failed", nil)
	}
	return apperr.New("plugins.not_found", nil)
}
