package aiauthoring_test

import (
	"context"
	"github.com/yottaapp/yotta/internal/ai"
	"github.com/yottaapp/yotta/internal/aiauthoring"
	"github.com/yottaapp/yotta/internal/authoringcontext"
	"os"
	"testing"
	"time"
)

func TestVisualProposalLive(t *testing.T) {
	if os.Getenv("YOTTA_CODEX_SMOKE") != "1" {
		t.Skip("requires signed-in Codex and a desktop")
	}
	runtime := testRuntime(t, time.Now())
	source, err := runtime.Application.CreateSource(context.Background(), "视觉创作临时验收")
	if err != nil {
		t.Fatal(err)
	}
	observation := &authoringcontext.Service{Application: runtime.Application, Screen: authoringcontext.CaptureScreen}
	observation.SetEditor(authoringcontext.Editor{WorkflowID: source.WorkflowID()})
	manager, err := aiauthoring.NewManager(runtime.Application, runtime.Builtins, time.Now, observation)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := ai.SealModelProfile(ai.ModelProfileDraft{Provider: ai.ProviderCodexSubscription, Endpoint: "codex://subscription", Model: "gpt-5.6-luna", MaxOutputTokens: 2048, Capabilities: ai.ProfileCapabilities{ToolCalling: true, ParallelTools: true, StructuredOutput: true}, Evaluation: ai.EvaluationUnverified})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := ai.NewNativeProvider(profile, ai.HTTPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	review, err := manager.Propose(context.Background(), aiauthoring.Runtime{Profile: profile, Provider: &tracedVisualProvider{AgentProvider: provider.(ai.AgentProvider), t: t}, Credential: "subscription"}, aiauthoring.ProposeRequest{WorkflowID: source.WorkflowID(), BaseRevision: source.Revision(), TrustClass: "user-authored", Instruction: "请调用 authoring_context 和 automation_capture（slot 为空，screen 为 true），根据屏幕创建修改提案：增加一个文本拼接节点，输入 a 为一句画面摘要，输入 b 为空字符串。查询节点用法，完成编译和预览，不要运行或应用。", Progress: func(e aiauthoring.ConversationProgress) { t.Log(e.Kind, e.Facts) }})
	if err != nil {
		t.Fatal(err)
	}
	if review.Status != "proposed" || len(review.Changes) == 0 {
		t.Fatalf("no proposal: %+v", review)
	}
	t.Log("proposal", review.Summary, review.Usage)
}

type tracedVisualProvider struct {
	ai.AgentProvider
	t *testing.T
}

func (p *tracedVisualProvider) StartAgent(ctx context.Context, credential string, request ai.AgentStartRequest) (ai.Outcome, any, error) {
	out, state, err := p.AgentProvider.StartAgent(ctx, credential, request)
	p.log(out)
	return out, state, err
}
func (p *tracedVisualProvider) ContinueAgent(ctx context.Context, credential string, state any, request ai.AgentContinueRequest) (ai.Outcome, any, error) {
	out, next, err := p.AgentProvider.ContinueAgent(ctx, credential, state, request)
	p.log(out)
	return out, next, err
}
func (p *tracedVisualProvider) log(out ai.Outcome) {
	if out.Usage.InputTotal != nil {
		p.t.Log("tokens", *out.Usage.InputTotal, *out.Usage.OutputTotal)
	}
}
