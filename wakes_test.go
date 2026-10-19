package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/plugintest"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

// manifestWakes are plugin.json's wake sources, for plugintest.Options.Wakes (the fake
// then refuses a fire the host would).
func manifestWakes(t *testing.T) []pluginsdk.WakeSpec {
	t.Helper()
	data, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := pluginsdk.LoadManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	return m.Wakes
}

func TestManifestWakes(t *testing.T) {
	ws := manifestWakes(t)
	if len(ws) != 1 || ws[0].Name != wakeTick {
		t.Fatalf("wakes = %+v", ws)
	}
	if a, ok := ws[0].Arg("every"); !ok || a.TypeOr() != pluginsdk.ArgDuration || a.Default != tickDefault {
		t.Errorf("every arg = %+v", a)
	}
}

func (s *showcase) loopCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ticks.loops)
}

func (s *showcase) subCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ticks.subs)
}

// Subscribe, the loop fires with every as the match; a re-arm keeps one loop;
// unsubscribing stops it.
func TestTickWake(t *testing.T) {
	s := fast()
	h := start(t, s, plugintest.Options{Granted: every, Wakes: manifestWakes(t)})
	uctx := pluginsdk.UiContext{ThreadID: "s-1"}
	id, err := h.SubscribeWake(wakeTick, map[string]string{"every": "20ms"}, uctx)
	if err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return len(h.WakeFires()) >= 2 })
	f := h.WakeFires()[0]
	if f.Source != wakeTick || f.Match["every"] != "20ms" || f.Title != "tick 1 (every 20ms)" || !strings.Contains(string(f.Payload), `"n":1`) {
		t.Errorf("fire = %+v", f)
	}
	if h.WakeDelivered()[0] != 1 {
		t.Errorf("delivered %v", h.WakeDelivered())
	}
	if _, err := h.SubscribeWakeWith(pluginsdk.WakeSubscription{Subscription: id, Source: wakeTick, Args: map[string]string{"every": "20ms"}, Context: uctx, Repeat: true}); err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(*plugintest.Host) bool { return s.subCount() == 1 })
	if n := s.loopCount(); n != 1 {
		t.Errorf("%d loops after a re-arm", n)
	}
	if err := h.UnsubscribeWake(id); err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(*plugintest.Host) bool { return s.loopCount() == 0 && s.subCount() == 0 })
	n := len(h.WakeFires())
	time.Sleep(100 * time.Millisecond)
	if m := len(h.WakeFires()); m > n+1 {
		t.Errorf("still ticking after the last unsubscription: %d more fires", m-n)
	}
}

// A one-shot goes after its fire; its loop with it.
func TestTickOneShot(t *testing.T) {
	s := fast()
	h := start(t, s, plugintest.Options{Granted: every, Wakes: manifestWakes(t)})
	if _, err := h.SubscribeWakeWith(pluginsdk.WakeSubscription{Source: wakeTick, Args: map[string]string{"every": "20ms"}}); err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return len(h.WakeFires()) >= 1 && len(h.WakeSubscriptions()) == 0 })
	h.WaitFor(5*time.Second, func(*plugintest.Host) bool { return s.loopCount() == 0 && s.subCount() == 0 })
}

func TestFireTickCommand(t *testing.T) {
	ctx := testCtx(t)
	s := fast()
	h := start(t, s, plugintest.Options{Granted: every, Wakes: manifestWakes(t)})
	res, err := h.Command(ctx, "fire-tick", nil, pluginsdk.UiContext{})
	if err != nil || !strings.HasSuffix(res.Message, "code-foundry thread subscribe --plugin showcase/tick --arg every=2s") || len(h.WakeFires()) != 0 {
		t.Errorf("fire-tick with nobody subscribed = %q, %v", res.Message, err)
	}
	if _, err := h.SubscribeWake(wakeTick, map[string]string{"every": "1h"}, pluginsdk.UiContext{ThreadID: "s-1"}); err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(*plugintest.Host) bool { return s.subCount() == 1 })
	res, err = h.Command(ctx, "fire-tick", nil, pluginsdk.UiContext{})
	if err != nil || !strings.HasPrefix(res.Message, "Fired tick for every=1h: woke 1\n") ||
		!strings.HasSuffix(res.Message, "\nWait for the next one: code-foundry thread subscribe --plugin showcase/tick --arg every=1h") {
		t.Errorf("fire-tick = %q, %v", res.Message, err)
	}
	if f := h.WakeFires(); len(f) != 1 || f[0].Title != "tick 1 (every 1h)" || !strings.Contains(string(f[0].Payload), `"manual":true`) {
		t.Errorf("fires %+v", f)
	}
	if _, err := h.Command(ctx, "fire-tick", map[string]string{"every": "soon"}, pluginsdk.UiContext{}); pluginsdk.ErrorCode(err) != pluginsdk.CodeInvalidParams {
		t.Errorf("a bad every: %v", err)
	}
}

