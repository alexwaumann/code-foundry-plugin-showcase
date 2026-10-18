package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

// The gallery's tabs. Tab keys share the view's key namespace, hence the prefix.
const (
	tabText    = "tab-text"
	tabData    = "tab-data"
	tabLog     = "tab-log"
	tabForms   = "tab-forms"
	tabStatus  = "tab-status"
	tabActions = "tab-actions"
	tabOwned   = "tab-owned"
	tabHost    = "tab-host"
)

// tabDef is one tab: its label, what its header sub-line tells the tester to look for,
// how to draw it, and its events.
type tabDef struct {
	key, label, look string
	nodes            func(g *gallery) []ui.Node
	acts             map[string]actFunc
}

type actFunc func(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error

// tabs is the registry of tabs, in strip order (at most 8: ui.MaxTabs).
var tabs []tabDef

func init() {
	tabs = []tabDef{textTab, dataTab, logTab, formsTab, statusTab, actionsTab, ownedTab, hostTab}
}

func tabByKey(key string) (tabDef, bool) {
	for _, t := range tabs {
		if t.key == key {
			return t, true
		}
	}
	return tabDef{}, false
}

// gallery is the one instance of the global gallery surface. Its own UI state is
// guarded by s.mu like the shared state; renderMu keeps its views in order.
type gallery struct {
	s        *showcase
	open     pluginsdk.SurfaceOpen
	renderMu sync.Mutex

	tab     string
	opened  time.Time // times in the view are anchored here, so re-renders are stable
	visible bool
	lastAct *pluginsdk.SurfaceAct
	renders int

	// Data
	selectedRow, selectedChip string
	// Log
	logText   string
	logLines  []ui.LogLine
	logShown  bool // the last view carried the log nodes (their buffers live)
	streaming bool
	streamRun int
	// Forms
	profile  profileForm
	boomWhat string
	// Status
	progress     float64
	progressFail bool
	stale        bool
	// Actions
	busyKey string
	clicks  int
}

func (s *showcase) newGallery(_ context.Context, h *pluginsdk.Host, open pluginsdk.SurfaceOpen) (pluginsdk.Surface, error) {
	g := &gallery{s: s, open: open, tab: tabText, progress: 0.62, opened: s.now()}
	g.resetLog()
	g.profile = newProfileForm()
	s.mu.Lock()
	if s.h == nil {
		s.h = h
	}
	s.galleries[open.Instance] = g
	s.mu.Unlock()
	return g, g.render()
}

// render builds the active tab's view and sends it.
func (g *gallery) render() error {
	g.renderMu.Lock()
	defer g.renderMu.Unlock()
	g.s.mu.Lock()
	if _, open := g.s.galleries[g.open.Instance]; !open {
		g.s.mu.Unlock()
		return nil
	}
	v := g.viewLocked()
	h := g.s.h
	g.s.mu.Unlock()
	err := h.UpdateView(g.open.Instance, v)
	g.s.mu.Lock()
	if err == nil {
		g.logShown = g.tab == tabLog
		g.renders++
	}
	g.s.mu.Unlock()
	if err != nil {
		h.Log().Error("view refused", "tab", g.tab, "err", err)
	}
	return err
}

// viewLocked is the whole view; s.mu held.
func (g *gallery) viewLocked() ui.View {
	t, _ := tabByKey(g.tab)
	items := make([]ui.Tab, 0, len(tabs))
	for _, d := range tabs {
		tab := ui.NewTab(d.key, d.label)
		switch {
		case d.key == tabLog && g.streaming:
			tab.Badge = ui.NewPill("live", ui.ToneAttention)
		case d.key == tabHost:
			if n := g.s.eventTotalLocked(); n > 0 {
				tab.Badge = ui.NewPill(strconv.Itoa(n), ui.ToneNeutral)
			}
		case d.key == tabForms && g.profile.hasErrors():
			tab.Badge = ui.NewPill("errors", ui.ToneError)
		}
		items = append(items, tab)
	}
	header := &ui.Header{
		Icon:        "layers",
		Title:       "Showcase · " + t.label,
		TabTitle:    "Showcase",
		Pill:        ui.NewPill("v0.1.0", ui.ToneNeutral),
		Trailing:    ui.Plain(fmt.Sprintf("view %d", g.renders+1)),
		SublineIcon: "eye",
		Subline:     ui.Plain(t.look),
		Actions: []ui.Button{
			ui.EventButton("hdr-refresh", "Re-render this tab", "refresh").WithIcon("refresh-cw").AsIconOnly(),
			ui.URLButton("hdr-readme", "Open the README", readmeURL).WithIcon("external-link").AsIconOnly(),
		},
	}
	return ui.View{Header: header, Tabs: ui.NewTabs(g.tab, items...), Nodes: t.nodes(g)}
}

const readmeURL = "https://github.com/alexwaumann/code-foundry-plugin-showcase#readme"

// Act routes tabs, header buttons and each tab's events.
func (g *gallery) Act(ctx context.Context, act pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	a := act
	g.lastAct = &a
	g.clicks++
	g.s.mu.Unlock()
	switch {
	case act.Kind == pluginsdk.ActTab:
		if _, ok := tabByKey(act.Key); !ok {
			return fmt.Errorf("unknown tab %q", act.Key)
		}
		g.s.mu.Lock()
		g.tab = act.Key
		g.s.mu.Unlock()
		if act.Key == tabOwned {
			go g.s.ensureOwned(context.WithoutCancel(ctx))
		}
		if act.Key == tabHost {
			g.s.refreshSecrets(ctx)
		}
		if act.Key == tabForms {
			g.refreshTokenSet(ctx)
		}
		return g.render()
	case act.Action == "refresh":
		return g.render()
	}
	g.s.mu.Lock()
	t, _ := tabByKey(g.tab)
	g.s.mu.Unlock()
	if f := t.acts[act.Action]; f != nil {
		return f(ctx, g, act)
	}
	// An element of a view of another tab (the last 4 versions still act).
	for _, d := range tabs {
		if f := d.acts[act.Action]; f != nil {
			return f(ctx, g, act)
		}
	}
	return fmt.Errorf("unknown action %q", act.Action)
}

func (g *gallery) Visibility(visible bool) {
	g.s.mu.Lock()
	g.visible = visible
	g.s.mu.Unlock()
}

func (g *gallery) Close() {
	g.s.mu.Lock()
	delete(g.s.galleries, g.open.Instance)
	g.streamRun++ // stops a running stream
	g.s.mu.Unlock()
}

func (s *showcase) eventTotalLocked() int {
	n := 0
	for _, sub := range s.subs {
		n += sub.Count
	}
	return n
}

// actContextPairs shows what the last act's context said: the panel's thread, project
// and view (a global surface learns the viewer's thread only from acts).
func (g *gallery) actContextLocked() []ui.Pair {
	if g.lastAct == nil {
		return []ui.Pair{ui.NewPair("ctx-none", "Context", ui.Spans(ui.T("click anything: the act names the panel's thread").WithTone(ui.ToneMuted)))}
	}
	c := g.lastAct.Context
	return []ui.Pair{
		{Key: "ctx-thread", Label: "Thread", Value: orNone(c.ThreadID, true), Copyable: c.ThreadID != ""},
		{Key: "ctx-project", Label: "Project", Value: orNone(c.ProjectID, true)},
		{Key: "ctx-view", Label: "View", Value: orNone(c.View, false)},
	}
}

func orNone(s string, mono bool) *ui.Inline {
	if s == "" {
		return ui.Spans(ui.T("none").WithTone(ui.ToneMuted))
	}
	sp := ui.T(s)
	if mono {
		sp = sp.AsMono()
	}
	return ui.Spans(sp)
}
