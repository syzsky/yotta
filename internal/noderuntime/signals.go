package noderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodeadapter"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/signals"
	"strings"
	"time"
)

func awaitSignal(ctx context.Context, inv nodeadapter.Invocation, ready <-chan struct{}, duration time.Duration) error {
	if inv.Await != nil {
		return inv.Await(ctx, ready, duration)
	}
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ready:
		return nil
	}
}
func signalAdapter(b nodes.Builtins, kind string) nodeadapter.Adapter {
	return func(ctx context.Context, inv nodeadapter.Invocation) (_ nodeadapter.AdapterResult, runErr error) {
		deferred := false
		effect := "https://schemas.yotta.dev/effects/signals/" + kind + "/v1"
		record := func(ctx context.Context, cause error) error {
			return recordAdapterOutcome(ctx, inv, nodeadapter.AdapterAction{EffectID: effect, Action: "signals." + kind, SummaryCode: "signals." + kind}, "signals.failed", cause)
		}
		defer func() {
			if !deferred {
				runErr = errors.Join(runErr, record(ctx, runErr))
			}
		}()
		fail := func(err error) error {
			code := "signals.failed"
			if errors.Is(err, signals.ErrOverflow) {
				code = "signals.queue_full"
			}
			if errors.Is(err, context.DeadlineExceeded) {
				code = "signals.wait_timeout"
				err = errors.New("signal wait elapsed")
			}
			if errors.Is(err, context.Canceled) {
				return err
			}
			return &nodeadapter.NodeFailure{Code: code, Output: "failed", Cause: err}
		}
		if inv.Signals == nil {
			return nodeadapter.AdapterResult{}, fail(errors.New("run signal bus unavailable"))
		}
		var name string
		if err := json.Unmarshal(inv.Inputs["name"].InlineJSON(), &name); err != nil || strings.TrimSpace(name) == "" || len(name) > 128 {
			return nodeadapter.AdapterResult{}, fail(errors.New("signal name must contain 1 to 128 bytes"))
		}
		if kind == "send" {
			inv.Signals.Publish(name, signals.Message{Name: name, EventID: uuid.NewString(), Value: inv.Inputs["value"].InlineJSON()})
			return nodeadapter.AdapterResult{ExecOutputs: []string{"completed"}, Outputs: map[string]datatype.ValueEnvelope{}}, nil
		}
		capacity := signals.Capacity
		if kind == "wait" {
			capacity = 1
		}
		sub, err := inv.Signals.Subscribe(name, "", capacity)
		if err != nil {
			return nodeadapter.AdapterResult{}, fail(err)
		}
		encode := func(m signals.Message) (map[string]datatype.ValueEnvelope, error) {
			var value any
			if len(m.Value) > 0 {
				if err := json.Unmarshal(m.Value, &value); err != nil {
					return nil, err
				}
			}
			out := map[string]datatype.ValueEnvelope{}
			for port, v := range map[string]any{"value": value, "event-id": m.EventID} {
				envelope, err := sealStateOutput(b, inv, port, v)
				if err != nil {
					return nil, err
				}
				out[port] = envelope
			}
			return out, nil
		}
		if kind == "listen" {
			initial, err := encode(signals.Message{})
			if err != nil {
				sub.Close()
				return nodeadapter.AdapterResult{}, fail(err)
			}
			deferred = true
			return nodeadapter.AdapterResult{Subscription: &nodeadapter.SubscriptionRequest{Initial: initial, Ready: sub.Ready(), Poll: func() (map[string]datatype.ValueEnvelope, bool, error) {
				m, ok, e := sub.Poll()
				if e != nil {
					return nil, false, fail(e)
				}
				if !ok {
					return nil, false, nil
				}
				out, e := encode(m)
				return out, true, e
			}, Close: func(ctx context.Context, cause error) error { sub.Close(); return record(ctx, cause) }}}, nil
		}
		defer sub.Close()
		duration := int64(0)
		if configured, ok := inv.Config["timeoutMs"]; ok {
			raw, _ := json.Marshal(configured)
			if err := json.Unmarshal(raw, &duration); err != nil {
				return nodeadapter.AdapterResult{}, fail(err)
			}
		}
		if err := awaitSignal(ctx, inv, sub.Ready(), time.Duration(duration)*time.Millisecond); err != nil {
			if ctx.Err() != nil {
				return nodeadapter.AdapterResult{}, ctx.Err()
			}
			return nodeadapter.AdapterResult{}, fail(err)
		}
		message, ok, err := sub.Poll()
		if err != nil {
			return nodeadapter.AdapterResult{}, fail(err)
		}
		if !ok {
			return nodeadapter.AdapterResult{}, fail(signals.ErrClosed)
		}
		outputs, err := encode(message)
		return nodeadapter.AdapterResult{ExecOutputs: []string{"completed"}, Outputs: outputs}, err
	}
}
