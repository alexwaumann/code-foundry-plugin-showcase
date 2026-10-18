package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

var hostTab = tabDef{
	key: tabHost, label: "Host",
	short: "Host calls on this panel's thread",
	look:  "Each button calls the host on the thread this panel is beside: flags in the sidebar, toasts, the Gated tab, env, Keychain, event counts, host commands.",
	nodes: hostNodes,
	acts: map[string]actFunc{
		"flag-seen":     flagUntilSeen,
		"toast":         toast,
		"env-on":        envOn,
		"env-off":       envOff,
		"env-bad":       envBad,
		"secret-save":   secretSave,
		"secret-read":   secretRead,
		"secret-set":    secretSet,
		"secret-delete": secretDelete,
		"sub":           subAct,
		"unsub":         unsubAct,
		"invoke":        invokeAct,
	},
}

const demoSecret = "demo"

func hostNodes(g *gallery) []ui.Node {
	s := g.s
	thread := ""
	if g.lastAct != nil {
		thread = g.lastAct.Context.ThreadID
	}
	nodes := []ui.Node{}
	if thread == "" {
		nodes = append(nodes, ui.Node{Key: "host-no-thread", Notice: &ui.Notice{Icon: "info", Title: "Which thread?",
			Body: ui.Plain("A global surface learns the panel's thread from each act. Click Refresh context (or any button) beside a thread first.")}})
	}
	nodes = append(nodes, ui.NewSection("sec-ctx", "Acting on",
		ui.NewPairs("pairs-ctx", g.actContextLocked()...),
		ui.NewActions("bar-ctx", ui.EventButton("ctx-refresh", "Refresh context", "refresh").WithIcon("refresh-cw"))))

	// Flags.
	flagPairs := []ui.Pair{ui.NewPair("flag-none", "Last flag", ui.Spans(ui.T("none set from here").WithTone(ui.ToneMuted)))}
	if f, ok := s.flags[thread]; ok && thread != "" {
		tone := ui.ToneNeutral
		switch f.Level {
		case pluginsdk.FlagAttention:
			tone = ui.ToneAttention
		case pluginsdk.FlagError:
			tone = ui.ToneError
		}
		flagPairs = []ui.Pair{{Key: "flag-level", Label: "Level", Pill: ui.NewPill(f.Level, tone)},
			ui.NewPair("flag-reason", "Reason", orNone(f.Reason, false)),
			ui.NewPair("flag-at", "Set", ui.Spans(ui.At(f.At, ui.TimeRelative), ui.T(map[bool]string{true: " · until seen", false: ""}[f.UntilSeen])))}
		if f.Err != "" {
			flagPairs = append(flagPairs, ui.NewPair("flag-err", "Refused", ui.Spans(ui.T(f.Err).WithTone(ui.ToneError))))
		}
	}
	nodes = append(nodes, ui.NewSection("sec-flags", "SetFlag (permission threadStatus)",
		ui.NewActions("bar-flags",
			ui.CommandButton("flag-att", "Attention", "plugin.showcase.flag", map[string]string{"level": "attention", "reason": "needs a look (showcase)"}).WithStyle(ui.ButtonAttention).WithIcon("flag"),
			ui.CommandButton("flag-err", "Error", "plugin.showcase.flag", map[string]string{"level": "error", "reason": "something failed (showcase)"}).WithIcon("circle-x"),
			ui.EventButton("flag-seen-btn", "Attention until seen", "flag-seen").WithIcon("eye"),
			ui.CommandButton("flag-clear", "Clear", "plugin.showcase.flag", map[string]string{"level": "none"}).WithIcon("x"),
		),
		ui.NewPairs("pairs-flag", flagPairs...)))

	// Toasts.
	nodes = append(nodes, ui.NewSection("sec-toasts", "Toast and ToastIn (panel)",
		ui.NewActions("bar-toasts",
			ui.EventButton("toast-info", "Info", "toast").WithData("app info"),
			ui.EventButton("toast-att", "Attention", "toast").WithData("app attention"),
			ui.EventButton("toast-err", "Error", "toast").WithData("app error"),
			ui.EventButton("toast-in-info", "Info here", "toast").WithData("panel info"),
			ui.EventButton("toast-in-att", "Attention here", "toast").WithData("panel attention"),
			ui.EventButton("toast-in-err", "Error here", "toast").WithData("panel error"),
		)))

	// Context facts and the gate.
	gateThread := s.gates.Threads[thread]
	gatePairs := ui.NewPairs("pairs-gate",
		ui.Pair{Key: "gate-thread", Label: "This thread", Pill: onOff(gateThread)},
		ui.Pair{Key: "gate-global", Label: "Everywhere", Pill: onOff(s.gates.Global)},
		ui.NewPair("gate-count", "Threads gated", ui.Plain(fmt.Sprint(len(s.gates.Threads)))))
	nodes = append(nodes, ui.NewSection("sec-gate", "SetContext: showcase.on",
		gatePairs,
		ui.NewActions("bar-gate",
			ui.CommandButton("gate-on", "Turn gate on", "plugin.showcase.gate", map[string]string{"state": "on"}).WithStyle(ui.ButtonOutline).WithIcon("lock-open"),
			ui.CommandButton("gate-off", "Turn gate off", "plugin.showcase.gate", map[string]string{"state": "off"}).WithIcon("lock"),
			ui.CommandButton("gate-all-on", "Gate everywhere", "plugin.showcase.gate", map[string]string{"state": "on", "scope": "global"}),
			ui.CommandButton("gate-all-off", "Ungate everywhere", "plugin.showcase.gate", map[string]string{"state": "off", "scope": "global"}),
		)))

	// Session environment.
	env := s.env
	envPairs := []ui.Pair{{Key: "env-state", Label: "State", Pill: onOff(env.On)}}
	if !env.At.IsZero() {
		envPairs = append(envPairs, ui.NewPair("env-at", "Pushed", ui.Spans(ui.At(env.At, ui.TimeRelative), ui.T(map[bool]string{true: " · applied", false: " · not applied"}[env.Applied]))))
	}
	if env.On {
		envPairs = append(envPairs, ui.NewPair("env-what", "Adds", ui.Spans(ui.T("SHOWCASE_ENV, PATH += bin, --add-dir addon").AsMono())))
	}
	envNodes := []ui.Node{ui.NewPairs("pairs-env", envPairs...)}
	if len(env.Issues) > 0 {
		envNodes = append(envNodes, ui.Node{Key: "env-issues", Notice: &ui.Notice{Tone: ui.ToneAttention, Title: fmt.Sprintf("%d issue(s): dropped, the rest applied", len(env.Issues)),
			Detail: strings.Join(env.Issues, "\n")}})
	}
	if env.Err != "" {
		envNodes = append(envNodes, ui.Node{Key: "env-err", Notice: &ui.Notice{Tone: ui.ToneError, Title: "Refused", Detail: env.Err}})
	}
	envNodes = append(envNodes,
		ui.NewText("env-how", ui.Spans(ui.T("Check: ").WithTone(ui.ToneMuted), ui.T("code-foundry plugin env showcase").AsMono(), ui.T(", or printenv in a new thread.").WithTone(ui.ToneMuted))),
		ui.NewActions("bar-env",
			ui.EventButton("env-on-btn", "Environment on", "env-on").WithIcon("play").WithStyle(ui.ButtonOutline),
			ui.EventButton("env-off-btn", "Off", "env-off").WithIcon("square"),
			ui.EventButton("env-bad-btn", "Push a bad one", "env-bad").WithIcon("triangle-alert")))
	nodes = append(nodes, ui.NewSection("sec-env", "SetEnvironment (permission sessionEnv)", envNodes...))

	// Secrets.
	secretField := ui.SecretField("f-demo-secret", "demo", "Demo secret", demoSecret)
	for _, si := range s.secrets.List {
		if si.Key == demoSecret {
			secretField.Input.HasValue = true
		}
	}
	saveSecret := ui.EventButton("secret-save-btn", "Store", "secret-save").WithIcon("key-round").WithStyle(ui.ButtonOutline)
	secretRows := []ui.Row{}
	for _, si := range s.secrets.List {
		secretRows = append(secretRows, ui.Row{Key: "secret-" + si.Key, Glyph: ui.IconGlyph("key", ui.ToneMuted), Title: si.Key,
			Line2:   ui.Spans(ui.T("set at "), ui.At(si.UpdatedAt, ui.TimeRelative)),
			Actions: []ui.Button{ui.EventButton("secret-del-"+si.Key, "Delete "+si.Key, "secret-delete").WithData(si.Key).WithIcon("trash-2").AsIconOnly().WithStyle(ui.ButtonDestructive)}})
	}
	secNodes := []ui.Node{
		ui.NewForm("form-secret", &saveSecret, secretField),
		ui.Node{Key: "list-secrets", List: &ui.List{Rows: secretRows, EmptyText: "No secrets stored"}},
		ui.NewActions("bar-secrets",
			ui.EventButton("secret-read-btn", "Read demo", "secret-read").WithIcon("eye"),
			ui.EventButton("secret-set-btn", "Set random from plugin", "secret-set").WithIcon("plus"),
			ui.EventButton("secret-delete-btn", "Delete demo", "secret-delete").WithData(demoSecret).WithIcon("trash-2").WithStyle(ui.ButtonDestructive)),
	}
	if s.secrets.Result != "" {
		tone := ui.ToneNeutral
		if s.secrets.Tone == "error" {
			tone = ui.ToneError
		}
		secNodes = append(secNodes, ui.NewText("secret-result", ui.Spans(ui.T(s.secrets.Result).WithTone(tone), ui.T(" · "), ui.At(s.secrets.At, ui.TimeRelative))))
	}
	nodes = append(nodes, ui.NewSection("sec-secrets", "Secrets (never shown)", secNodes...))

	// Events.
	subRows := []ui.Row{}
	for _, src := range sources {
		sub := s.subs[src]
		r := ui.Row{Key: "sub-" + src, Title: src, Trailing: ui.Plain(plural(sub.Count, "event"))}
		if sub.ID != "" {
			r.Glyph = ui.StatusGlyph(ui.StatusRunning)
			r.TitleMeta = ui.Spans(ui.T(sub.ID).AsMono())
			r.Actions = []ui.Button{ui.EventButton("unsub-"+src, "Unsubscribe", "unsub").WithData(src)}
		} else {
			r.Glyph = ui.StatusGlyph(ui.StatusOff)
			r.Actions = []ui.Button{ui.EventButton("subscribe-"+src, "Subscribe", "sub").WithData(src).WithStyle(ui.ButtonOutline)}
		}
		switch {
		case sub.Err != "":
			r.Line2 = ui.Spans(ui.T(sub.Err).WithTone(ui.ToneError))
		case !sub.LastAt.IsZero():
			r.Line2 = ui.Spans(ui.T(fmt.Sprintf("last %s · %d snapshots · ", sub.Last, sub.Snaps)), ui.At(sub.LastAt, ui.TimeRelative))
		default:
			r.Line2 = ui.Spans(ui.T("no events yet").WithTone(ui.ToneMuted))
		}
		subRows = append(subRows, r)
	}
	nodes = append(nodes, ui.NewSection("sec-events", "Subscribe: events per source",
		ui.Node{Key: "list-subs", List: &ui.List{Rows: subRows}}))

	// Invoke.
	invNodes := []ui.Node{ui.NewActions("bar-invoke",
		ui.EventButton("inv-checks", "pr.checks", "invoke").WithData("pr.checks").WithIcon("git-pull-request"),
		ui.EventButton("inv-send", "thread.send hello", "invoke").WithData("thread.send").WithIcon("send"),
		ui.EventButton("inv-url", "view.open.url", "invoke").WithData("view.open.url").WithIcon("globe"),
		ui.EventButton("inv-denied", "thread.archive (denied)", "invoke").WithData("thread.archive").WithIcon("ban"),
	)}
	names := make([]string, 0, len(s.invokes))
	for k := range s.invokes {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		r := s.invokes[k]
		key := "inv-res-" + strings.ReplaceAll(k, ".", "-")
		glyph, line := ui.StatusGlyph(ui.StatusOK), ui.Spans(ui.T(firstLine(r.Message)))
		if r.Err != "" {
			glyph, line = ui.StatusGlyph(ui.StatusFailed), ui.Spans(ui.T(fmt.Sprintf("%d: ", r.Code)).AsMono(), ui.T(r.Err).WithTone(ui.ToneError))
		}
		row := ui.Row{Key: key, Glyph: glyph, Title: k, TitleMeta: orNone(short(r.Thread), true), Line2: line, Trailing: ui.Spans(ui.At(r.At, ui.TimeRelative))}
		if r.Err != "" {
			row.Line2 = ui.Spans(ui.T(fmt.Sprintf("failed with %d", r.Code)).WithTone(ui.ToneError))
		}
		invNodes = append(invNodes, ui.Node{Key: key + "-list", List: &ui.List{Rows: []ui.Row{row}}})
		if r.Err != "" {
			// The whole error: a row's line 2 is one truncated line.
			invNodes = append(invNodes, ui.Node{Key: key + "-err", Text: &ui.Text{Content: ui.Spans(ui.T(r.Err).WithTone(ui.ToneError)), Size: ui.TextSm}})
		}
		if r.JSON != "" {
			invNodes = append(invNodes, ui.Node{Key: key + "-json", Log: &ui.Log{Title: k + " --json", Rows: 6, Wrap: true, Text: r.JSON}})
		}
	}
	nodes = append(nodes, ui.NewSection("sec-invoke", "Invoke with the act's context", invNodes...))
	return nodes
}

