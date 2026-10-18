package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

var logTab = tabDef{
	key: tabLog, label: "Log",
	short: "Streaming, tones, no-wrap, noFollow",
	look:  "Stream follows the tail (tab badge live, button busy); lines log has every tone and does not wrap; noFollow stays put.",
	nodes: logNodes,
	acts: map[string]actFunc{
		"log-stream":     streamLog,
		"log-lines":      appendLines,
		"log-start-over": startOver,
	},
}

const logSeed = "$ showcase stream\nready: press Stream to append 20 lines, one every 250 ms\n"

// maxKeep bounds the plugin's own copy of the streamed text (it re-seeds the node
// when the tab comes back; the host keeps its own 2,000 lines).
const maxKeep = 32 << 10

func (g *gallery) resetLog() {
	g.logText = logSeed
	g.logLines = []ui.LogLine{
		{Text: "neutral: checking 42 files"},
		{Text: "muted: cache hit for module graph", Tone: ui.ToneMuted},
		{Text: "attention: warning: unused variable x in a line long enough that it must scroll sideways rather than wrap at 280 px or even 420 px", Tone: ui.ToneAttention},
		{Text: "error: missing return at the end of function", Tone: ui.ToneError},
		{Text: "ok: 41 passed (ok tone, neutral outside pills?)", Tone: ui.ToneOK},
	}
}

func logNodes(g *gallery) []ui.Node {
	// While the log nodes are on screen their buffers live in the host: leave text
	// and lines empty so appends are kept. When the tab comes back, seed them again.
	text, lines := "", []ui.LogLine(nil)
	if !g.logShown {
		text, lines = g.logText, g.logLines
	}
	streamBtn := ui.EventButton("log-stream-btn", "Stream 20 lines", "log-stream").WithIcon("play").WithStyle(ui.ButtonOutline)
	if g.streaming {
		streamBtn.Label, streamBtn.Busy, streamBtn.Disabled = "Streaming…", true, true
	}
	return []ui.Node{
		ui.NewActions("log-bar",
			streamBtn,
			ui.EventButton("log-lines-btn", "Append toned lines", "log-lines").WithIcon("plus"),
			ui.EventButton("log-start-over-btn", "Start over", "log-start-over").WithIcon("rotate-ccw"),
		),
		{Key: "log-text", Log: &ui.Log{Title: "Text log (wraps, 10 rows)", Rows: 10, Wrap: true, Text: text}},
		{Key: "log-lines", Log: &ui.Log{Title: "Lines log (every tone, no wrap, truncated head)", Rows: 8, Truncated: true, Lines: lines}},
		{Key: "log-nofollow", Log: &ui.Log{Title: "noFollow, 4 rows", Rows: 4, NoFollow: true, Text: numbered(30)}},
	}
}

func numbered(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %02d of %d: the view should stay at the top\n", i, n)
	}
	return b.String()
}

func (g *gallery) keepText(s string) {
	g.logText += s
	if len(g.logText) > maxKeep {
		g.logText = g.logText[len(g.logText)-maxKeep:]
	}
}

// streamLog appends 20 lines, one every streamEvery, then a verdict: the last output
// goes out before the view that ends the run.
func streamLog(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	if g.streaming {
		g.s.mu.Unlock()
		return nil
	}
	g.streaming = true
	g.streamRun++
	run := g.streamRun
	every := g.s.streamEvery
	h := g.s.h
	g.s.mu.Unlock()
	if err := g.render(); err != nil {
		return err
	}
	go func() {
		start := time.Now()
		for i := 1; i <= 20; i++ {
			time.Sleep(every)
			line := fmt.Sprintf("[%s] streamed line %02d/20 of run %d\n", time.Now().Format("15:04:05.000"), i, run)
			g.s.mu.Lock()
			if g.streamRun != run {
				g.s.mu.Unlock()
				return
			}
			g.keepText(line)
			shown := g.logShown
			g.s.mu.Unlock()
			if shown {
				_ = h.AppendLog(g.open.Instance, "log-text", line)
			}
		}
		done := fmt.Sprintf("done: 20 lines in %s\n", time.Since(start).Round(time.Millisecond))
		g.s.mu.Lock()
		g.keepText(done)
		shown := g.logShown
		g.s.mu.Unlock()
		if shown {
			_ = h.AppendLog(g.open.Instance, "log-text", done)
		}
		g.s.mu.Lock()
		g.streaming = false
		g.s.mu.Unlock()
		_ = g.render()
	}()
	return nil
}

var toneCycle = []ui.Tone{ui.ToneNeutral, ui.ToneMuted, ui.ToneAttention, ui.ToneError, ui.ToneOK}

// appendLines appends one line per tone to the lines log.
func appendLines(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	n := len(g.logLines)
	add := make([]pluginsdk.AppendLine, 0, len(toneCycle))
	for i, t := range toneCycle {
		tone := string(t)
		if tone == "" {
			tone = "neutral"
		}
		text := fmt.Sprintf("appended %d (%s)", n+i+1, tone)
		add = append(add, pluginsdk.AppendLine{Text: text, Tone: string(t)})
		g.logLines = append(g.logLines, ui.LogLine{Text: text, Tone: t})
	}
	shown := g.logShown
	h := g.s.h
	g.s.mu.Unlock()
	if !shown {
		return g.render()
	}
	return h.AppendLogLines(g.open.Instance, "log-lines", add...)
}

// startOver replaces both buffers by rendering the nodes with their seed text.
func startOver(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	g.streamRun++
	g.streaming = false
	g.resetLog()
	g.logShown = false // the next view sets text and lines: the host replaces the buffers
	g.s.mu.Unlock()
	return g.render()
}
