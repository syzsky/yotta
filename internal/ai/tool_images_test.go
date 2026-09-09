package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCodexForwardsTrustedAuthoringInstructions(t *testing.T) {
	out := &bufferCloser{}
	client := &codexClient{stdin: out, scanner: bufio.NewScanner(strings.NewReader("{\"id\":1,\"result\":{\"thread\":{\"id\":\"thread-1\"},\"model\":\"test\"}}\n"))}
	_, _, err := client.startThread(ModelProfileDraft{Model: "test"}, nil, "Inspect the screenshot before proposing.")
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		Params map[string]any `json:"params"`
	}
	if err = json.Unmarshal(out.Bytes(), &request); err != nil {
		t.Fatal(err)
	}
	if request.Params["developerInstructions"] != "Inspect the screenshot before proposing." {
		t.Fatal("trusted instructions were lost")
	}
}

type bufferCloser struct{ bytes.Buffer }

func (*bufferCloser) Close() error { return nil }

func TestCodexToolImageUsesImageContent(t *testing.T) {
	out := &bufferCloser{}
	client := &codexClient{stdin: out}
	result := ToolResult{CallID: "call-1", Name: "capture", Value: json.RawMessage(`{"width":1}`), Image: &ImageInput{MediaType: "image/png", Data: []byte("image-bytes")}}
	if err := client.respondTool(json.RawMessage(`1`), result); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			ContentItems []struct {
				Type     string `json:"type"`
				ImageURL string `json:"imageUrl"`
			} `json:"contentItems"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Result.ContentItems) != 2 || response.Result.ContentItems[1].Type != "inputImage" || !strings.HasPrefix(response.Result.ContentItems[1].ImageURL, "data:image/png;base64,") {
		t.Fatalf("response=%s", out.String())
	}
}

func TestNativeAgentToolImagesUseMultimodalContent(t *testing.T) {
	for _, kind := range []ProviderKind{ProviderOpenAIResponses, ProviderAnthropicMessages} {
		t.Run(string(kind), func(t *testing.T) {
			var continuation []byte
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				body, _ := io.ReadAll(r.Body)
				if requests == 2 {
					continuation = body
				}
				if kind == ProviderOpenAIResponses {
					if requests == 1 {
						_, _ = w.Write([]byte(`{"id":"resp_1","model":"model-snapshot-1","status":"completed","output":[{"type":"function_call","name":"echo","call_id":"call-1","arguments":"{\"value\":\"ok\"}"}],"usage":{"input_tokens":10,"output_tokens":5}}`))
					} else {
						_, _ = w.Write([]byte(`{"id":"resp_2","model":"model-snapshot-1","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":5}}`))
					}
				} else {
					if requests == 1 {
						_, _ = w.Write([]byte(`{"id":"msg_1","model":"model-snapshot-1","stop_reason":"tool_use","content":[{"type":"tool_use","id":"call-1","name":"echo","input":{"value":"ok"}}],"usage":{"input_tokens":10,"output_tokens":5}}`))
					} else {
						_, _ = w.Write([]byte(`{"id":"msg_2","model":"model-snapshot-1","stop_reason":"end_turn","content":[{"type":"text","text":"seen"}],"usage":{"input_tokens":10,"output_tokens":5}}`))
					}
				}
			}))
			defer server.Close()
			provider := nativeAgentProviderForTest(t, kind, server.URL)
			_, state, err := provider.StartAgent(context.Background(), "test", agentStartForTest(t))
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = provider.ContinueAgent(context.Background(), "test", state, AgentContinueRequest{AttemptID: "attempt-2", Results: []ToolResult{{CallID: "call-1", Name: "echo", Value: json.RawMessage(`{"value":"ok"}`), Image: &ImageInput{MediaType: "image/png", Data: []byte("pixels")}}}})
			if err != nil {
				t.Fatal(err)
			}
			expected := `"type":"input_image"`
			if kind == ProviderAnthropicMessages {
				expected = `"type":"image"`
			}
			if !bytes.Contains(continuation, []byte(expected)) {
				t.Fatalf("image missing: %s", continuation)
			}
		})
	}
}
