package aiauthoring_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/yottaapp/yotta/internal/ai"
	"github.com/yottaapp/yotta/internal/aiauthoring"
	"github.com/yottaapp/yotta/internal/authoringcontext"
	"image"
	"testing"
	"time"
)

type observationProvider struct {
	t          *testing.T
	turn       int
	workflowID string
}

func (p *observationProvider) StartAgent(context.Context, string, ai.AgentStartRequest) (ai.Outcome, any, error) {
	return toolOutcome("context", "authoring_context", `{}`), 1, nil
}
func (p *observationProvider) ContinueAgent(_ context.Context, _ string, _ any, request ai.AgentContinueRequest) (ai.Outcome, any, error) {
	p.turn++
	if p.turn == 1 {
		var output map[string]string
		if json.Unmarshal(request.Results[0].Value, &output) != nil {
			p.t.Fatal("invalid context")
		}
		var current authoringcontext.Context
		if json.Unmarshal([]byte(output["contextJson"]), &current) != nil || current.WorkflowID != p.workflowID || current.Editor.WorkflowID != p.workflowID {
			p.t.Fatal("proposal context escaped workflow scope")
		}
		return toolOutcome("capture", "automation_capture", `{"slot":"","screen":true}`), 2, nil
	}
	result := request.Results[0]
	if result.Image == nil {
		p.t.Fatal("image was not delivered to provider")
	}
	if _, _, err := image.Decode(bytes.NewReader(result.Image.Data)); err != nil {
		p.t.Fatal(err)
	}
	return answerProvider{}.StartAgent(context.Background(), "", ai.AgentStartRequest{})
}
func TestProposalSeesImageWithoutMutatingWorkflow(t *testing.T) {
	now := time.Now()
	runtime := testRuntime(t, now)
	created, err := runtime.Application.CreateSource(context.Background(), "Visual proposal")
	if err != nil {
		t.Fatal(err)
	}
	observation := &authoringcontext.Service{Application: runtime.Application, Screen: func(context.Context) (image.Image, image.Point, error) {
		return image.NewRGBA(image.Rect(0, 0, 320, 200)), image.Point{}, nil
	}}
	observation.SetEditor(authoringcontext.Editor{WorkflowID: "another-workflow", Dirty: true})
	manager, err := aiauthoring.NewManager(runtime.Application, runtime.Builtins, time.Now, observation)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := ai.SealModelProfile(ai.ModelProfileDraft{Provider: ai.ProviderOpenAIResponses, Model: "test-model", MaxOutputTokens: 1024, Capabilities: ai.ProfileCapabilities{ToolCalling: true}, Pricing: ai.TokenPricing{InputMicrounitsPerMillion: 1, OutputMicrounitsPerMillion: 1}, Evaluation: ai.EvaluationUnverified, ProviderMetadata: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	provider := &observationProvider{t: t, workflowID: created.WorkflowID()}
	review, err := manager.Propose(context.Background(), aiauthoring.Runtime{Profile: profile, Provider: provider, Credential: "test"}, aiauthoring.ProposeRequest{WorkflowID: created.WorkflowID(), BaseRevision: created.Revision(), Instruction: "Look at my screen.", TrustClass: "user-authored", AllowAnswerOnly: true})
	if err != nil || review.Summary == "" || provider.turn != 2 {
		t.Fatalf("review=%+v error=%v", review, err)
	}
	after, err := runtime.Application.GetSource(created.WorkflowID())
	if err != nil || after.Hash() != created.Hash() {
		t.Fatal("observation changed workflow")
	}
}
