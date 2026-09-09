package aiauthoring_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/ai"
	"github.com/yottaapp/yotta/internal/aiauthoring"
)

func TestProposalRetainedVisualContextReachesReview(t *testing.T) {
	runtime := testRuntime(t, time.Now())
	source, err := runtime.Application.CreateSource(context.Background(), "Visual context")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := aiauthoring.NewManager(runtime.Application, runtime.Builtins, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := ai.SealModelProfile(ai.ModelProfileDraft{Provider: ai.ProviderOpenAIResponses, Model: "test-model", MaxOutputTokens: 8192, Capabilities: ai.ProfileCapabilities{ToolCalling: true}, Pricing: ai.TokenPricing{InputMicrounitsPerMillion: 1, OutputMicrounitsPerMillion: 1}, Evaluation: ai.EvaluationUnverified, ProviderMetadata: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	review, err := manager.Propose(context.Background(), aiauthoring.Runtime{Profile: profile, Provider: &visualUsageProvider{scriptedProvider: scriptedProvider{workflowID: source.WorkflowID()}}, Credential: "test"}, aiauthoring.ProposeRequest{WorkflowID: source.WorkflowID(), BaseRevision: source.Revision(), Instruction: "Create a reviewed change from the observed context.", TrustClass: "user-authored"})
	if err != nil {
		t.Fatal(err)
	}
	if review.Status != aiauthoring.StatusProposed || review.Usage.InputTokens != 320_000 {
		t.Fatalf("review: %+v", review)
	}
	after, err := runtime.Application.GetSource(source.WorkflowID())
	if err != nil || after.Hash() != source.Hash() {
		t.Fatal("proposal mutated workflow")
	}
}

// Two retained screenshots plus schemas can exceed 80k per model round. These
// inputs are cumulative billing usage, not the size of a single prompt.
type visualUsageProvider struct{ scriptedProvider }

func (p *visualUsageProvider) StartAgent(ctx context.Context, credential string, request ai.AgentStartRequest) (ai.Outcome, any, error) {
	out, state, err := p.scriptedProvider.StartAgent(ctx, credential, request)
	input := int64(80_000)
	out.Usage.InputTotal = &input
	return out, state, err
}
func (p *visualUsageProvider) ContinueAgent(ctx context.Context, credential string, state any, request ai.AgentContinueRequest) (ai.Outcome, any, error) {
	out, next, err := p.scriptedProvider.ContinueAgent(ctx, credential, state, request)
	input := int64(80_000)
	out.Usage.InputTotal = &input
	return out, next, err
}
