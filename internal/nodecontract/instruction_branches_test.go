package nodecontract

import "testing"

func branchDraftForTest() Draft {
	d := concatContractDraftForTest()
	d.Execution.Class = ExecutionEffect
	d.Execution.Evaluation = EvaluationPush
	d.Execution.Cache = CacheNone
	d.Execution.Effects = []EffectID{"https://schemas.yotta.dev/effects/test/branch/v1"}
	d.Ports.ExecInputs = []SignalPort{{ID: "in"}}
	d.Ports.ExecOutputs = []SignalPort{{ID: "moving"}, {ID: "marker"}, {ID: "recover"}, {ID: "done"}}
	d.Instruction = Invoke()
	d.Instruction.Invoke.Branches = []BranchInstruction{{Output: "moving", Coalesce: true}, {Output: "marker"}, {Output: "recover"}}
	return d
}

func TestInvocationBranchesAreSemanticAndRoundTrip(t *testing.T) {
	draft := branchDraftForTest()
	c, err := Seal(draft)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(c.Bytes())
	if err != nil || opened.NodeRef() != c.NodeRef() || len(opened.Machine().Instruction.Invoke.Branches) != 3 {
		t.Fatalf("branch contract roundtrip: %v", err)
	}
	draft.Instruction.Invoke.Branches[0].Coalesce = false
	changed, err := Seal(draft)
	if err != nil || changed.NodeRef().SemanticDigest == c.NodeRef().SemanticDigest {
		t.Fatalf("coalescing did not change semantic identity: %v", err)
	}
}

func TestInvocationBranchesRejectInvalidDeclarations(t *testing.T) {
	for name, mutate := range map[string]func(*Draft){
		"unknown":      func(d *Draft) { d.Instruction.Invoke.Branches[0].Output = "missing" },
		"empty":        func(d *Draft) { d.Instruction.Invoke.Branches[0].Output = "" },
		"duplicate":    func(d *Draft) { d.Instruction.Invoke.Branches[1].Output = "moving" },
		"subscription": func(d *Draft) { d.Instruction.Invoke.Subscription = &SubscriptionInstruction{} },
		"control":      func(d *Draft) { d.Execution.Class = ExecutionControl },
		"pull":         func(d *Draft) { d.Execution.Evaluation = EvaluationPull },
	} {
		t.Run(name, func(t *testing.T) {
			d := branchDraftForTest()
			mutate(&d)
			if _, err := Seal(d); err == nil {
				t.Fatal("accepted invalid branch declaration")
			}
		})
	}
}