func onOff(on bool) *ui.Pill {
	if on {
		return ui.NewPill("on", ui.ToneOK)
	}
	return ui.NewPill("off", ui.ToneMuted)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	if s == "" {
		return "(no message)"
	}
	return s
}

func needThread(act pluginsdk.SurfaceAct) (string, error) {
	if act.Context.ThreadID == "" {
		return "", fmt.Errorf("this panel is not beside a thread")
	}
	return act.Context.ThreadID, nil
}

func flagUntilSeen(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	thread, err := needThread(act)
	if err != nil {
		return err
	}
	if _, err := g.s.setFlag(ctx, g.s.host(), thread, pluginsdk.FlagAttention, "until you open this thread (showcase)", true); err != nil {
		return err
	}
	return g.render()
}

func toast(_ context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	where, level, _ := strings.Cut(act.Data, " ")
	h := g.s.host()
	text := fmt.Sprintf("Showcase %s toast (%s)", level, where)
	var err error
	if where == "panel" {
		err = h.ToastIn(g.open.Instance, level, text)
	} else {
		err = h.Toast(level, text)
	}
	if err != nil {
		return err
	}
	return g.render()
}

func (s *showcase) pushEnv(ctx context.Context, g *gallery, env pluginsdk.Environment, on, bad bool) error {
	h := s.host()
	res, err := h.SetEnvironment(ctx, env)
	s.mu.Lock()
	s.env = envState{On: on, Bad: bad, Applied: res.Applied, Issues: res.Issues, At: s.now()}
	if err != nil {
		s.env.Err = err.Error()
	}
	s.saveLocked()
	s.mu.Unlock()
	return g.render()
}

