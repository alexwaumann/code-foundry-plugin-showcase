package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

var ownedTab = tabDef{
	key: tabOwned, label: "Owned",
	short: "Plugin thread and terminal nodes",
	look:  "The plugin's own threads (editable, read-only) and a terminal ticking every second; Read scrollback counts its whole output; Open in bottom panel; nothing here shows in the sidebar.",
	nodes: ownedNodes,
	acts: map[string]actFunc{
		"owned-ask":      askHelper,
		"owned-narrate":  narrate,
		"owned-archive":  archiveOwned,
		"owned-recreate": recreateOwned,
		"term-restart":   restartTerminal,
		"term-stop":      stopTerminal,
		"term-read":      readScrollback,
		"owned-refresh":  refreshOwnedAct,
		"owned-variant":  pickVariant,
	},
}

const (
	helperTitle   = "Showcase helper"
	narratorTitle = "Showcase narrator"
	tickerTitle   = "showcase ticker"
)

// tickerScript prints a line a second and SHOWCASE_TERM from the terminal's env.
const tickerScript = `echo "ticker in $(pwd) with SHOWCASE_TERM=$SHOWCASE_TERM"; i=0; while :; do i=$((i+1)); printf 'tick %3d  %s\n' "$i" "$(date +%H:%M:%S)"; sleep 1; done`

const askPrompt = "Use your showcase-hello skill: which SHOWCASE_* variables do you see, and where did the skill come from? Two short sentences."
const narratePrompt = "In one short sentence, say hello from the showcase plugin's read-only narrator thread."

// ensureOwned creates the helper and narrator threads and the ticker terminal when they
// do not exist yet (the Owned tab's first view), then re-renders.
func (s *showcase) ensureOwned(ctx context.Context) {
	s.refreshOwned(ctx)
	s.mu.Lock()
	if s.owned.Creating {
		s.mu.Unlock()
		return
	}
	need := s.owned.Helper == "" || s.owned.Narrator == "" || s.owned.Terminal == ""
	s.owned.Creating = need
	h := s.h
	helper, narrator, term := s.owned.Helper, s.owned.Narrator, s.owned.Terminal
	s.mu.Unlock()
	if !need {
		s.renderAll()
		return
	}
	s.renderAll()
	var errs []string
	data := h.DataDir()
	if helper == "" && h.Granted().Threads {
		if _, err := h.CreateThread(ctx, pluginsdk.ThreadCreateParams{Title: helperTitle, Cwd: data,
			AddDirs: []string{addonDir(data)}, Env: map[string]string{"SHOWCASE_THREAD": "helper", "SHOWCASE_ROLE": "editable"}}); err != nil {
			errs = append(errs, "helper thread: "+err.Error())
		}
	}
	if narrator == "" && h.Granted().Threads {
		if _, err := h.CreateThread(ctx, pluginsdk.ThreadCreateParams{Title: narratorTitle, Cwd: data,
			AddDirs: []string{addonDir(data)}, Env: map[string]string{"SHOWCASE_THREAD": "narrator", "SHOWCASE_ROLE": "read-only"}}); err != nil {
			errs = append(errs, "narrator thread: "+err.Error())
		}
	}
	if term == "" && h.Granted().Terminals {
		if _, err := h.CreateTerminal(ctx, pluginsdk.TerminalCreateParams{Argv: []string{"/bin/sh", "-c", tickerScript},
			Title: tickerTitle, Env: map[string]string{"SHOWCASE_TERM": "ticker"}}); err != nil {
			errs = append(errs, "terminal: "+err.Error())
		}
	}
	s.refreshOwned(ctx)
	s.mu.Lock()
	s.owned.Creating = false
	if len(errs) > 0 {
		s.owned.Err = fmt.Sprint(errs)
	} else {
		s.owned.Err = ""
	}
	s.mu.Unlock()
	s.renderAll()
}

