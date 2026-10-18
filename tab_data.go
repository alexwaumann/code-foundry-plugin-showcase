package main

import (
	"context"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

var dataTab = tabDef{
	key: tabData, label: "Data",
	short: "Pairs, row variants, dense, chips",
	look:  "Pairs, every row and glyph variant (click a row to select it), dense vs normal, and every chip status.",
	nodes: dataNodes,
	acts: map[string]actFunc{
		"select-row":  selectRow,
		"select-chip": selectChip,
		"row-hide":    func(ctx context.Context, g *gallery, _ pluginsdk.SurfaceAct) error { return g.render() },
	},
}

func selectRow(_ context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	if g.selectedRow == act.Data {
		g.selectedRow = ""
	} else {
		g.selectedRow = act.Data
	}
	g.s.mu.Unlock()
	return g.render()
}

func selectChip(_ context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	g.selectedChip = act.Data
	g.s.mu.Unlock()
	return g.render()
}

var statuses = []ui.Status{ui.StatusOK, ui.StatusFailed, ui.StatusAttention, ui.StatusRunning, ui.StatusPending, ui.StatusSkipped, ui.StatusOff}

func dataNodes(g *gallery) []ui.Node {
	base := g.opened
	statusRows := func(prefix string, withLine2 bool) []ui.Row {
		rows := make([]ui.Row, 0, len(statuses)+1)
		for i, st := range statuses {
			key := prefix + "-" + string(st)
			r := ui.Row{Key: key, Glyph: ui.StatusGlyph(st), Title: "Status " + string(st),
				Action: ui.EventAction("select-row", key), Selected: g.selectedRow == key}
			switch st {
			case ui.StatusRunning:
				r.TitleMeta = ui.Plain("push")
				r.Trailing = ui.Spans(ui.At(base.Add(-42*time.Second), ui.TimeElapsed))
			case ui.StatusFailed:
				r.Pill = ui.NewPill("failed", ui.ToneError)
				r.Trailing = ui.Spans(ui.For(time.Duration(i+1) * 31 * time.Second))
			case ui.StatusAttention:
				r.Pill = ui.NewPill("needs you", ui.ToneAttention)
			case ui.StatusOK:
				r.Pill = ui.NewPill("passed", ui.ToneOK)
				r.Trailing = ui.Spans(ui.For(95 * time.Second))
			default:
				r.Trailing = ui.Plain("#" + string(rune('1'+i)))
			}
			if withLine2 {
				r.Line2 = ui.Spans(ui.T("line 2: started "), ui.At(base.Add(-time.Duration(i+1)*time.Minute), ui.TimeRelative), ui.T(" · "), ui.T("main").AsMono())
			}
			rows = append(rows, r)
		}
		return rows
	}

	buttonsRows := []ui.Row{
		{Key: "row-btn-3", Glyph: ui.StatusGlyph(ui.StatusRunning), Title: "Three trailing buttons",
			TitleMeta: ui.Plain("icon-only + labelled"),
			Line2:     ui.Plain("Stop (destructive, command with confirm), Hide (event with data), Open (URL)"),
			Actions: []ui.Button{
				ui.CommandButton("row-btn-3-stop", "Stop", "plugin.showcase.boom", map[string]string{"what": "the row's run"}).WithIcon("square").AsIconOnly().WithStyle(ui.ButtonDestructive),
				ui.EventButton("row-btn-3-hide", "Hide", "row-hide").WithData("row-btn-3"),
				ui.URLButton("row-btn-3-open", "Open on GitHub", "https://github.com/alexwaumann/code-foundry-plugin-showcase").WithIcon("external-link").AsIconOnly(),
			}},
		{Key: "row-btn-long", Glyph: ui.IconGlyph("file-text", ui.ToneMuted),
			Title:     "A very long row title that must truncate or wrap at 280 px without pushing the buttons out of the row",
			TitleMeta: ui.Plain("long meta text too"),
			Pill:      ui.NewPill("long pill", ui.ToneAttention),
			Line2:     ui.Spans(ui.T("/a/long/path/in/line2/that/also/needs/to/truncate/cleanly.go").AsMono()),
			Trailing:  ui.Spans(ui.For(3 * time.Hour)),
			Actions:   []ui.Button{ui.EventButton("row-btn-long-hide", "Hide", "row-hide").WithData("row-btn-long").WithIcon("x").AsIconOnly()}},
		{Key: "row-plain", Title: "No glyph, no line 2, no buttons", Action: ui.EventAction("select-row", "row-plain"), Selected: g.selectedRow == "row-plain"},
	}

	iconRows := []ui.Row{}
	for _, it := range []struct {
		icon string
		tone ui.Tone
	}{{"package", ui.ToneNeutral}, {"clock", ui.ToneMuted}, {"triangle-alert", ui.ToneAttention}, {"circle-x", ui.ToneError}, {"circle-check", ui.ToneOK}, {"not-an-icon", ui.ToneNeutral}} {
		key := "row-icon-" + it.icon
		tone := string(it.tone)
		if tone == "" {
			tone = "neutral"
		}
		iconRows = append(iconRows, ui.Row{Key: key, Glyph: ui.IconGlyph(it.icon, it.tone), Title: it.icon, TitleMeta: ui.Plain(tone),
			Action: ui.EventAction("select-row", key), Selected: g.selectedRow == key})
	}
	iconRows = append(iconRows, ui.Row{Key: "row-icon-status", Glyph: ui.StatusGlyph(ui.StatusFailed), Title: "status glyph in icon style", TitleMeta: ui.Plain("failed")})

	denseRows := statusRows("dense", true)
	denseRows = append(denseRows, ui.Row{Key: "dense-btns", Glyph: ui.StatusGlyph(ui.StatusRunning), Title: "Dense with buttons",
		Line2: ui.Plain("hidden in dense"), Trailing: ui.Plain("trailing"),
		Actions: []ui.Button{
			ui.EventButton("dense-btns-label", "Labelled (hidden)", "row-hide").WithData("dense-btns"),
			ui.EventButton("dense-btns-icon", "Icon-only (shown)", "row-hide").WithData("dense-btns").WithIcon("x").AsIconOnly(),
		}})

	chipList := make([]ui.Chip, 0, len(statuses)+2)
	for i, st := range statuses {
		key := "chip-" + string(st)
		c := ui.Chip{Key: key, Label: string(st), Status: st, Selected: g.selectedChip == key, Action: ui.EventAction("select-chip", key)}
		if i%2 == 0 {
			c.Meta = ui.Spans(ui.For(time.Duration(i+1) * 13 * time.Second))
		}
		chipList = append(chipList, c)
	}
	chipList = append(chipList,
		ui.Chip{Key: "chip-none", Label: "no status", Action: ui.EventAction("select-chip", "chip-none"), Selected: g.selectedChip == "chip-none"},
		ui.Chip{Key: "chip-static", Label: "not clickable", Status: ui.StatusOK})

	selected := g.selectedRow
	if selected == "" {
		selected = "none"
	}
	chipSel := g.selectedChip
	if chipSel == "" {
		chipSel = "none"
	}

	return []ui.Node{
		ui.NewSection("sec-pairs", "Pairs",
			ui.NewPairs("pairs-kinds",
				ui.NewPair("pr-plain", "Plain", ui.Plain("a plain value")),
				ui.Pair{Key: "pr-mono", Label: "Mono, copyable", Value: ui.Spans(ui.T("cf/showcase-branch").AsMono()), Copyable: true},
				ui.Pair{Key: "pr-pill", Label: "Pill", Pill: ui.NewPill("passing", ui.ToneOK)},
				ui.Pair{Key: "pr-both", Label: "Value and pill", Value: ui.Plain("3 of 5"), Pill: ui.NewPill("2 failing", ui.ToneError)},
				ui.NewPair("pr-time", "Time", ui.Spans(ui.At(base.Add(-3*time.Minute), ui.TimeRelative))),
				ui.Pair{Key: "pr-elapsed", Label: "Elapsed, copyable", Value: ui.Spans(ui.At(base, ui.TimeElapsed)), Copyable: true},
				ui.NewPair("pr-duration", "Duration", ui.Spans(ui.For(95*time.Second))),
				ui.NewPair("pr-link", "Link", ui.Spans(ui.T("octo/app#12").Link("https://github.com/alexwaumann/code-foundry/pull/1"))),
				ui.NewPair("pr-tone", "Toned", ui.Spans(ui.T("2 failed").WithTone(ui.ToneError), ui.T(" · 1 waiting").WithTone(ui.ToneAttention))),
				ui.NewPair("pr-long-label", "A label that is forty characters long!!!", ui.Plain("value beside a long label")),
				ui.Pair{Key: "pr-long", Label: "Long value", Value: ui.Spans(ui.T("/Users/someone/.code-foundry/worktrees/a-very-long-project-name/cf-a-very-long-branch-name").AsMono()), Copyable: true},
			),
		),
		ui.NewText("txt-selected", ui.Spans(ui.T("Selected row: ").WithTone(ui.ToneMuted), ui.T(selected).AsMono(), ui.T(" · chip: ").WithTone(ui.ToneMuted), ui.T(chipSel).AsMono())),
		ui.NewSection("sec-list", "List: every status (dots)",
			ui.Node{Key: "list-status", List: &ui.List{Rows: statusRows("st", true)}}),
		ui.NewSection("sec-list-buttons", "List: buttons, long text, plain",
			ui.Node{Key: "list-buttons", List: &ui.List{Rows: buttonsRows}}),
		ui.NewSection("sec-list-icons", "List: icon glyphs",
			ui.Node{Key: "list-icons", List: &ui.List{Rows: iconRows, GlyphStyle: ui.GlyphIcon}}),
		ui.NewSection("sec-list-dense", "List: dense",
			ui.Node{Key: "list-dense", List: &ui.List{Rows: denseRows, Dense: true, GlyphStyle: ui.GlyphIcon}}),
		ui.NewSection("sec-list-empty", "List: empty",
			ui.Node{Key: "list-empty", List: &ui.List{Rows: []ui.Row{}, EmptyText: "No rows: this is the list's emptyText"}}),
		ui.NewSection("sec-chips", "Chips: every status",
			ui.NewChips("chips-all", chipList...)),
		ui.NewSection("sec-chips-2", "Chips: a second group",
			ui.NewChips("chips-pipeline",
				ui.Chip{Key: "pipe-build", Label: "Build", Status: ui.StatusOK, Meta: ui.Spans(ui.For(31 * time.Second))},
				ui.Chip{Key: "pipe-test", Label: "Test", Status: ui.StatusRunning, Meta: ui.Spans(ui.At(base, ui.TimeElapsed))},
				ui.NewChip("pipe-deploy", "Deploy", ui.StatusPending),
				ui.Chip{Key: "pipe-long", Label: "a chip label of 32 characters!!!", Status: ui.StatusSkipped})),
	}
}
