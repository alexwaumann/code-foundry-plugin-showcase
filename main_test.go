package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/plugintest"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

// every is the manifest's permission block.
var every = pluginsdk.Permissions{
	Commands:     []string{"thread.send", "view.open.url", "pr.checks"},
	Events:       []string{"threads", "project", "gh"},
	ThreadStatus: true, SessionEnv: true, Secrets: true, Threads: true, Terminals: true,
}

const inst = "showcase/gallery/global"

// fast returns a showcase with short timers for tests.
func fast() *showcase {
	s := newShowcase()
	s.renderDelay, s.streamEvery, s.busyFor, s.saveFor = 5*time.Millisecond, time.Millisecond, 20*time.Millisecond, 20*time.Millisecond
	return s
}

func start(t *testing.T, s *showcase, opts plugintest.Options) *plugintest.Host {
	t.Helper()
	if opts.Name == "" {
		opts.Name = "showcase"
	}
	return plugintest.Start(t, s.plugin(), opts)
}

func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// decode reads an instance's last view and checks it as the host will.
func decode(t *testing.T, h *plugintest.Host, instance string) ui.View {
	t.Helper()
	raw, ok := h.View(instance)
	if !ok {
		t.Fatal("no view")
	}
	var v ui.View
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if err := v.Validate(); err != nil {
		t.Fatalf("invalid view: %v", err)
	}
	return v
}

// find returns the node with key, looking inside sections.
func find(nodes []ui.Node, key string) *ui.Node {
	for i := range nodes {
		if nodes[i].Key == key {
			return &nodes[i]
		}
		if s := nodes[i].Section; s != nil {
			if n := find(s.Children, key); n != nil {
				return n
			}
		}
	}
	return nil
}

func mustFind(t *testing.T, v ui.View, key string) *ui.Node {
	t.Helper()
	n := find(v.Nodes, key)
	if n == nil {
		t.Fatalf("no node %s in tab %s", key, v.Tabs.Active)
	}
	return n
}

func pairValue(t *testing.T, n *ui.Node, key string) string {
	t.Helper()
	for _, p := range n.Pairs.Items {
		if p.Key == key {
			var b strings.Builder
			if p.Value != nil {
				b.WriteString(p.Value.Text)
				for _, s := range p.Value.Spans {
					b.WriteString(s.Text)
				}
			}
			if p.Pill != nil {
				b.WriteString("[" + p.Pill.Label + "]")
			}
			return b.String()
		}
	}
	t.Fatalf("no pair %s", key)
	return ""
}