func ownedNodes(g *gallery) []ui.Node {
	o := g.s.owned
	h := g.s.h
	nodes := []ui.Node{}
	if o.Err != "" {
		nodes = append(nodes, ui.Node{Key: "owned-err", Notice: &ui.Notice{Tone: ui.ToneError, Title: "Could not set up", Detail: o.Err}})
	}
	if h != nil && (!h.Granted().Threads || !h.Granted().Terminals) {
		nodes = append(nodes, ui.Node{Key: "owned-perm", Notice: &ui.Notice{Tone: ui.ToneAttention, Title: "Permissions missing",
			Body: ui.Plain("This tab needs the threads and terminals permissions.")}})
	}
	if o.Creating {
		nodes = append(nodes, ui.NewProgress("owned-creating", "Creating the plugin's threads and terminal", nil))
	}

	// Threads.
	threadInfo := ui.NewPairs("pairs-owned",
		ui.NewPair("own-cwd", "Cwd", ui.Spans(ui.T("plugin data dir").WithTone(ui.ToneMuted))),
		ui.NewPair("own-adddir", "Add-dir", ui.Spans(ui.T("addon/.claude/skills/showcase-hello").AsMono())),
		ui.NewPair("own-env", "Env", ui.Spans(ui.T("SHOWCASE_THREAD, SHOWCASE_ROLE").AsMono())),
		ui.NewPair("own-sent", "Sent by plugin", ui.Plain(fmt.Sprint(o.Sent))))
	helper := ui.NewEmpty("helper-none", "No helper thread yet", ui.Plain("Open this tab with the threads permission to create it."))
	if o.Helper != "" {
		helper = ui.Node{Key: "helper", Thread: &ui.Thread{ThreadID: o.Helper, Placeholder: "Ask the showcase helper…", Height: ui.HeightPx(420)}}
	}
	narrator := ui.NewEmpty("narrator-none", "No narrator thread yet", nil)
	if o.Narrator != "" {
		narrator = ui.Node{Key: "narrator", Thread: &ui.Thread{ThreadID: o.Narrator, ReadOnly: true, Height: ui.HeightPx(240)}}
	}
	// Each thread node holds its own stream in the GUI, so one at a time by default;
	// "both" is there to test the host with two (and the terminal) on screen.
	variant := g.threadVariant
	chip := func(key, label, v string) ui.Chip {
		return ui.Chip{Key: key, Label: label, Selected: variant == v, Action: ui.EventAction("owned-variant", v)}
	}
	threadNodes := []ui.Node{threadInfo,
		ui.NewChips("chips-variant", chip("var-editable", "Editable", "editable"), chip("var-readonly", "Read-only", "readonly"), chip("var-both", "Both (stress)", "both"))}
	if variant != "readonly" {
		threadNodes = append(threadNodes, ui.NewText("helper-label", ui.Spans(ui.T("Editable: composer, 420 px").AsStrong())), helper)
	}
	if variant != "editable" {
		threadNodes = append(threadNodes, ui.NewText("narrator-label", ui.Spans(ui.T("Read-only: no composer, 240 px").AsStrong())), narrator)
	}
	threadsSec := ui.NewSection("sec-threads", "Plugin threads", append(threadNodes,
		ui.NewActions("bar-owned",
			ui.EventButton("owned-ask-btn", "Ask the helper", "owned-ask").WithIcon("send").WithStyle(ui.ButtonOutline),
			ui.EventButton("owned-narrate-btn", "Narrate", "owned-narrate").WithIcon("message-square"),
			ui.EventButton("owned-refresh-btn", "Refresh list", "owned-refresh").WithIcon("refresh-cw").AsIconOnly(),
			ui.EventButton("owned-archive-btn", "Archive both", "owned-archive").WithStyle(ui.ButtonDestructive).WithIcon("trash-2"),
			ui.EventButton("owned-recreate-btn", "Recreate", "owned-recreate").WithIcon("rotate-ccw")))...)

	// The plugin's view of its own threads and terminals (thread/list, terminal/list).
	rows := []ui.Row{}
	for _, t := range o.Threads {
		st := ui.StatusOK
		switch t.Status {
		case "busy":
			st = ui.StatusRunning
		case "attention":
			st = ui.StatusAttention
		case "error":
			st = ui.StatusFailed
		}
		if t.State == "disconnected" {
			st = ui.StatusOff
		}
		r := ui.Row{Key: "lst-" + t.ThreadID, Glyph: ui.StatusGlyph(st), Title: t.Title, TitleMeta: ui.Spans(ui.T(short(t.ThreadID)).AsMono()),
			Line2: ui.Spans(ui.T(t.State+" · "+t.Status+" · "), ui.T(t.Model).AsMono(), ui.T(fmt.Sprintf(" · $%.4f", t.CostUSD)))}
		if t.Archived {
			r.Pill = ui.NewPill("archived", ui.ToneMuted)
		}
		rows = append(rows, r)
	}
	for _, t := range o.Terminals {
		st := ui.StatusRunning
		line2 := ui.Spans(ui.T("running since "), ui.At(t.StartedAt, ui.TimeRelative))
		if t.State == "exited" {
			st = ui.StatusOff
			line2 = ui.Spans(ui.T(fmt.Sprintf("exited %d", t.ExitCode)))
		}
		rows = append(rows, ui.Row{Key: "lst-" + t.TerminalID, Glyph: ui.StatusGlyph(st), Title: t.Title, TitleMeta: ui.Spans(ui.T(short(t.TerminalID)).AsMono()), Line2: line2})
	}
	listSec := ui.NewSection("sec-owned-list", "thread/list and terminal/list",
		ui.Node{Key: "list-owned", List: &ui.List{Rows: rows, EmptyText: "Nothing yet"}})
	listSec.Section.Collapsible, listSec.Section.Collapsed = true, true

	// Terminal.
	term := ui.Node{Key: "term-none", Empty: &ui.Empty{Glyph: ui.IconGlyph("terminal", ui.ToneMuted), Title: "No terminal",
		Body:    ui.Plain("Start one: a shell loop that ticks every second."),
		Actions: []ui.Button{ui.EventButton("term-start-btn", "Start", "term-restart").WithIcon("play").WithStyle(ui.ButtonOutline)}}}
	termNodes := []ui.Node{term}
	if o.Terminal != "" {
		termNodes = []ui.Node{{Key: "term", Terminal: &ui.Terminal{TerminalID: o.Terminal, Title: tickerTitle, Rows: 10,
			Actions: []ui.Button{
				ui.EventButton("term-read-btn", "Read scrollback", "term-read").WithIcon("file-text"),
				ui.EventButton("term-stop-btn", "Stop", "term-stop").WithIcon("square").WithStyle(ui.ButtonDestructive),
			}}}}
		if r := o.Read; r.Terminal == o.Terminal && !r.At.IsZero() {
			termNodes = append(termNodes, readNode(r))
		}
		termNodes = append(termNodes, ui.NewActions("bar-term", ui.EventButton("term-restart-btn", "Restart", "term-restart").WithIcon("rotate-ccw")))
	}
	termSec := ui.NewSection("sec-term", "Plugin terminal (10 rows)", termNodes...)
	return append(nodes, threadsSec, termSec, listSec)
}

