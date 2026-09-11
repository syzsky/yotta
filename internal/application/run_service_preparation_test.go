package application_test

import (
	"context"
	"github.com/yottaapp/yotta/internal/apperr"
	appcore "github.com/yottaapp/yotta/internal/application"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/targetruntime"
	"testing"
	"time"
)

func TestRunServicePreparationFailureHasDurableIdentity(t *testing.T) {
	app, _, _, _, _, _ := newTestApplication(t, time.Now().UTC(), nil)
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer app.Close(context.Background())
	saved := createConcatWorkflow(t, app)
	calls := 0
	app.SetRunServicePreparer(func(context.Context, []string, targetruntime.Snapshot) ([]string, error) {
		calls++
		return nil, apperr.New("plugins.start_failed", nil)
	})
	result, err := app.StartRun(context.Background(), appcore.StartRunRequest{WorkflowID: saved.Source.WorkflowID(), Principal: "user-1"})
	if err == nil || calls != 1 || !result.Record.Valid() {
		t.Fatalf("result=%+v calls=%d err=%v", result, calls, err)
	}
	stored, err := app.GetRun(result.Record.Admission().RunID)
	if err != nil || stored.Status() != run.StatusFailed {
		t.Fatalf("status=%s err=%v", stored.Status(), err)
	}
	failure, ok := stored.Failure()
	if !ok || failure.Code != "plugins.start_failed" {
		t.Fatalf("failure=%+v", failure)
	}
	if active := app.ActiveSourceRuns(saved.Source.WorkflowID()); len(active) != 0 {
		t.Fatalf("failed preparation queued jobs: %v", active)
	}
}