func openGallery(t *testing.T, h *plugintest.Host) {
	t.Helper()
	if err := h.OpenSurface(testCtx(t), inst, "gallery", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
}

func switchTab(t *testing.T, h *plugintest.Host, key string, uctx pluginsdk.UiContext) ui.View {
	t.Helper()
	if err := h.Act(testCtx(t), pluginsdk.SurfaceAct{Instance: inst, Kind: pluginsdk.ActTab, Action: "tab", Key: key, Value: key, Context: uctx}); err != nil {
		t.Fatal(err)
	}
	v := decode(t, h, inst)
	if v.Tabs.Active != key {
		t.Fatalf("active tab %s, want %s", v.Tabs.Active, key)
	}
	return v
}

func click(t *testing.T, h *plugintest.Host, action, key, data string, uctx pluginsdk.UiContext) error {
	t.Helper()
	return h.Act(testCtx(t), pluginsdk.SurfaceAct{Instance: inst, Kind: pluginsdk.ActClick, Action: action, Key: key, Data: data, Context: uctx})
}

// plugin.json is valid and declares exactly what the plugin serves.
func TestManifest(t *testing.T) {
	data, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	m, warnings, err := pluginsdk.LoadManifest(data)
	if err != nil || len(warnings) > 0 {
		t.Fatalf("plugin.json: %v %v", err, warnings)
	}
	p := plugin()
	for _, c := range m.Commands {
		if p.Commands[c.Name] == nil {
			t.Errorf("command %s is declared but not served", c.Name)
		}
	}
	for _, s := range m.Surfaces {
		if p.Surfaces[s.ID] == nil {
			t.Errorf("surface %s is declared but not served", s.ID)
		}
	}
	if len(p.Commands) != len(m.Commands) || len(p.Surfaces) != len(m.Surfaces) {
		t.Errorf("serves %d commands and %d surfaces, declares %d and %d", len(p.Commands), len(p.Surfaces), len(m.Commands), len(m.Surfaces))
	}
	if !m.Permissions.Equal(every.Normalize()) {
		t.Errorf("permissions %+v, want %+v", m.Permissions, every)
	}
	if len(tabs) > ui.MaxTabs {
		t.Errorf("%d tabs, at most %d", len(tabs), ui.MaxTabs)
	}
}

// Every tab renders a valid view, with the active tab set and a sub-line saying what
// to look for.
func TestEveryTabValid(t *testing.T) {
	for _, perms := range []pluginsdk.Permissions{every, {}} {
		h := start(t, fast(), plugintest.Options{Granted: perms})
		openGallery(t, h)
		v := decode(t, h, inst)
		if v.Tabs == nil || len(v.Tabs.Items) != len(tabs) || v.Tabs.Active != tabText {
			t.Fatalf("tabs %+v", v.Tabs)
		}
		for _, d := range tabs {
			v := switchTab(t, h, d.key, pluginsdk.UiContext{ThreadID: "s-1"})
			if len(v.Nodes) == 0 {
				t.Errorf("tab %s has no nodes", d.key)
			}
			if v.Header == nil || v.Header.Subline == nil || v.Header.Subline.Text != d.look {
				t.Errorf("tab %s: header sub-line %+v", d.key, v.Header)
			}
		}
	}
}

func TestCommands(t *testing.T) {
	ctx := testCtx(t)
	h := start(t, fast(), plugintest.Options{Granted: every})
	thread := pluginsdk.UiContext{ThreadID: "s-1"}
	for _, tt := range []struct {
		name    string
		args    map[string]string
		uctx    pluginsdk.UiContext
		want    string
		wantErr string
	}{
		{"ping", nil, pluginsdk.UiContext{}, "pong 1", ""},
		{"ping", nil, thread, "pong 2", ""},
		{"boom", map[string]string{"what": "the demo"}, pluginsdk.UiContext{}, "Boom: the demo is gone", ""},
		{"boom", map[string]string{"what": " "}, pluginsdk.UiContext{}, "", "say what to blow up"},
		{"flag", map[string]string{"level": "attention", "reason": "look"}, pluginsdk.UiContext{}, "", "no thread"},
		{"flag", map[string]string{"level": "attention", "reason": "look", "thread": "s-1"}, pluginsdk.UiContext{}, "Flagged s-1: look (attention)", ""},
		{"flag", map[string]string{"level": "error", "reason": strings.Repeat("x", 81)}, thread, "", "1 to 80"},
		{"flag", map[string]string{"level": "none"}, thread, "Cleared the flag on s-1", ""},
		{"gate", map[string]string{"state": "on", "scope": "thread"}, thread, "Gate on for s-1", ""},
		{"gate", map[string]string{"state": "on", "scope": "thread"}, pluginsdk.UiContext{}, "", "no thread"},
		{"gate", map[string]string{"state": "on", "scope": "global"}, pluginsdk.UiContext{}, "Gate on everywhere", ""},
		{"gate", map[string]string{"state": "off", "scope": "thread"}, thread, "Gate off for s-1", ""},
	} {
		res, err := h.Command(ctx, tt.name, tt.args, tt.uctx)
		if tt.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("%s %v: err %v, want %q", tt.name, tt.args, err, tt.wantErr)
			}
			continue
		}
		if err != nil || !strings.HasPrefix(res.Message, tt.want) {
			t.Errorf("%s %v = %q, %v; want %q", tt.name, tt.args, res.Message, err, tt.want)
		}
	}
	flags := h.Flags()
	if len(flags) != 2 || flags[0].Level != "attention" || flags[0].ThreadID != "s-1" || flags[1].Level != "none" {
		t.Errorf("flags %+v", flags)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return len(h.Contexts()) == 3 })
	c := h.Contexts()
	if c[0].Key != "on" || c[0].Value != true || c[0].ThreadID != "s-1" || c[1].ThreadID != "" || c[2].Value != nil {
		t.Errorf("contexts %+v", c)
	}
}

