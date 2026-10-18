package main

import (
	"context"
	"fmt"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

var statusTab = tabDef{
	key: tabStatus, label: "Status",
	look:  "Progress bars, a notice per tone, empty states, and sections: titled, collapsible, card tones, stale (dimmed), nested two deep.",
	nodes: statusNodes,
	acts: map[string]actFunc{
		"progress-step": stepProgress,
		"progress-fail": failProgress,
		"stale-toggle":  toggleStale,
		"status-noop":   func(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error { return g.render() },
	},
}

func stepProgress(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	g.progress += 0.1
	if g.progress > 1.0001 {
		g.progress = 0
	}
	g.s.mu.Unlock()
	return g.render()
}

func failProgress(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	g.progressFail = !g.progressFail
	g.s.mu.Unlock()
	return g.render()
}

func toggleStale(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	g.stale = !g.stale
	g.s.mu.Unlock()
	return g.render()
}

func statusNodes(g *gallery) []ui.Node {
	pct := g.progress
	if pct > 1 {
		pct = 1
	}
	det := ui.Node{Key: "prog-det", Progress: &ui.Progress{Value: ui.Fraction(pct), Label: "Uploading artifacts",
		Meta: ui.Plain(fmt.Sprintf("%.0f of 100 MB", pct*100))}}
	if g.progressFail {
		det.Progress.Tone = ui.ToneError
		det.Progress.Label = "Upload failed"
	}
	noop := func(key, label string) ui.Button { return ui.EventButton(key, label, "status-noop") }

	stale := ui.NewSection("sec-stale", "Stale section", ui.NewPairs("pairs-stale",
		ui.NewPair("stale-a", "Last known", ui.Plain("3 runs, 1 failing")),
		ui.Pair{Key: "stale-b", Label: "Status", Pill: ui.NewPill("failing", ui.ToneError)}))
	stale.Section.Stale = g.stale
	stale.Section.Meta = ui.Plain("toggle with the button")
	if g.stale {
		stale.Section.Pill = ui.NewPill("stale", ui.ToneMuted)
	}
	stale.Section.Actions = []ui.Button{ui.EventButton("stale-btn", "Toggle stale", "stale-toggle").WithIcon("refresh-cw")}

	titled := ui.NewSection("sec-titled", "Titled section with meta, pill and 2 buttons",
		ui.NewText("titled-body", ui.Plain("The title row carries a meta, a pill and up to two buttons.")))
	titled.Section.Meta, titled.Section.Pill = ui.Plain("last 24 h"), ui.NewPill("3", ui.ToneNeutral)
	titled.Section.Actions = []ui.Button{noop("titled-a", "Refresh").WithIcon("refresh-cw").AsIconOnly(), noop("titled-b", "More")}

	collapsed := ui.NewSection("sec-collapsed", "Collapsible, starts collapsed",
		ui.NewText("collapsed-body", ui.Plain("Hidden until opened; the host remembers your toggle across views.")))
	collapsed.Section.Collapsible, collapsed.Section.Collapsed = true, true
	collapsed.Section.Pill = ui.NewPill("2", ui.ToneNeutral)

	open := ui.NewSection("sec-open", "Collapsible, starts open",
		ui.NewText("open-body", ui.Plain("Collapse me, then press Re-render in the header: I stay collapsed.")))
	open.Section.Collapsible = true

	card := func(key, title string, tone ui.Tone) ui.Node {
		n := ui.NewSection(key, title, ui.NewText(key+"-body", ui.Plain("A card draws a box; its tone colours the border.")))
		n.Section.Card, n.Section.Tone = true, tone
		return n
	}

	inner := ui.NewSection("sec-inner", "Inner section (depth 2)",
		ui.NewText("inner-body", ui.Plain("section > section > node is the deepest allowed.")))
	inner.Section.Collapsible = true
	nested := ui.NewSection("sec-outer", "Outer section", ui.NewText("outer-body", ui.Plain("Outer text.")), inner)
	nested.Section.Card = true

	return []ui.Node{
		ui.NewSection("sec-progress", "Progress",
			det,
			ui.NewProgress("prog-indet", "Waiting for a runner", nil),
			ui.Node{Key: "prog-indet-meta", Progress: &ui.Progress{Label: "Indeterminate with meta", Meta: ui.Spans(ui.T("queued "), ui.At(g.opened, ui.TimeElapsed))}},
			ui.Node{Key: "prog-zero", Progress: &ui.Progress{Value: ui.Fraction(0), Label: "Zero"}},
			ui.Node{Key: "prog-full", Progress: &ui.Progress{Value: ui.Fraction(1), Label: "Full"}},
			ui.Node{Key: "prog-bare", Progress: &ui.Progress{Value: ui.Fraction(0.3)}},
			ui.NewActions("prog-bar",
				ui.EventButton("prog-step-btn", "Advance 10%", "progress-step").WithIcon("play"),
				ui.EventButton("prog-fail-btn", "Toggle failed", "progress-fail")),
		),
		ui.NewSection("sec-notices", "Notices",
			ui.Node{Key: "notice-neutral", Notice: &ui.Notice{Icon: "info", Title: "Neutral notice", Body: ui.Plain("Something to know; no colour.")}},
			ui.Node{Key: "notice-attention", Notice: &ui.Notice{Tone: ui.ToneAttention, Icon: "key-round", Title: "Sign in needed",
				Body: ui.Plain("The CI server wants a token."), Detail: "HTTP 401 from https://ci.example.com/api/v4/projects/42/pipelines?ref=main",
				Actions: []ui.Button{noop("notice-att-go", "Add a token").WithStyle(ui.ButtonAttention), noop("notice-att-later", "Later")}}},
			ui.Node{Key: "notice-error", Notice: &ui.Notice{Tone: ui.ToneError, Icon: "circle-x", Title: "Build failed",
				Body:   ui.Spans(ui.T("Step "), ui.T("go test ./...").AsMono(), ui.T(" exited 1.")),
				Detail: "--- FAIL: TestGallery (0.01s)\n    gallery_test.go:42: view refused: nodes[3].list.rows[0].title: required\nFAIL\nexit status 1",
				Actions: []ui.Button{noop("notice-err-a", "Retry").WithIcon("rotate-ccw"), noop("notice-err-b", "Open log").WithStyle(ui.ButtonLink),
					ui.URLButton("notice-err-c", "Docs", "https://github.com/alexwaumann/code-foundry").WithIcon("external-link").AsIconOnly()}}},
			ui.Node{Key: "notice-bare", Notice: &ui.Notice{Body: ui.Plain("A notice with only a body: no title, no icon.")}},
		),
		ui.NewSection("sec-empties", "Empty states",
			ui.Node{Key: "empty-full", Empty: &ui.Empty{Glyph: ui.IconGlyph("package", ui.ToneMuted), Title: "No runs",
				Body: ui.Plain("Push a commit to start one."), Actions: []ui.Button{noop("empty-full-a", "Refresh").WithIcon("refresh-cw"), ui.URLButton("empty-full-b", "Docs", "https://github.com/alexwaumann/code-foundry")}}},
			ui.Node{Key: "empty-status", Empty: &ui.Empty{Glyph: ui.StatusGlyph(ui.StatusRunning), Title: "Loading runs"}},
			ui.Node{Key: "empty-title", Empty: &ui.Empty{Title: "Only a title"}},
		),
		ui.NewSection("sec-sections", "Sections", titled, collapsed, open, stale,
			card("sec-card", "Card", ui.ToneNeutral), card("sec-card-att", "Card, attention", ui.ToneAttention), card("sec-card-err", "Card, error", ui.ToneError)),
		nested,
	}
}