// termRead is what the last Read scrollback found: terminal/read with the scrollback
// and without, to compare.
type termRead struct {
	Terminal         string
	Lines, Screen    int
	Truncated        bool
	First, Last, Err string
	At               time.Time
}

func readNode(r termRead) ui.Node {
	if r.Err != "" {
		return ui.Node{Key: "term-read", Text: &ui.Text{Size: ui.TextSm, Content: ui.Spans(ui.T(r.Err).WithTone(ui.ToneError))}}
	}
	spans := []ui.Span{ui.T(fmt.Sprintf("%d lines with the scrollback, %d on the screen", r.Lines, r.Screen)).AsStrong()}
	if r.Truncated {
		spans = append(spans, ui.T(" (its head cut at 1 MB)"))
	}
	spans = append(spans, ui.T(" · "), ui.At(r.At, ui.TimeRelative))
	if r.First != "" {
		spans = append(spans, ui.T(" · first "), ui.T(clipRunes(r.First, 60)).AsMono())
	}
	return ui.Node{Key: "term-read", Text: &ui.Text{Size: ui.TextSm, Content: ui.Spans(spans...)}}
}

// readScrollback reads the ticker's whole output (terminal/read with the scrollback) and
// its screen, and shows how many lines each has.
func readScrollback(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	s := g.s
	s.mu.Lock()
	id, h := s.owned.Terminal, s.h
	s.mu.Unlock()
	if id == "" {
		return fmt.Errorf("no terminal to read: start it first")
	}
	r := termRead{Terminal: id, At: s.now()}
	if !h.HasCapability("terminal.scrollback") {
		r.Err = "This Code Foundry cannot read a terminal's scrollback (no terminal.scrollback capability)."
	} else if all, err := h.ReadTerminalWith(ctx, id, pluginsdk.ReadTerminalOptions{Scrollback: true}); err != nil {
		r.Err = "terminal/read: " + err.Error()
	} else {
		r.Lines, r.Truncated = all.Lines, all.Truncated
		lines := strings.Split(all.Text, "\n")
		r.First, r.Last = lines[0], lines[len(lines)-1]
		// The screen's rows below the last output are empty: not lines.
		if screen, err := h.ReadTerminal(ctx, id, 0); err == nil && strings.TrimRight(screen.Text, "\n") != "" {
			r.Screen = strings.Count(strings.TrimRight(screen.Text, "\n"), "\n") + 1
		}
	}
	s.mu.Lock()
	s.owned.Read = r
	s.mu.Unlock()
	return g.render()
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func askHelper(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	return g.s.sendOwned(ctx, g, func(o ownedState) string { return o.Helper }, askPrompt)
}

func narrate(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	return g.s.sendOwned(ctx, g, func(o ownedState) string { return o.Narrator }, narratePrompt)
}

func (s *showcase) sendOwned(ctx context.Context, g *gallery, pick func(ownedState) string, text string) error {
	s.mu.Lock()
	id, h := pick(s.owned), s.h
	s.mu.Unlock()
	if id == "" {
		return fmt.Errorf("that thread does not exist yet")
	}
	if err := h.SendThread(ctx, id, text); err != nil {
		return fmt.Errorf("sending: %w", err)
	}
	s.mu.Lock()
	s.owned.Sent++
	s.mu.Unlock()
	s.refreshOwned(ctx)
	return g.render()
}

func archiveOwned(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	s := g.s
	s.mu.Lock()
	ids, h := []string{s.owned.Helper, s.owned.Narrator}, s.h
	s.mu.Unlock()
	for _, id := range ids {
		if id != "" {
			if err := h.ArchiveThread(ctx, id); err != nil {
				return fmt.Errorf("archiving %s: %w", short(id), err)
			}
		}
	}
	s.refreshOwned(ctx)
	return g.render()
}

func recreateOwned(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	go g.s.ensureOwned(context.WithoutCancel(ctx))
	return nil
}

func refreshOwnedAct(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.refreshOwned(ctx)
	return g.render()
}

func stopTerminal(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	s := g.s
	s.mu.Lock()
	id, h := s.owned.Terminal, s.h
	s.mu.Unlock()
	if id != "" {
		if err := h.CloseTerminal(ctx, id); err != nil {
			return fmt.Errorf("closing the terminal: %w", err)
		}
	}
	s.refreshOwned(ctx)
	return g.render()
}

func restartTerminal(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	if err := stopTerminal(ctx, g, act); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	go g.s.ensureOwned(context.WithoutCancel(ctx))
	return nil
}

func pickVariant(_ context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	switch act.Data {
	case "editable", "readonly", "both":
	default:
		return fmt.Errorf("unknown variant %q", act.Data)
	}
	g.s.mu.Lock()
	g.threadVariant = act.Data
	g.s.mu.Unlock()
	return g.render()
}