func showcaseEnv(data string) pluginsdk.Environment {
	return pluginsdk.Environment{
		Env:         map[string]string{"SHOWCASE_ENV": "on", "SHOWCASE_FROM": "the showcase plugin's session environment"},
		PathPrepend: []string{binDir(data)},
		AddDirs:     []string{addonDir(data)},
		Scope:       &pluginsdk.EnvironmentScope{NoProject: true},
	}
}

func envOn(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	return g.s.pushEnv(ctx, g, showcaseEnv(g.s.host().DataDir()), true, false)
}

func envOff(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	return g.s.pushEnv(ctx, g, pluginsdk.Environment{}, false, false)
}

// envBad pushes one good variable and three the host drops with issues.
func envBad(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	env := pluginsdk.Environment{
		Env:         map[string]string{"SHOWCASE_ENV": "partly", "PATH": "/nope", "CODE_FOUNDRY_X": "reserved", "1BAD": "name"},
		PathPrepend: []string{"/tmp/outside-home"},
		AddDirs:     []string{"~/does-not-exist-showcase"},
		Scope:       &pluginsdk.EnvironmentScope{NoProject: true},
	}
	return g.s.pushEnv(ctx, g, env, true, true)
}

func (s *showcase) refreshSecrets(ctx context.Context) {
	h := s.host()
	if !h.Granted().Secrets {
		return
	}
	list, err := h.Secrets().List(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.secrets.Result, s.secrets.Tone, s.secrets.At = "list: "+err.Error(), "error", s.now()
		return
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Key < list[j].Key })
	s.secrets.List = list
}

