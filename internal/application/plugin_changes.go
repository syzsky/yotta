package application

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/apperr"
	run "github.com/yottaapp/yotta/internal/run"
	"github.com/yottaapp/yotta/internal/targetruntime"
)

func (a *Application) failRunPreparation(ctx context.Context, record run.Record, cause error) (run.Record, error) {
	cleanup := context.WithoutCancel(ctx)
	at := a.transitionTime(record.Admission().QueuedAt)
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		next, err := record.Cancel(at)
		if err != nil {
			return record, err
		}
		if err := a.runs.Update(cleanup, record.Digest(), next); err != nil {
			return record, err
		}
		a.emit(next, cause)
		return next, nil
	}
	running, err := record.Start(at)
	if err != nil {
		return record, err
	}
	if err := a.runs.Update(cleanup, record.Digest(), running); err != nil {
		return record, err
	}
	problem := apperr.Project(cause)
	next, err := running.Fail(at, run.RunError{Code: problem.ID, Category: run.ErrorCategoryInfrastructure})
	if err != nil {
		return running, err
	}
	if err := a.runs.Update(cleanup, running.Digest(), next); err != nil {
		return running, err
	}
	a.emit(next, cause)
	return next, nil
}

// SetRunServicePreparer installs application-owned service preparation once per Run.
// It runs under the plugin read lock, before the Run is published to workers.
func (a *Application) SetRunServicePreparer(prepare func(context.Context, []string, targetruntime.Snapshot) ([]string, error)) {
	a.pluginMu.Lock()
	defer a.pluginMu.Unlock()
	a.prepareRunServices = prepare
}

// WithPluginChange serializes package mutations against new Run preparation.
// Running jobs are checked by the caller and keep their existing resources.
func (a *Application) WithPluginChange(change func() error) error {
	a.pluginMu.Lock()
	defer a.pluginMu.Unlock()
	return change()
}

func (a *Application) SetPluginValidator(validate func([]byte) ([]string, error)) {
	a.pluginMu.Lock()
	defer a.pluginMu.Unlock()
	a.validatePlugins = validate
}

// PluginInUse uses the immutable package set captured when the Run was queued,
// even if its editable Source has since removed the plugin node.
func (a *Application) PluginInUse(id string) bool {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	for _, job := range a.jobs {
		for _, p := range job.packageIDs {
			if p == id {
				return true
			}
		}
	}
	return false
}
