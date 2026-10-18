package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
)

// showcase is the plugin's state shared by every surface instance and command. Fields
// below mu are guarded by it; views are built under mu and sent outside it.
type showcase struct {
	now func() time.Time
	// renderDelay batches event-driven re-renders (counters tick on every event).
	renderDelay time.Duration
	// streamEvery is the pause between streamed log lines; busyFor how long a
	// "loading" button spins; saveFor how long a form save pretends to take.
	streamEvery, busyFor, saveFor time.Duration

	mu        sync.Mutex
	h         *pluginsdk.Host
	started   time.Time
	galleries map[string]*gallery
	gateds    map[string]*gated
	pending   bool // an event render is scheduled

	pings, booms int
	gates        gateState
	flags        map[string]flagResult // by thread id
	subs         map[string]*subscription
	env          envState
	owned        ownedState
	secrets      secretState
	invokes      map[string]invokeResult // by command
	closed       bool
}

type gateState struct {
	Global  bool            `json:"global"`
	Threads map[string]bool `json:"threads"`
	SetAt   time.Time       `json:"setAt"`
}

type flagResult struct {
	Level, Reason string
	UntilSeen     bool
	Changed       bool
	Err           string
	At            time.Time
}

// subscription is one event source the Host tab can subscribe to.
type subscription struct {
	ID       string
	Count    int
	Snaps    int
	Last     string // the change's key inside the source ("updated", "snapshot", …)
	LastAt   time.Time
	Err      string
	Pending  bool
	Duration time.Duration
}

type envState struct {
	On      bool      `json:"on"`
	Bad     bool      `json:"bad"`
	Applied bool      `json:"applied"`
	Issues  []string  `json:"issues"`
	At      time.Time `json:"at"`
	Err     string    `json:"-"`
}

type ownedState struct {
	Helper, Narrator, Terminal string
	Creating                   bool
	Err                        string
	Threads                    []pluginsdk.PluginThread
	Terminals                  []pluginsdk.PluginTerminal
	Sent                       int
}

type secretState struct {
	List   []pluginsdk.SecretInfo
	Result string // what the last secret button found ("set at …" lives in List)
	Tone   string
	At     time.Time
}

type invokeResult struct {
	Command  string
	Thread   string
	Message  string
	JSON     string
	ExitCode int
	Err      string
	Code     int
	At       time.Time
}

// persisted is what survives a restart in the data dir (facts and the environment's
// state; the environment itself is kept by the host).
type persisted struct {
	Gate gateState `json:"gate"`
	Env  envState  `json:"env"`
}

// sources are the event sources the Host tab shows, in order.
var sources = []string{"threads", "project", "gh"}

func newShowcase() *showcase {
	s := &showcase{
		now:         time.Now,
		renderDelay: 400 * time.Millisecond,
		streamEvery: 250 * time.Millisecond,
		busyFor:     2 * time.Second,
		saveFor:     800 * time.Millisecond,
		galleries:   map[string]*gallery{},
		gateds:      map[string]*gated{},
		flags:       map[string]flagResult{},
		subs:        map[string]*subscription{},
		invokes:     map[string]invokeResult{},
		gates:       gateState{Threads: map[string]bool{}},
	}
	for _, src := range sources {
		s.subs[src] = &subscription{}
	}
	return s
}

// init restores state, re-sets the context facts (they end with the process), lays
// out the files the session environment and the plugin threads point at, and finds the
// threads and terminals of a previous run.
func (s *showcase) init(ctx context.Context, h *pluginsdk.Host) error {
	s.mu.Lock()
	s.h = h
	s.started = s.now()
	s.mu.Unlock()
	if err := writeAddon(h.DataDir()); err != nil {
		h.Log().Warn("addon files", "err", err)
	}
	if p, err := loadPersisted(h.DataDir()); err == nil {
		s.mu.Lock()
		s.gates = p.Gate
		if s.gates.Threads == nil {
			s.gates.Threads = map[string]bool{}
		}
		s.env = p.Env
		s.mu.Unlock()
		if p.Gate.Global {
			_ = h.SetContext(pluginsdk.ContextParams{Key: "on", Value: true})
		}
		for id, on := range p.Gate.Threads {
			if on {
				_ = h.SetContext(pluginsdk.ContextParams{Key: "on", Value: true, ThreadID: id})
			}
		}
	}
	for _, f := range h.Flags() {
		s.mu.Lock()
		s.flags[f.ThreadID] = flagResult{Level: f.Level, Reason: f.Reason, UntilSeen: f.UntilSeen, At: f.SetAt}
		s.mu.Unlock()
	}
	s.refreshOwned(ctx)
	h.Log().Info("showcase is up", "host", h.HostVersion(), "dev", h.Dev(), "flags", len(h.Flags()))
	return nil
}

func (s *showcase) shutdown() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func (s *showcase) host() *pluginsdk.Host {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.h
}

// save writes the persisted state; called with mu held.
func (s *showcase) saveLocked() {
	if s.h == nil {
		return
	}
	data, err := json.MarshalIndent(persisted{Gate: s.gates, Env: s.env}, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(s.h.DataDir(), "state.json"), data, 0o600)
	}
	if err != nil {
		s.h.Log().Warn("saving state", "err", err)
	}
}

func loadPersisted(dir string) (persisted, error) {
	var p persisted
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(data, &p)
}