func (s *showcase) secretResult(text, tone string) {
	s.mu.Lock()
	s.secrets.Result, s.secrets.Tone, s.secrets.At = text, tone, s.now()
	s.mu.Unlock()
}

func secretSave(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	if sk, ok := act.Stored("demo"); ok {
		g.s.secretResult("stored by the host under "+sk.Key, "")
	} else {
		g.s.secretResult("left empty: nothing stored", "")
	}
	g.s.refreshSecrets(ctx)
	return g.render()
}

func secretRead(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	v, found, err := g.s.host().Secrets().Get(ctx, demoSecret)
	switch {
	case err != nil:
		g.s.secretResult("get: "+err.Error(), "error")
	case !found:
		g.s.secretResult("demo is not set", "")
	default:
		g.s.secretResult(fmt.Sprintf("demo is set: %d characters (value withheld)", len(v)), "")
	}
	g.s.refreshSecrets(ctx)
	return g.render()
}

func secretSet(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	if err := g.s.host().Secrets().Set(ctx, "random", hex.EncodeToString(b)); err != nil {
		g.s.secretResult("set: "+err.Error(), "error")
	} else {
		g.s.secretResult("set random (16 characters) from the plugin", "")
	}
	g.s.refreshSecrets(ctx)
	return g.render()
}