// The Host tab lists the live subscriptions and the fires, with Fire tick and the line
// that subscribes.
func TestHostTabWakes(t *testing.T) {
	ctx := testCtx(t)
	s := fast()
	h := start(t, s, plugintest.Options{Granted: every, Wakes: manifestWakes(t)})
	openGallery(t, h)
	v := switchTab(t, h, tabHost, pluginsdk.UiContext{ThreadID: "s-1"})
	if l := mustFind(t, v, "list-wakes").List; len(l.Rows) != 0 || l.EmptyText == "" {
		t.Errorf("subscriptions before any: %+v", l)
	}
	bar := mustFind(t, v, "bar-wakes").Actions.Buttons
	if len(bar) != 1 || bar[0].Action.Command == nil || bar[0].Action.Command.Name != "plugin.showcase.fire-tick" {
		t.Errorf("Fire tick button = %+v", bar)
	}
	if hint := mustFind(t, v, "wake-hint").Text.Content.Spans[1].Text; hint != "code-foundry thread subscribe --plugin showcase/tick --arg every=2s" {
		t.Errorf("hint %q", hint)
	}

	id, err := h.SubscribeWake(wakeTick, map[string]string{"every": "1h"}, pluginsdk.UiContext{ThreadID: "s-1"})
	if err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool {
		n := find(decode(t, h, inst).Nodes, "list-wakes")
		return n != nil && len(n.List.Rows) == 1
	})
	row := mustFind(t, decode(t, h, inst), "list-wakes").List.Rows[0]
	if row.Key != "wsub-"+id || row.Title != "every 1h" || !strings.Contains(fmt.Sprint(row.Line2.Spans), "s-1") {
		t.Errorf("subscription row %+v", row)
	}
	if _, err := h.Command(ctx, "fire-tick", nil, pluginsdk.UiContext{ThreadID: "s-1"}); err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool {
		n := find(decode(t, h, inst).Nodes, "list-fires")
		return n != nil && len(n.List.Rows) == 1 && n.List.Rows[0].Title == "tick 1 (every 1h)"
	})
	if err := h.UnsubscribeWake(id); err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool {
		n := find(decode(t, h, inst).Nodes, "list-wakes")
		return n != nil && len(n.List.Rows) == 0
	})
}

// Read scrollback on the Owned tab's terminal node: the whole output's line count beside
// the screen's; without the capability, says so.
func TestReadScrollback(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps []string
		want string
	}{
		{"capable", []string{"terminal.scrollback"}, "30 lines with the scrollback, 10 on the screen"},
		{"older host", nil, "cannot read a terminal's scrollback"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := start(t, fast(), plugintest.Options{Granted: every, Capabilities: tt.caps})
			openGallery(t, h)
			switchTab(t, h, tabOwned, pluginsdk.UiContext{})
			h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return find(decode(t, h, inst).Nodes, "term") != nil })
			v := decode(t, h, inst)
			acts := mustFind(t, v, "term").Terminal.Actions
			if len(acts) != 2 || acts[0].Key != "term-read-btn" || len(acts) > ui.MaxTerminalButtons {
				t.Fatalf("terminal buttons %+v", acts)
			}
			term := h.PluginTerminals()[0].TerminalID
			var all, screen []string
			for i := 1; i <= 30; i++ {
				all = append(all, fmt.Sprintf("tick %3d", i))
			}
			screen = all[20:]
			h.SetTerminalScrollback(term, strings.Join(all, "\n"))
			h.SetTerminalText(term, strings.Join(screen, "\n")+"\n\n")
			if err := click(t, h, "term-read", "term-read-btn", "", pluginsdk.UiContext{}); err != nil {
				t.Fatal(err)
			}
			text := mustFind(t, decode(t, h, inst), "term-read").Text.Content.Spans[0].Text
			if !strings.Contains(text, tt.want) {
				t.Errorf("read shows %q, want %q", text, tt.want)
			}
			if tt.caps != nil {
				reads := h.TerminalReads()
				if len(reads) != 2 || !reads[0].Scrollback || reads[1].Scrollback {
					t.Errorf("reads %+v", reads)
				}
			}
		})
	}
}
