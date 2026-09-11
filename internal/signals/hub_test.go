package signals

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestBroadcastOrderIsolationAndNoReplay(t *testing.T) {
	var a, b Hub
	a.Publish("next", Message{Value: json.RawMessage(`0`)})
	x, _ := a.Subscribe("next", "", Capacity)
	y, _ := a.Subscribe("next", "", Capacity)
	z, _ := b.Subscribe("next", "", Capacity)
	for i := 0; i < 3; i++ {
		v, _ := json.Marshal(i)
		a.Publish("next", Message{Value: v})
	}
	for _, s := range []*Subscription{x, y} {
		for i := 0; i < 3; i++ {
			m, ok, e := s.Poll()
			var got int
			json.Unmarshal(m.Value, &got)
			if e != nil || !ok || got != i {
				t.Fatalf("%v %v %d", e, ok, got)
			}
		}
	}
	if _, ok, _ := z.Poll(); ok {
		t.Fatal("cross-run signal")
	}
	x.Close()
	y.Close()
	a.Close()
	b.Close()
}
func TestOverflowIsExplicitAndSingleWaitConsumesOnlyFirst(t *testing.T) {
	var h Hub
	slow, _ := h.Subscribe("x", "", Capacity)
	once, _ := h.Subscribe("x", "", 1)
	for i := 0; i <= Capacity; i++ {
		v, _ := json.Marshal(i)
		h.Publish("x", Message{Value: v})
	}
	if _, _, e := slow.Poll(); !errors.Is(e, ErrOverflow) {
		t.Fatal(e)
	}
	m, ok, e := once.Poll()
	if e != nil || !ok || string(m.Value) != "0" {
		t.Fatalf("%+v %v", m, e)
	}
	next, _ := h.Subscribe("x", "", 1)
	if _, ok, _ := next.Poll(); ok {
		t.Fatal("old next click replayed")
	}
	h.Close()
}
