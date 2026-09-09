package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCodexSubscriptionToolImageSmoke(t *testing.T) {
	if os.Getenv("YOTTA_CODEX_SMOKE") != "1" {
		t.Skip("set YOTTA_CODEX_SMOKE=1 to use the signed-in Codex subscription")
	}
	profile, err := SealModelProfile(ModelProfileDraft{
		Provider: ProviderCodexSubscription, Endpoint: "codex://subscription", Model: "gpt-5.6-luna",
		MaxOutputTokens: 128, Capabilities: ProfileCapabilities{StructuredOutput: true, ToolCalling: true, ParallelTools: true},
		Evaluation: EvaluationUnverified,
	})
	if err != nil {
		var failure *ProviderFailure
		if errors.As(err, &failure) {
			t.Fatal(failure.Message)
		}
		t.Fatal(err)
	}
	toolSet, err := SealToolSet(ToolSetDraft{ID: "yotta.ai.codex-smoke-tools", Version: "1.0.0", Owner: "test", Tools: []ToolManifestDraft{{
		Name: "echo", Description: "Echo one value for this required smoke test.", Authority: ToolAuthorityPure,
		InputSchema:  json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`),
	}}})
	if err != nil {
		var failure *ProviderFailure
		if errors.As(err, &failure) {
			t.Fatal(failure.Message)
		}
		t.Fatal(err)
	}
	manifest, err := SealPromptManifest(PromptManifestDraft{ID: "yotta.ai.codex-tool-smoke", Version: "1.0.0", Owner: "test", Instructions: "You must call echo exactly once. Then inspect the returned image and name its left color and right color in English, in that order."})
	if err != nil {
		var failure *ProviderFailure
		if errors.As(err, &failure) {
			t.Fatal(failure.Message)
		}
		t.Fatal(err)
	}
	prompt, err := RenderPrompt(manifest, []PromptBlock{{Kind: PromptBlockUser, Content: "Call echo with value image once, then inspect the returned image and name the left color and right color in English."}})
	if err != nil {
		var failure *ProviderFailure
		if errors.As(err, &failure) {
			t.Fatal(failure.Message)
		}
		t.Fatal(err)
	}
	provider, err := newCodexProvider(profile)
	if err != nil {
		var failure *ProviderFailure
		if errors.As(err, &failure) {
			t.Fatal(failure.Message)
		}
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	maximum := int64(64)
	first, state, err := provider.StartAgent(ctx, "", AgentStartRequest{
		AttemptID: "codex-tool-start", Prompt: prompt,
		ToolSet: ToolSetArtifact{Digest: toolSet.Digest(), Manifest: json.RawMessage(toolSet.Bytes())},
		Limits:  GenerationLimits{MaxOutputTokens: &maximum}, MaxParallelism: 1, Retention: RetentionNoApplicationState,
	})
	if err != nil {
		var failure *ProviderFailure
		if errors.As(err, &failure) {
			t.Fatal(failure.Message)
		}
		t.Fatal(err)
	}
	if first.Finish.Kind != FinishToolCalls || len(first.Items) != 1 || first.Items[0].ToolCall == nil {
		t.Fatalf("first outcome = %#v", first)
	}
	final := first
	next := state
	for turn := 0; turn < 4 && final.Finish.Kind == FinishToolCalls; turn++ {
		call := final.Items[0].ToolCall
		final, next, err = provider.ContinueAgent(ctx, "", next, AgentContinueRequest{AttemptID: fmt.Sprintf("codex-image-%d", turn), Results: []ToolResult{{CallID: call.CallID, Name: call.Name, Value: json.RawMessage(`{"value":"image attached; inspect it now"}`), Image: smokeImage(t)}}})
		if err != nil {
			var failure *ProviderFailure
			if errors.As(err, &failure) {
				t.Fatal(failure.Message)
			}
			t.Fatal(err)
		}
	}
	if next != nil || final.Finish.Kind != FinishCompleted {
		t.Fatalf("image tool did not complete: %#v", final)
	}
	text := strings.ToLower(joinText(final.Items))
	if !strings.Contains(text, "red") || !strings.Contains(text, "blue") || strings.Index(text, "red") > strings.Index(text, "blue") {
		t.Fatalf("model did not see image: %s", text)
	}
	t.Logf("model saw image: %s", text)
}

func smokeImage(t *testing.T) *ImageInput {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	draw.Draw(img, image.Rect(0, 0, 100, 100), &image.Uniform{C: color.RGBA{R: 255, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(100, 0, 200, 100), &image.Uniform{C: color.RGBA{B: 255, A: 255}}, image.Point{}, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		var failure *ProviderFailure
		if errors.As(err, &failure) {
			t.Fatal(failure.Message)
		}
		t.Fatal(err)
	}
	return &ImageInput{MediaType: "image/png", Data: out.Bytes()}
}