func TestFlagRefused(t *testing.T) {
	h := start(t, fast(), plugintest.Options{})
	_, err := h.Command(testCtx(t), "flag", map[string]string{"level": "error", "reason": "x"}, pluginsdk.UiContext{ThreadID: "s-1"})
	if pluginsdk.ErrorCode(err) != pluginsdk.CodePermissionDenied {
		t.Errorf("flag without threadStatus: %v (code %d)", err, pluginsdk.ErrorCode(err))
	}
}

func TestDataTab(t *testing.T) {
	h := start(t, fast(), plugintest.Options{Granted: every})
	openGallery(t, h)
	switchTab(t, h, tabData, pluginsdk.UiContext{})
	if err := click(t, h, "select-row", "st-failed", "st-failed", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
	v := decode(t, h, inst)
	rows := mustFind(t, v, "list-status").List.Rows
	if !rows[1].Selected || rows[0].Selected {
		t.Errorf("row selection: %+v", rows[:2])
	}
	if err := click(t, h, "select-chip", "chip-running", "chip-running", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
	v = decode(t, h, inst)
	for _, c := range mustFind(t, v, "chips-all").Chips.Chips {
		if c.Selected != (c.Key == "chip-running") {
			t.Errorf("chip %s selected=%v", c.Key, c.Selected)
		}
	}
	if !mustFind(t, v, "list-dense").List.Dense {
		t.Error("dense list is not dense")
	}
}

func TestLogTab(t *testing.T) {
	h := start(t, fast(), plugintest.Options{Granted: every})
	openGallery(t, h)
	v := switchTab(t, h, tabLog, pluginsdk.UiContext{})
	if got := mustFind(t, v, "log-text").Log.Text; got != logSeed {
		t.Errorf("first view seeds the log: %q", got)
	}
	// The next view keeps the buffer (empty text), and the stream appends.
	if err := click(t, h, "log-stream", "log-stream-btn", "", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool {
		n := 0
		for _, a := range h.Appends() {
			if a.Key == "log-text" {
				n++
			}
		}
		return n == 21
	})
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool {
		v := decode(t, h, inst)
		return !mustFind(t, v, "log-bar").Actions.Buttons[0].Busy
	})
	v = decode(t, h, inst)
	if mustFind(t, v, "log-text").Log.Text != "" {
		t.Error("a re-render while the log is shown must leave text empty")
	}
	if b := v.Tabs.Items[2].Badge; b != nil {
		t.Errorf("badge after the stream: %+v", b)
	}
	if err := click(t, h, "log-lines", "log-lines-btn", "", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
	last := h.Appends()[len(h.Appends())-1]
	if last.Key != "log-lines" || len(last.Lines) != len(toneCycle) || last.Lines[3].Tone != "error" {
		t.Errorf("lines append %+v", last)
	}
	// Away and back: the nodes come back seeded with everything so far.
	switchTab(t, h, tabText, pluginsdk.UiContext{})
	v = switchTab(t, h, tabLog, pluginsdk.UiContext{})
	if txt := mustFind(t, v, "log-text").Log.Text; !strings.Contains(txt, "done: 20 lines") {
		t.Errorf("re-seeded log lacks the stream: %q", txt)
	}
	if n := len(mustFind(t, v, "log-lines").Log.Lines); n != 10 {
		t.Errorf("re-seeded lines: %d", n)
	}
	if err := click(t, h, "log-start-over", "log-start-over-btn", "", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
	if txt := mustFind(t, decode(t, h, inst), "log-text").Log.Text; txt != logSeed {
		t.Errorf("start over: %q", txt)
	}
}

func TestFormsTab(t *testing.T) {
	ctx := testCtx(t)
	h := start(t, fast(), plugintest.Options{Granted: every})
	openGallery(t, h)
	switchTab(t, h, tabForms, pluginsdk.UiContext{})
	submit := func(values map[string]any) error {
		return h.Act(ctx, pluginsdk.SurfaceAct{Instance: inst, Kind: pluginsdk.ActSubmit, Action: "profile-save", Key: "profile-save-btn", Values: values})
	}
	good := map[string]any{"handle": "ada-l", "email": "ada@x.dev", "pin": "1234", "colour": "green", "plan": "team",
		"tags": []any{"go"}, "langs": []any{}, "project": "", "thread": "", "notify": true, "onFail": true, "onDone": false, "disabled": "read only"}

	// Invalid: every rule the plugin re-checks.
	bad := map[string]any{}
	for k, v := range good {
		bad[k] = v
	}
	bad["handle"], bad["email"], bad["tags"], bad["onFail"] = "admin", "nope", []any{"go", "ts", "py", "rs"}, false
	if err := submit(bad); err != nil {
		t.Fatal(err)
	}
	v := decode(t, h, inst)
	form := mustFind(t, v, "form-profile").Form
	if form.Error == "" || form.Fields[0].Error == "" || form.Fields[1].Error == "" || form.Fields[6].Error == "" || form.Fields[11].Fieldset.Error == "" {
		t.Errorf("plugin-side errors missing: form %q fields %+v", form.Error, form.Fields)
	}
	if v.Tabs.Items[3].Badge == nil || v.Tabs.Items[3].Badge.Tone != ui.ToneError {
		t.Errorf("forms badge %+v", v.Tabs.Items[3].Badge)
	}
	bad["handle"] = "Not Valid"
	if err := submit(bad); err != nil {
		t.Fatal(err)
	}
	if e := mustFind(t, decode(t, h, inst), "form-profile").Form.Fields[0].Error; !strings.Contains(e, "Lower case") {
		t.Errorf("pattern re-check: %q", e)
	}

	// Change events: the draft read-back.
	if err := h.Act(ctx, pluginsdk.SurfaceAct{Instance: inst, Kind: pluginsdk.ActChange, Action: "profile-draft", Key: "f-email", Field: "email", Value: "ada@typ"}); err != nil {
		t.Fatal(err)
	}
	if got := pairValue(t, mustFind(t, decode(t, h, inst), "pairs-draft"), "draft-email"); got != "ada@typ" {
		t.Errorf("draft %q", got)
	}

	// Valid, with a stored token: busy, then saved values and a new resetToken.
	h.SetSecret(tokenKey, "s3cr3t-value")
	good["token"] = map[string]any{"key": tokenKey, "stored": true}
	if err := submit(good); err != nil {
		t.Fatal(err)
	}
	v = decode(t, h, inst)
	if f := mustFind(t, v, "form-profile").Form; !f.Busy || f.Error != "" {
		t.Errorf("valid submit: busy %v error %q", f.Busy, f.Error)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool {
		f := mustFind(t, decode(t, h, inst), "form-profile").Form
		return !f.Busy && f.ResetToken == "1"
	})
	v = decode(t, h, inst)
	form = mustFind(t, v, "form-profile").Form
	if form.Fields[0].Input.Value != "ada-l" || !form.Fields[2].Input.HasValue {
		t.Errorf("saved values: handle %q token hasValue %v", form.Fields[0].Input.Value, form.Fields[2].Input.HasValue)
	}
	rb := mustFind(t, v, "pairs-readback")
	if got := pairValue(t, rb, "rb-pin"); got != "4 characters (value withheld)" {
		t.Errorf("pin read-back %q", got)
	}
	if got := pairValue(t, rb, "rb-token"); got != "stored under token" {
		t.Errorf("token read-back %q", got)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return len(h.Toasts()) == 1 })

	// Reset and Fill example bump the token.
	for i, a := range []string{"profile-example", "profile-reset"} {
		if err := click(t, h, a, a+"-btn", "", pluginsdk.UiContext{}); err != nil {
			t.Fatal(err)
		}
		if got := mustFind(t, decode(t, h, inst), "form-profile").Form.ResetToken; got != []string{"2", "3"}[i] {
			t.Errorf("%s: resetToken %s", a, got)
		}
	}

	// The command form submits through plugin.showcase.boom.
	b := mustFind(t, v, "form-boom").Form.Submit
	if b.Action.Command == nil || b.Action.Command.Name != "plugin.showcase.boom" || b.Style != ui.ButtonDestructive {
		t.Errorf("boom form submit %+v", b)
	}

	// No secret value anywhere.
	for _, s := range []string{string(mustJSON(h.Logs())), string(mustJSON(h.Toasts())), string(mustRaw(h, inst))} {
		if strings.Contains(s, "s3cr3t-value") || strings.Contains(s, `"1234"`) {
			t.Error("a secret leaked")
		}
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func mustRaw(h *plugintest.Host, instance string) []byte {
	raw, _ := h.View(instance)
	return raw
}

func TestStatusTab(t *testing.T) {
	h := start(t, fast(), plugintest.Options{Granted: every})
	openGallery(t, h)
	v := switchTab(t, h, tabStatus, pluginsdk.UiContext{})
	if p := mustFind(t, v, "prog-indet").Progress; p.Value != nil {
		t.Error("indeterminate progress has a value")
	}
	for _, a := range []string{"progress-step", "progress-fail", "stale-toggle"} {
		if err := click(t, h, a, a, "", pluginsdk.UiContext{}); err != nil {
			t.Fatal(err)
		}
	}
	v = decode(t, h, inst)
	if p := mustFind(t, v, "prog-det").Progress; *p.Value < 0.7 || p.Tone != ui.ToneError {
		t.Errorf("progress %+v", p)
	}
	if !mustFind(t, v, "sec-stale").Section.Stale {
		t.Error("stale toggle")
	}
	if n := mustFind(t, v, "sec-inner"); n.Section == nil {
		t.Error("nested section missing")
	}
}

func TestActionsTab(t *testing.T) {
	h := start(t, fast(), plugintest.Options{Granted: every})
	openGallery(t, h)
	v := switchTab(t, h, tabActions, pluginsdk.UiContext{})
	for _, b := range mustFind(t, v, "bar-styles").Actions.Buttons {
		if b.Style == "primary" {
			t.Error("a filled button")
		}
	}
	uctx := pluginsdk.UiContext{ThreadID: "s-9", View: "thread"}
	if err := click(t, h, "click", "ev-json", `{"run":7,"job":"test"}`, uctx); err != nil {
		t.Fatal(err)
	}
	last := mustFind(t, decode(t, h, inst), "pairs-last")
	if got := pairValue(t, last, "last-data"); got != `{"run":7,"job":"test"}` {
		t.Errorf("last data %q", got)
	}
	if got := pairValue(t, last, "ctx-thread"); got != "s-9" {
		t.Errorf("act context thread %q", got)
	}
	if err := click(t, h, "load", "btn-load", "", uctx); err != nil {
		t.Fatal(err)
	}
	busy := func(h *plugintest.Host) bool {
		return mustFind(t, decode(t, h, inst), "bar-loading").Actions.Buttons[0].Busy
	}
	if !busy(h) {
		t.Error("load button not busy")
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return !busy(h) })
	if err := click(t, h, "nope-not-handled", "ev-unknown", "", uctx); err == nil {
		t.Error("an unknown event was accepted")
	}
	// Command buttons name the plugin's own commands or permitted host commands.
	for _, b := range mustFind(t, v, "bar-commands").Actions.Buttons {
		name := b.Action.Command.Name
		if !strings.HasPrefix(name, "plugin.showcase.") && !every.AllowsCommand(name) {
			t.Errorf("button %s runs %s, not permitted", b.Key, name)
		}
	}
}

func TestOwnedTab(t *testing.T) {
	ctx := testCtx(t)
	h := start(t, fast(), plugintest.Options{Granted: every})
	openGallery(t, h)
	switchTab(t, h, tabOwned, pluginsdk.UiContext{})
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool {
		v := decode(t, h, inst)
		return find(v.Nodes, "helper") != nil && find(v.Nodes, "narrator") != nil && find(v.Nodes, "term") != nil
	})
	created := h.CreatedThreads()
	if len(created) != 2 || created[0].Title != helperTitle || created[0].Cwd != h.DataDir() ||
		len(created[0].AddDirs) != 1 || created[0].Env["SHOWCASE_THREAD"] != "helper" {
		t.Fatalf("created threads %+v", created)
	}
	if _, err := os.Stat(addonDir(h.DataDir()) + "/.claude/skills/showcase-hello/SKILL.md"); err != nil {
		t.Errorf("skill file: %v", err)
	}
	terms := h.CreatedTerminals()
	if len(terms) != 1 || terms[0].Title != tickerTitle || terms[0].Env["SHOWCASE_TERM"] != "ticker" {
		t.Fatalf("created terminals %+v", terms)
	}
	v := decode(t, h, inst)
	if !mustFind(t, v, "narrator").Thread.ReadOnly || mustFind(t, v, "helper").Thread.ReadOnly {
		t.Error("read-only variants swapped")
	}
	if len(mustFind(t, v, "term").Terminal.Actions) != 2 {
		t.Error("terminal buttons")
	}
	for _, a := range []string{"owned-ask", "owned-narrate"} {
		if err := click(t, h, a, a+"-btn", "", pluginsdk.UiContext{}); err != nil {
			t.Fatal(err)
		}
	}
	if s := h.ThreadSends(); len(s) != 2 || s[0].Text != askPrompt || s[1].ThreadID == s[0].ThreadID {
		t.Errorf("sends %+v", s)
	}
	if err := click(t, h, "term-stop", "term-stop-btn", "", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
	if len(h.PluginTerminals()) != 0 {
		t.Errorf("terminal not closed: %+v", h.PluginTerminals())
	}
	if find(decode(t, h, inst).Nodes, "term-none") == nil {
		t.Error("no empty state after stop")
	}
	if err := click(t, h, "owned-archive", "owned-archive-btn", "", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
	for _, th := range h.PluginThreads() {
		if !th.Archived {
			t.Errorf("thread %s not archived", th.ThreadID)
		}
	}
	// Recreate makes new ones.
	if err := click(t, h, "owned-recreate", "owned-recreate-btn", "", pluginsdk.UiContext{}); err != nil {
		t.Fatal(err)
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return len(h.CreatedThreads()) == 4 && len(h.CreatedTerminals()) == 2 })
	_ = ctx
}

func TestOwnedWithoutPermission(t *testing.T) {
	h := start(t, fast(), plugintest.Options{})
	openGallery(t, h)
	switchTab(t, h, tabOwned, pluginsdk.UiContext{})
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return find(decode(t, h, inst).Nodes, "owned-perm") != nil })
	if len(h.CreatedThreads()) != 0 {
		t.Error("created a thread without permission")
	}
}

func TestHostTab(t *testing.T) {
	ctx := testCtx(t)
	h := start(t, fast(), plugintest.Options{Granted: every,
		Invoke: func(_ context.Context, p pluginsdk.InvokeParams) (pluginsdk.CommandResult, error) {
			switch p.Command {
			case "pr.checks":
				if p.Context.ThreadID != "s-7" {
					return pluginsdk.CommandResult{}, pluginsdk.NewError(pluginsdk.CodeUnavailable, "has no pull request")
				}
				return pluginsdk.CommandResult{Message: "2 checks", JSON: json.RawMessage(`{"repo":"o/r","number":3,"checks":[]}`)}, nil
			case "thread.send":
				return pluginsdk.CommandResult{Message: "sent"}, nil
			}
			return pluginsdk.CommandResult{}, pluginsdk.NewError(pluginsdk.CodeNotFound, "unknown command")
		}})
	openGallery(t, h)
	uctx := pluginsdk.UiContext{ThreadID: "s-7", ProjectID: "p-1"}
	v := switchTab(t, h, tabHost, pluginsdk.UiContext{})
	if find(v.Nodes, "host-no-thread") == nil {
		t.Error("no hint without a thread")
	}
	v = switchTab(t, h, tabHost, uctx)
	if find(v.Nodes, "host-no-thread") != nil {
		t.Error("hint with a thread")
	}

	// Flag until seen from the act's context.
	if err := click(t, h, "flag-seen", "flag-seen-btn", "", uctx); err != nil {
		t.Fatal(err)
	}
	if f := h.Flags(); len(f) != 1 || f[0].ThreadID != "s-7" || !f[0].UntilSeen {
		t.Errorf("flags %+v", f)
	}
	if err := click(t, h, "flag-seen", "flag-seen-btn", "", pluginsdk.UiContext{}); err == nil {
		t.Error("flag without a thread was accepted")
	}

	// Toasts: app and panel, every level.
	for _, d := range []string{"app info", "app attention", "app error", "panel info", "panel attention", "panel error"} {
		if err := click(t, h, "toast", "toast-x", d, uctx); err != nil {
			t.Fatal(err)
		}
	}
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return len(h.Toasts()) == 6 })
	if ts := h.Toasts(); ts[0].Instance != "" || ts[3].Instance != inst || ts[5].Level != "error" {
		t.Errorf("toasts %+v", ts)
	}

	// Environment on, bad, off.
	for _, a := range []string{"env-on", "env-bad", "env-off"} {
		if err := click(t, h, a, a+"-btn", "", uctx); err != nil {
			t.Fatal(err)
		}
	}
	envs := h.Environments()
	if len(envs) != 3 || envs[0].Env["SHOWCASE_ENV"] != "on" || len(envs[0].AddDirs) != 1 || envs[0].Scope == nil || !envs[0].Scope.NoProject ||
		envs[1].Env["PATH"] == "" || len(envs[2].Env) != 0 {
		t.Errorf("environments %+v", envs)
	}
	if p, err := loadPersisted(h.DataDir()); err != nil || p.Env.On {
		t.Errorf("persisted env %+v %v", p, err)
	}

	// Secrets: stored by a form, read (length only), set, delete.
	h.SetSecret(demoSecret, "hunter2")
	if err := h.Act(ctx, pluginsdk.SurfaceAct{Instance: inst, Kind: pluginsdk.ActSubmit, Action: "secret-save", Key: "secret-save-btn",
		Values: map[string]any{"demo": map[string]any{"key": demoSecret, "stored": true}}, Context: uctx}); err != nil {
		t.Fatal(err)
	}
	v = decode(t, h, inst)
	if rows := mustFind(t, v, "list-secrets").List.Rows; len(rows) != 1 || rows[0].Title != demoSecret {
		t.Errorf("secret rows %+v", rows)
	}
	if !mustFind(t, v, "form-secret").Form.Fields[0].Input.HasValue {
		t.Error("hasValue not set")
	}
	if err := click(t, h, "secret-read", "secret-read-btn", "", uctx); err != nil {
		t.Fatal(err)
	}
	if txt := mustFind(t, decode(t, h, inst), "secret-result").Text.Content.Spans[0].Text; txt != "demo is set: 7 characters (value withheld)" {
		t.Errorf("read: %q", txt)
	}
	if err := click(t, h, "secret-set", "secret-set-btn", "", uctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Secret("random"); !ok {
		t.Error("random not set")
	}
	if err := click(t, h, "secret-delete", "secret-delete-btn", demoSecret, uctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Secret(demoSecret); ok {
		t.Error("demo not deleted")
	}
	if strings.Contains(string(mustRaw(h, inst)), "hunter2") || strings.Contains(string(mustJSON(h.Logs())), "hunter2") {
		t.Error("the secret leaked")
	}

	// Subscribe, count events (snapshot first), unsubscribe.
	if err := click(t, h, "sub", "subscribe-threads", "threads", uctx); err != nil {
		t.Fatal(err)
	}
	subs := h.Subscriptions()
	if len(subs) != 1 || subs[0][0] != "threads" {
		t.Fatalf("subscriptions %+v", subs)
	}
	row := mustFind(t, decode(t, h, inst), "list-subs").List.Rows[0]
	id := row.TitleMeta.Spans[0].Text
	_ = h.Event(pluginsdk.Event{Subscription: id, Snapshot: true, Event: json.RawMessage(`{"threadList":{"snapshot":{}},"seq":"1","epoch":"e"}`)})
	_ = h.Event(pluginsdk.Event{Subscription: id, Event: json.RawMessage(`{"threadList":{"updated":{"id":"s-7"}},"seq":"2","epoch":"e"}`)})
	h.WaitFor(5*time.Second, func(h *plugintest.Host) bool {
		v := decode(t, h, inst)
		r := mustFind(t, v, "list-subs").List.Rows[0]
		return r.Trailing.Text == "2 events" && v.Tabs.Items[7].Badge != nil && v.Tabs.Items[7].Badge.Label == "2"
	})
	if err := click(t, h, "unsub", "unsub-threads", "threads", uctx); err != nil {
		t.Fatal(err)
	}
	if r := mustFind(t, decode(t, h, inst), "list-subs").List.Rows[0]; r.Glyph.Status != ui.StatusOff {
		t.Errorf("after unsubscribe %+v", r)
	}

	// Invoke with the act's context; a refused command keeps its code.
	for _, c := range []string{"pr.checks", "thread.send", "thread.archive"} {
		if err := click(t, h, "invoke", "inv-x", c, uctx); err != nil {
			t.Fatal(err)
		}
	}
	inv := h.Invocations()
	if len(inv) != 3 || inv[0].Context.ThreadID != "s-7" || inv[1].Args["text"] == "" {
		t.Errorf("invocations %+v", inv)
	}
	v = decode(t, h, inst)
	if mustFind(t, v, "inv-res-pr-checks-json").Log.Text == "" {
		t.Error("pr.checks JSON not shown")
	}
	denied := mustFind(t, v, "inv-res-thread-archive-list").List.Rows[0]
	if denied.Glyph.Status != ui.StatusFailed || !strings.Contains(denied.Line2.Spans[0].Text, "-32001") {
		t.Errorf("denied row %+v", denied.Line2)
	}
}

func TestGatedSurface(t *testing.T) {
	ctx := testCtx(t)
	s := fast()
	h := start(t, s, plugintest.Options{Granted: every})
	if _, err := h.Command(ctx, "gate", map[string]string{"state": "on", "scope": "thread"}, pluginsdk.UiContext{ThreadID: "s-3"}); err != nil {
		t.Fatal(err)
	}
	const gi = "showcase/gated/thread:s-3"
	if err := h.OpenSurfaceWith(ctx, pluginsdk.SurfaceOpen{Instance: gi, Surface: "gated", Context: pluginsdk.UiContext{ThreadID: "s-3"},
		Facts: map[string]json.RawMessage{"showcase.on": json.RawMessage("true")}}); err != nil {
		t.Fatal(err)
	}
	v := decode(t, h, gi)
	if got := pairValue(t, mustFind(t, v, "gated-facts"), "gated-fact"); got != "true" {
		t.Errorf("fact %q", got)
	}
	if b := mustFind(t, v, "gated-bar").Actions.Buttons[0]; b.Action.Command.Name != "plugin.showcase.gate" || b.Action.Command.Args["state"] != "off" {
		t.Errorf("gate off button %+v", b)
	}
	if err := h.Act(ctx, pluginsdk.SurfaceAct{Instance: gi, Kind: pluginsdk.ActClick, Action: "x"}); err == nil {
		t.Error("gated accepted an event")
	}
}

// The gate survives a restart: Init sets the facts again from the data dir.
func TestGateSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := testCtx(t)
	h := start(t, fast(), plugintest.Options{Granted: every, DataDir: dir})
	for _, a := range []map[string]string{{"state": "on", "scope": "thread"}, {"state": "on", "scope": "global"}} {
		if _, err := h.Command(ctx, "gate", a, pluginsdk.UiContext{ThreadID: "s-5"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	h2 := start(t, fast(), plugintest.Options{Granted: every, DataDir: dir})
	h2.WaitFor(5*time.Second, func(h *plugintest.Host) bool { return len(h.Contexts()) == 2 })
	got := map[string]bool{}
	for _, c := range h2.Contexts() {
		got[c.ThreadID] = c.Value == true
	}
	if !got[""] || !got["s-5"] {
		t.Errorf("contexts after restart %+v", h2.Contexts())
	}
}
