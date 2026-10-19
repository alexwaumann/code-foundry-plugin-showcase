package main

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

// The tick wake source (plugin.json's wakes): a thread subscribes with `code-foundry
// thread subscribe --plugin showcase/tick [--arg every=2s] [--repeat]` and is woken every
// <every> (default 10s), or once without --repeat. One loop per distinct every fires
// the source with that every as the match. The Host tab lists the live subscriptions as
// wake/subscribed and wake/unsubscribed tell them, and fires it now ("Fire tick", the
// fire-tick command). The host sends every live subscription again after each start.

const (
	wakeTick    = "tick"
	tickDefault = "10s"
	// maxTickFires is how many recent fires the Host tab lists.
	maxTickFires = 5
)

// tickState is the tick source's state; guarded by showcase.mu.
type tickState struct {
	subs  map[string]pluginsdk.WakeSubscription // live, by subscription id
	loops map[string]context.CancelFunc         // every → its loop
	n     map[string]int                        // every → ticks fired
	fires []tickFire                            // the latest, newest last
}

type tickFire struct {
	Every, Title, Err string
	Woke              int
	Manual            bool
	At                time.Time
}

func newTickState() tickState {
	return tickState{subs: map[string]pluginsdk.WakeSubscription{}, loops: map[string]context.CancelFunc{}, n: map[string]int{}}
}

func everyOf(s pluginsdk.WakeSubscription) string {
	if e := s.Args["every"]; e != "" {
		return e
	}
	return tickDefault
}

// wakeSubscribed arms a subscription (again after a restart: the same id is the same one).
func (s *showcase) wakeSubscribed(_ context.Context, h *pluginsdk.Host, sub pluginsdk.WakeSubscription) {
	if sub.Source != wakeTick {
		h.Log().Warn("subscription to an unknown wake source", "source", sub.Source, "subscription", sub.Subscription)
		return
	}
	every := everyOf(sub)
	d, err := time.ParseDuration(every)
	if err != nil || d <= 0 {
		h.Log().Warn("tick: bad every", "subscription", sub.Subscription, "every", every)
		return
	}
	s.mu.Lock()
	if s.h == nil {
		s.h = h
	}
	s.ticks.subs[sub.Subscription] = sub
	if _, ok := s.ticks.loops[every]; !ok && !s.closed {
		ctx, cancel := context.WithCancel(context.Background())
		s.ticks.loops[every] = cancel
		go s.tickLoop(ctx, h, every, d)
		h.Log().Info("tick armed", "every", every, "subscription", sub.Subscription)
	}
	s.mu.Unlock()
	s.renderSoon()
}

// wakeUnsubscribed disarms a subscription; its every's loop stops with the last one.
func (s *showcase) wakeUnsubscribed(_ context.Context, h *pluginsdk.Host, u pluginsdk.WakeUnsubscribed) {
	s.mu.Lock()
	sub, ok := s.ticks.subs[u.Subscription]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.ticks.subs, u.Subscription)
	every := everyOf(sub)
	last := true
	for _, o := range s.ticks.subs {
		if everyOf(o) == every {
			last = false
		}
	}
	if cancel, ok := s.ticks.loops[every]; ok && last {
		cancel()
		delete(s.ticks.loops, every)
		h.Log().Info("tick stopped", "every", every)
	}
	s.mu.Unlock()
	s.renderSoon()
}

func (s *showcase) tickLoop(ctx context.Context, h *pluginsdk.Host, every string, d time.Duration) {
	tk := time.NewTicker(d)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			if _, err := s.fireTick(ctx, h, every, false); err != nil {
				h.Log().Warn("tick: fire", "every", every, "err", err.Error())
			}
		}
	}
}

// stopTicksLocked ends every loop (the plugin is stopping); s.mu held.
func (s *showcase) stopTicksLocked() {
	for e, cancel := range s.ticks.loops {
		cancel()
		delete(s.ticks.loops, e)
	}
}

// fireTick fires tick for the subscriptions with every and says how many were woken.
func (s *showcase) fireTick(ctx context.Context, h *pluginsdk.Host, every string, manual bool) (int, error) {
	s.mu.Lock()
	s.ticks.n[every]++
	n := s.ticks.n[every]
	s.mu.Unlock()
	title := fmt.Sprintf("tick %d (every %s)", n, every)
	woke, err := h.FireWake(ctx, wakeTick, map[string]string{"every": every}, title,
		map[string]any{"n": n, "every": every, "manual": manual, "at": s.now().UTC().Format(time.RFC3339)})
	f := tickFire{Every: every, Title: title, Woke: woke, Manual: manual, At: s.now()}
	if err != nil {
		f.Err = err.Error()
	}
	s.mu.Lock()
	s.ticks.fires = append(s.ticks.fires, f)
	if len(s.ticks.fires) > maxTickFires {
		s.ticks.fires = s.ticks.fires[len(s.ticks.fires)-maxTickFires:]
	}
	s.mu.Unlock()
	s.renderSoon()
	return woke, err
}