func secretDelete(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	key := act.Data
	if key == "" {
		key = demoSecret
	}
	if err := g.s.host().Secrets().Delete(ctx, key); err != nil {
		g.s.secretResult("delete: "+err.Error(), "error")
	} else {
		g.s.secretResult("deleted "+key, "")
	}
	g.s.refreshSecrets(ctx)
	return g.render()
}

func subAct(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	if err := g.s.subscribe(ctx, act.Data); err != nil {
		return err
	}
	return g.render()
}

func unsubAct(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	if err := g.s.unsubscribe(ctx, act.Data); err != nil {
		return err
	}
	return g.render()
}

// invokeAct runs a host command with the acting panel's context and keeps the result.
func invokeAct(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	command := act.Data
	args := map[string]string{}
	switch command {
	case "thread.send":
		args["text"] = "Hello from the showcase plugin's Host tab (thread.send through host/invoke). Reply with one word."
	case "view.open.url":
		args["url"] = "https://github.com/alexwaumann/code-foundry-plugin-showcase"
	case "pr.checks", "thread.archive":
	default:
		return fmt.Errorf("unknown command %q", command)
	}
	if command != "view.open.url" {
		if _, err := needThread(act); err != nil {
			return err
		}
	}
	uctx := pluginsdk.UiContext{ThreadID: act.Context.ThreadID, ProjectID: act.Context.ProjectID, WorktreePath: act.Context.WorktreePath}
	res, err := g.s.host().Invoke(ctx, command, args, uctx)
	r := invokeResult{Command: command, Thread: act.Context.ThreadID, Message: res.Message, ExitCode: res.ExitCode, At: g.s.now()}
	if len(res.JSON) > 0 {
		r.JSON = string(res.JSON)
		if len(r.JSON) > 4000 {
			r.JSON = r.JSON[:4000] + "\n…"
		}
	}
	if err != nil {
		r.Err, r.Code = err.Error(), pluginsdk.ErrorCode(err)
	}
	g.s.mu.Lock()
	g.s.invokes[command] = r
	g.s.mu.Unlock()
	return g.render()
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
