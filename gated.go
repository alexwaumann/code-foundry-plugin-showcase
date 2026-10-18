package main

import (
	"context"
	"fmt"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

// gated is the tiny thread surface offered only where showcase.on holds (the Host
// tab's "Turn gate on"): proof that context/set feeds when clauses.
type gated struct {
	s    *showcase
	open pluginsdk.SurfaceOpen
}

func (s *showcase) newGated(_ context.Context, h *pluginsdk.Host, open pluginsdk.SurfaceOpen) (pluginsdk.Surface, error) {
	d := &gated{s: s, open: open}
	s.mu.Lock()
	if s.h == nil {
		s.h = h
	}
	s.gateds[open.Instance] = d
	s.mu.Unlock()
	return d, d.render()
}

func (d *gated) render() error {
	d.s.mu.Lock()
	thread := d.open.Context.ThreadID
	on, global, at := d.s.gates.Threads[thread], d.s.gates.Global, d.s.gates.SetAt
	h := d.s.h
	d.s.mu.Unlock()
	why := "showcase.on is true for this thread"
	if !on && global {
		why = "showcase.on is true everywhere (global fact)"
	}
	fact := string(d.open.Facts["showcase.on"])
	if fact == "" {
		fact = "(not in facts)"
	}
	return h.UpdateView(d.open.Instance, ui.View{
		Header: &ui.Header{Title: "Gated", Pill: ui.NewPill("gate on", ui.ToneOK), Subline: ui.Plain("Offered while showcase.on holds")},
		Nodes: []ui.Node{
			ui.NewText("gated-why", ui.Plain(why+".")),
			ui.NewPairs("gated-facts",
				ui.Pair{Key: "gated-thread", Label: "Thread", Value: ui.Spans(ui.T(thread).AsMono()), Copyable: true},
				ui.NewPair("gated-fact", "Fact at open", ui.Spans(ui.T(fact).AsMono())),
				ui.NewPair("gated-at", "Gate changed", ui.Spans(ui.At(at, ui.TimeRelative)))),
			ui.NewActions("gated-bar",
				ui.CommandButton("gated-off", "Turn gate off", "plugin.showcase.gate", map[string]string{"state": "off"}).WithIcon("lock"),
				ui.CommandButton("gated-ping", "Ping", "plugin.showcase.ping", nil).WithIcon("zap")),
		},
	})
}

func (d *gated) Act(context.Context, pluginsdk.SurfaceAct) error {
	return fmt.Errorf("the gated tab has only command buttons")
}

func (d *gated) Visibility(bool) {}

func (d *gated) Close() {
	d.s.mu.Lock()
	delete(d.s.gateds, d.open.Instance)
	d.s.mu.Unlock()
}