// armedEvery are the every values with a subscription, sorted.
func (s *showcase) armedEvery() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, sub := range s.ticks.subs {
		if e := everyOf(sub); !slices.Contains(out, e) {
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

// fireTickCmd is the fire-tick command (and the Host tab's Fire tick button): fire tick
// now for one every, or for each armed one. Its message ends with the subscribe line.
func (s *showcase) fireTickCmd(ctx context.Context, h *pluginsdk.Host, req pluginsdk.CommandRequest) (pluginsdk.CommandResult, error) {
	targets := s.armedEvery()
	if e := strings.TrimSpace(req.Arg("every")); e != "" {
		if d, err := time.ParseDuration(e); err != nil || d <= 0 {
			return pluginsdk.CommandResult{}, pluginsdk.NewError(pluginsdk.CodeInvalidParams, "every: want a duration over zero (10s, 1m30s), got %q", e)
		}
		targets = []string{e}
	}
	if len(targets) == 0 {
		return pluginsdk.CommandResult{Message: "No thread subscribes to tick. Subscribe one with: " + h.WakeHint(wakeTick, map[string]string{"every": "2s"})}, nil
	}
	var lines []string
	for _, e := range targets {
		n, err := s.fireTick(ctx, h, e, true)
		if err != nil {
			return pluginsdk.CommandResult{}, fmt.Errorf("fire tick (every %s): %w", e, err)
		}
		lines = append(lines, fmt.Sprintf("Fired tick for every=%s: woke %d", e, n))
	}
	lines = append(lines, "Wait for the next one: "+h.WakeHint(wakeTick, map[string]string{"every": targets[0]}))
	return pluginsdk.CommandResult{Message: strings.Join(lines, "\n")}, nil
}

// wakeSection is the Host tab's tick section: the live subscriptions, the latest fires,
// Fire tick, and the line that subscribes. s.mu held.
func (s *showcase) wakeSectionLocked() ui.Node {
	ids := make([]string, 0, len(s.ticks.subs))
	for id := range s.ticks.subs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows := make([]ui.Row, 0, len(ids))
	for _, id := range ids {
		sub := s.ticks.subs[id]
		mode := "once"
		if sub.Repeat {
			mode = "repeat"
		}
		rows = append(rows, ui.Row{Key: "wsub-" + id, Glyph: ui.StatusGlyph(ui.StatusRunning), Title: "every " + everyOf(sub),
			TitleMeta: ui.Spans(ui.T(id).AsMono()),
			Line2:     ui.Spans(ui.T("thread "), ui.T(orDash(sub.Context.ThreadID)).AsMono(), ui.T(" · "+mode))})
	}
	nodes := []ui.Node{{Key: "list-wakes", List: &ui.List{Rows: rows, EmptyText: "No thread subscribes to tick"}}}
	if len(s.ticks.fires) > 0 {
		fires := make([]ui.Row, 0, len(s.ticks.fires))
		for i := len(s.ticks.fires) - 1; i >= 0; i-- {
			f := s.ticks.fires[i]
			r := ui.Row{Key: fmt.Sprintf("wfire-%d", i), Glyph: ui.StatusGlyph(ui.StatusOK), Title: f.Title,
				Line2: ui.Plain(fmt.Sprintf("woke %d", f.Woke)), Trailing: ui.Spans(ui.At(f.At, ui.TimeRelative))}
			if f.Manual {
				r.Line2 = ui.Plain(fmt.Sprintf("woke %d · fired by hand", f.Woke))
			}
			if f.Err != "" {
				r.Glyph, r.Line2 = ui.StatusGlyph(ui.StatusFailed), ui.Spans(ui.T(f.Err).WithTone(ui.ToneError))
			}
			fires = append(fires, r)
		}
		nodes = append(nodes, ui.Node{Key: "list-fires", List: &ui.List{Rows: fires}})
	}
	hint := pluginsdk.WakeHint("showcase", wakeTick, map[string]string{"every": "2s"})
	if s.h != nil {
		hint = s.h.WakeHint(wakeTick, map[string]string{"every": "2s"})
	}
	nodes = append(nodes,
		ui.NewActions("bar-wakes", ui.CommandButton("wake-fire", "Fire tick", "plugin.showcase.fire-tick", nil).WithIcon("bell").WithStyle(ui.ButtonOutline)),
		ui.Node{Key: "wake-hint", Text: &ui.Text{Size: ui.TextSm, Copyable: true,
			Content: ui.Spans(ui.T("Subscribe a thread: ").WithTone(ui.ToneMuted), ui.T(hint).AsMono())}})
	return ui.NewSection("sec-wakes", "Wake source: tick (FireWake)", nodes...)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