// addonDir holds a skill for --add-dir (the session environment and the helper
// thread); binDir an executable for the environment's PATH entry.
func addonDir(data string) string { return filepath.Join(data, "addon") }
func binDir(data string) string   { return filepath.Join(data, "bin") }

const addonSkill = `---
name: showcase-hello
description: "Answer questions about the Code Foundry showcase plugin's thread: say which SHOWCASE_* variables you see and that this skill came from the plugin's --add-dir."
---

# Showcase hello

You are running in a thread the Code Foundry **showcase** plugin created (or a thread
started while its session environment was on). When asked who you are:

1. Run ` + "`printenv | grep ^SHOWCASE`" + ` and report the variables.
2. Say that this skill was loaded from the plugin's ` + "`--add-dir`" + `.
3. Keep the answer to two short sentences.
`

const helloScript = `#!/bin/sh
echo "showcase-hello: on PATH from the showcase plugin's session environment"
`

func writeAddon(data string) error {
	skill := filepath.Join(addonDir(data), ".claude", "skills", "showcase-hello")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(addonSkill), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(binDir(data), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(binDir(data), "showcase-hello"), []byte(helloScript), 0o755)
}

// renderAll re-renders every open instance (shared state changed).
func (s *showcase) renderAll() {
	s.mu.Lock()
	gs := make([]*gallery, 0, len(s.galleries))
	for _, g := range s.galleries {
		gs = append(gs, g)
	}
	ds := make([]*gated, 0, len(s.gateds))
	for _, d := range s.gateds {
		ds = append(ds, d)
	}
	s.mu.Unlock()
	for _, g := range gs {
		_ = g.render()
	}
	for _, d := range ds {
		_ = d.render()
	}
}

// renderSoon batches re-renders caused by events: at most one per renderDelay.
func (s *showcase) renderSoon() {
	s.mu.Lock()
	if s.pending || s.closed {
		s.mu.Unlock()
		return
	}
	s.pending = true
	s.mu.Unlock()
	time.AfterFunc(s.renderDelay, func() {
		s.mu.Lock()
		s.pending = false
		s.mu.Unlock()
		s.renderAll()
	})
}

// onEvent counts events per source. It runs on the notification goroutine: count,
// schedule a render, return.
func (s *showcase) onEvent(_ context.Context, _ *pluginsdk.Host, ev pluginsdk.Event) {
	var outer map[string]json.RawMessage
	if json.Unmarshal(ev.Event, &outer) != nil {
		return
	}
	s.mu.Lock()
	for src, sub := range s.subs {
		if sub.ID != ev.Subscription {
			continue
		}
		key := eventKeys[src]
		var inner map[string]json.RawMessage
		_ = json.Unmarshal(outer[key], &inner)
		change := firstKey(inner)
		sub.Count++
		if ev.Snapshot {
			sub.Snaps++
		}
		sub.Last = change
		sub.LastAt = s.now()
	}
	s.mu.Unlock()
	s.renderSoon()
}

// eventKeys maps a manifest source to its key in the event JSON.
var eventKeys = map[string]string{"threads": "threadList", "project": "project", "gh": "gh"}

func firstKey(m map[string]json.RawMessage) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return "(empty)"
	}
	return keys[0]
}

// subscribe and unsubscribe one source for the Host tab.
func (s *showcase) subscribe(ctx context.Context, src string) error {
	s.mu.Lock()
	sub, ok := s.subs[src]
	if !ok {
		s.mu.Unlock()
		return pluginsdk.NewError(pluginsdk.CodeInvalidParams, "unknown source %q", src)
	}
	if sub.ID != "" {
		s.mu.Unlock()
		return nil
	}
	h := s.h
	s.mu.Unlock()
	id, err := h.Subscribe(ctx, src)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		sub.Err = err.Error()
		return nil
	}
	sub.ID, sub.Err = id, ""
	return nil
}

func (s *showcase) unsubscribe(ctx context.Context, src string) error {
	s.mu.Lock()
	sub, ok := s.subs[src]
	if !ok || sub.ID == "" {
		s.mu.Unlock()
		return nil
	}
	id, h := sub.ID, s.h
	s.mu.Unlock()
	err := h.Unsubscribe(ctx, id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		sub.Err = err.Error()
		return nil
	}
	sub.ID, sub.Err = "", ""
	return nil
}

// refreshOwned re-reads the plugin's own threads and terminals and picks up the
// helper, narrator and ticker by title.
func (s *showcase) refreshOwned(ctx context.Context) {
	h := s.host()
	var (
		threads    []pluginsdk.PluginThread
		terms      []pluginsdk.PluginTerminal
		terr, merr error
	)
	if h.Granted().Threads {
		threads, terr = h.Threads(ctx)
	}
	if h.Granted().Terminals {
		terms, merr = h.Terminals(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := errors.Join(terr, merr); err != nil {
		s.owned.Err = err.Error()
		return
	}
	s.owned.Threads, s.owned.Terminals = threads, terms
	s.owned.Helper, s.owned.Narrator, s.owned.Terminal = "", "", ""
	for _, t := range threads {
		if t.Archived {
			continue
		}
		switch t.Title {
		case helperTitle:
			s.owned.Helper = t.ThreadID
		case narratorTitle:
			s.owned.Narrator = t.ThreadID
		}
	}
	for _, t := range terms {
		if t.Title == tickerTitle {
			s.owned.Terminal = t.TerminalID
		}
	}
}
