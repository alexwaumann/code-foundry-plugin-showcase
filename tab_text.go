package main

import (
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

var textTab = tabDef{
	key: tabText, label: "Text",
	short: "Sizes, spans, ticking times, markdown",
	look:  "Every text size, inline span and time style; times tick on their own; markdown wraps at 280 px.",
	nodes: textNodes,
}

const markdownSample = "### Markdown\n\n" +
	"Plain, **bold**, *italic*, ~~struck~~ and `inline code`. A [link to the docs](https://github.com/alexwaumann/code-foundry) " +
	"and a bare URL: https://github.com/alexwaumann/code-foundry-plugin-showcase.\n\n" +
	"- a bullet\n- another, with a nested list\n  - nested one\n  - nested two\n\n" +
	"1. first\n2. second\n3. third\n\n" +
	"- [x] a done task\n- [ ] an open task\n\n" +
	"> A quote: explanations, not status.\n\n" +
	"```go\nfunc main() {\n\tfmt.Println(\"a fenced block that is wider than a 280 px panel should scroll, not wrap\")\n}\n```\n\n" +
	"| Table | Should |\n|---|---|\n| still | render |\n\n" +
	"<b>Raw HTML shows as text.</b>\n\n" +
	"---\n\nAfter a rule. A long word: supercalifragilisticexpialidocious-and-then-some-more-to-overflow."

func textNodes(g *gallery) []ui.Node {
	now := g.opened
	started := g.s.started
	if started.IsZero() {
		started = now
	}
	return []ui.Node{
		ui.NewSection("sec-sizes", "Sizes",
			ui.NewText("txt-base", ui.Plain("Base size (13 px): a sentence that wraps when the panel is narrow, as body text does.")),
			ui.Node{Key: "txt-sm", Text: &ui.Text{Content: ui.Plain("Small size (12 px, muted unless spans say otherwise)."), Size: ui.TextSm}},
			ui.Node{Key: "txt-lg", Text: &ui.Text{Content: ui.Plain("WDJB-MJHT"), Size: ui.TextLg, Copyable: true}},
			ui.Node{Key: "txt-copy", Text: &ui.Text{Content: ui.Spans(ui.T("~/.code-foundry/plugin-data/showcase").AsMono()), Copyable: true}},
			ui.Node{Key: "txt-sm-tone", Text: &ui.Text{Content: ui.Spans(ui.T("Small with a "), ui.T("red span").WithTone(ui.ToneError), ui.T(" inside.")), Size: ui.TextSm}},
			ui.NewText("txt-long", ui.Spans(ui.T("A long unbroken token: "), ui.T("/Users/someone/projects/a-very-long-folder-name/with/many/segments/and/no/spaces.go").AsMono())),
		),
		ui.NewSection("sec-spans", "Spans",
			ui.NewText("txt-tones", ui.Spans(
				ui.T("neutral "), ui.T("muted ").WithTone(ui.ToneMuted), ui.T("attention ").WithTone(ui.ToneAttention),
				ui.T("error ").WithTone(ui.ToneError), ui.T("ok (neutral outside pills)").WithTone(ui.ToneOK))),
			ui.NewText("txt-styles", ui.Spans(
				ui.T("strong").AsStrong(), ui.T(" · "), ui.T("mono").AsMono(), ui.T(" · "),
				ui.T("strong mono error").AsStrong().AsMono().WithTone(ui.ToneError), ui.T(" · "),
				ui.T("a link").Link("https://github.com/alexwaumann/code-foundry"), ui.T(" · "),
				ui.T("a mono link").AsMono().Link("https://github.com/alexwaumann/code-foundry-plugin-showcase"))),
		),
		ui.NewSection("sec-times", "Times and durations",
			ui.NewPairs("pairs-times",
				ui.NewPair("tm-rel", "Relative, past", ui.Spans(ui.At(now.Add(-90*time.Second), ui.TimeRelative))),
				ui.NewPair("tm-rel-old", "Relative, days", ui.Spans(ui.At(now.Add(-50*time.Hour), ui.TimeRelative))),
				ui.NewPair("tm-rel-future", "Relative, future", ui.Spans(ui.At(now.Add(72*time.Hour), ui.TimeRelative))),
				ui.NewPair("tm-elapsed", "Elapsed (ticks)", ui.Spans(ui.T("plugin up for "), ui.At(started, ui.TimeElapsed))),
				ui.NewPair("tm-countdown", "Countdown", ui.Spans(ui.At(now.Add(10*time.Minute), ui.TimeCountdown))),
				ui.NewPair("tm-short", "Countdown 20 s", ui.Spans(ui.At(now.Add(20*time.Second), ui.TimeCountdown), ui.T(" then expired").WithTone(ui.ToneMuted))),
				ui.NewPair("tm-expired", "Expired", ui.Spans(ui.At(now.Add(-time.Minute), ui.TimeCountdown))),
				ui.NewPair("du-ms", "Duration 450 ms", ui.Spans(ui.For(450*time.Millisecond))),
				ui.NewPair("du-s", "Duration 2.5 s", ui.Spans(ui.For(2500*time.Millisecond))),
				ui.NewPair("du-min", "Duration 95 s", ui.Spans(ui.For(95*time.Second))),
				ui.NewPair("du-h", "Duration 3 h", ui.Spans(ui.For(3*time.Hour+4*time.Minute+5*time.Second))),
				ui.NewPair("tm-mixed", "In a sentence", ui.Spans(ui.T("built "), ui.At(now.Add(-5*time.Minute), ui.TimeRelative), ui.T(" in "), ui.For(42*time.Second).WithTone(ui.ToneMuted))),
			),
		),
		ui.NewSection("sec-pills", "Pills",
			ui.NewPairs("pairs-pills",
				ui.Pair{Key: "pl-neutral", Label: "Neutral", Pill: ui.NewPill("draft", ui.ToneNeutral)},
				ui.Pair{Key: "pl-muted", Label: "Muted", Pill: ui.NewPill("skipped", ui.ToneMuted)},
				ui.Pair{Key: "pl-attention", Label: "Attention", Pill: ui.NewPill("needs you", ui.ToneAttention)},
				ui.Pair{Key: "pl-error", Label: "Error", Pill: ui.NewPill("failed", ui.ToneError)},
				ui.Pair{Key: "pl-ok", Label: "OK (emerald)", Pill: ui.NewPill("passing", ui.ToneOK)},
			),
		),
		ui.NewMarkdown("md", markdownSample),
	}
}
