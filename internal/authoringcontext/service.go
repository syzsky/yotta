// Package authoringcontext shares desktop observation between MCP and AI proposals.
package authoringcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	_ "image/png"
	"sync"
	"time"

	"github.com/yottaapp/yotta/internal/apperr"
	appcore "github.com/yottaapp/yotta/internal/application"
	"github.com/yottaapp/yotta/internal/automation/target"
	"github.com/yottaapp/yotta/internal/workflow/schema"
	"golang.org/x/image/draw"
)

type Editor struct {
	WorkflowID string `json:"workflowId"`
	GraphID    string `json:"graphId"`
	Dirty      bool   `json:"dirty"`
}
type TargetInfo struct {
	Slot    string `json:"slot"`
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	Adapter string `json:"adapter"`
}
type Targets interface {
	ResolveTarget(context.Context, string) (target.Target, error)
	CapturePNG(context.Context, string) ([]byte, error)
}
type Service struct {
	Application *appcore.Application
	Targets     Targets
	ListTargets func() []TargetInfo
	Screen      func(context.Context) (image.Image, image.Point, error)
	mu          sync.RWMutex
	editor      Editor
}

func (s *Service) SetEditor(editor Editor) { s.mu.Lock(); defer s.mu.Unlock(); s.editor = editor }
func (s *Service) Editor() Editor          { s.mu.RLock(); defer s.mu.RUnlock(); return s.editor }

type Context struct {
	Editor     Editor          `json:"editor"`
	WorkflowID string          `json:"workflowId"`
	Revision   int64           `json:"revision"`
	Source     json.RawMessage `json:"source"`
	Targets    []TargetInfo    `json:"targets"`
}

func (s *Service) Inspect(workflowID string) (Context, error) {
	if s == nil {
		return Context{}, problem("unavailable")
	}
	result := Context{Editor: s.Editor(), Targets: []TargetInfo{}}
	if s.ListTargets != nil {
		result.Targets = s.ListTargets()
	}
	if workflowID == "" {
		workflowID = result.Editor.WorkflowID
	}
	if workflowID == "" {
		return result, nil
	}
	snapshot, err := s.Application.GetSource(workflowID)
	if err != nil {
		return Context{}, problem("workflow_not_found")
	}
	result.WorkflowID, result.Revision, result.Source = workflowID, snapshot.Revision(), snapshot.Artifact()
	return result, nil
}

type CaptureRequest struct {
	WorkflowID string `json:"workflowId,omitempty"`
	Slot       string `json:"slot,omitempty"`
	Screen     bool   `json:"screen,omitempty"`
}
type CaptureInfo struct {
	Slot            string `json:"slot"`
	CoordinateSpace string `json:"coordinateSpace"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	SourceWidth     int    `json:"sourceWidth"`
	SourceHeight    int    `json:"sourceHeight"`
	OriginX         int    `json:"originX"`
	OriginY         int    `json:"originY"`
	CapturedAt      string `json:"capturedAt"`
}
type Capture struct {
	Info      CaptureInfo
	Data      []byte
	MediaType string
}
type ResolvedTarget struct {
	Slot   string `json:"slot"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (s *Service) Describe(ctx context.Context, slot string) (ResolvedTarget, error) {
	if s == nil || s.Targets == nil {
		return ResolvedTarget{}, problem("unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resolved, err := s.Targets.ResolveTarget(ctx, slot)
	if err != nil {
		return ResolvedTarget{}, problem("target_unavailable")
	}
	return ResolvedTarget{Slot: slot, Name: resolved.DisplayName, Kind: resolved.Kind, Width: resolved.Resolution.W, Height: resolved.Resolution.H}, nil
}
func (s *Service) Capture(ctx context.Context, request CaptureRequest) (Capture, error) {
	if s == nil {
		return Capture{}, problem("unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var frame image.Image
	var origin image.Point
	var err error
	space := "target"
	if request.Screen {
		if request.Slot != "" {
			return Capture{}, problem("invalid_capture")
		}
		if s.Screen == nil {
			return Capture{}, problem("screen_unavailable")
		}
		frame, origin, err = s.Screen(ctx)
		space = "screen"
	} else {
		if request.Slot == "" {
			current, inspectErr := s.Inspect(request.WorkflowID)
			if inspectErr != nil {
				return Capture{}, inspectErr
			}
			if current.Editor.WorkflowID == current.WorkflowID && current.Editor.Dirty {
				return Capture{}, problem("save_first")
			}
			var source schema.WorkflowSource
			if json.Unmarshal(current.Source, &source) != nil {
				return Capture{}, problem("default_target_missing")
			}
			for _, entry := range source.TargetDefaults {
				if entry.Target == "target" {
					request.Slot = entry.Slot
					break
				}
			}
			if request.Slot == "" {
				return Capture{}, problem("default_target_missing")
			}
		}
		if s.Targets == nil {
			return Capture{}, problem("unavailable")
		}
		var raw []byte
		raw, err = s.Targets.CapturePNG(ctx, request.Slot)
		if err == nil {
			config, _, decodeErr := image.DecodeConfig(bytes.NewReader(raw))
			if decodeErr != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 64_000_000 {
				return Capture{}, problem("invalid_image")
			}
			frame, _, err = image.Decode(bytes.NewReader(raw))
		}
	}
	if err != nil {
		return Capture{}, problem("capture_failed")
	}
	if ctx.Err() != nil {
		return Capture{}, problem("capture_failed")
	}
	if frame == nil {
		return Capture{}, problem("invalid_image")
	}
	bounds := frame.Bounds()
	info := CaptureInfo{Slot: request.Slot, CoordinateSpace: space, SourceWidth: bounds.Dx(), SourceHeight: bounds.Dy(), OriginX: origin.X, OriginY: origin.Y, CapturedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	// Keep images readable while staying within every provider's image budget.
	for limit := 1920; limit >= 480; limit /= 2 {
		w, h := bounds.Dx(), bounds.Dy()
		if max(w, h) > limit {
			scale := float64(limit) / float64(max(w, h))
			w = max(1, int(float64(w)*scale))
			h = max(1, int(float64(h)*scale))
		}
		resized := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(resized, resized.Bounds(), frame, bounds, draw.Src, nil)
		var encoded bytes.Buffer
		if jpeg.Encode(&encoded, resized, &jpeg.Options{Quality: 85}) != nil {
			return Capture{}, problem("invalid_image")
		}
		if encoded.Len() <= 1536<<10 {
			info.Width, info.Height = w, h
			return Capture{Info: info, Data: encoded.Bytes(), MediaType: "image/jpeg"}, nil
		}
	}
	return Capture{}, problem("invalid_image")
}
func problem(code string) error { return apperr.New("authoring.observation."+code, nil) }
