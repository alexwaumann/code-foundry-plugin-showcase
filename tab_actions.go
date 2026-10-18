package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

var actionsTab = tabDef{
	key: tabActions, label: "Actions",
	look:  "No filled buttons; amber outline only on attention; destructive asks via its command; the last click below names key, data and the panel's thread.",
	nodes: actionNodes,
	acts: map[string]actFunc{
		"click": func(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error { return g.render() },
		"load":  loadButton,
	},
}

var buttonStyles = []ui.ButtonStyle{ui.ButtonMuted, ui.ButtonOutline, ui.ButtonAttention, ui.ButtonDestructive, ui.ButtonLink}

var styleIcons = map[ui.ButtonStyle]string{ui.ButtonMuted: "circle", ui.ButtonOutline: "square", ui.ButtonAttention: "bell", ui.ButtonDestructive: "trash-2", ui.ButtonLink: "link"}

func loadButton(_ context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	if g.busyKey != "" {
		g.s.mu.Unlock()
		return nil
	}
	g.busyKey = act.Key
	d := g.s.busyFor
	g.s.mu.Unlock()
	go func() {
		time.Sleep(d)
		g.s.mu.Lock()
		g.busyKey = ""
		g.s.mu.Unlock()
		_ = g.render()
	}()
	return g.render()
}

func actionNodes(g *gallery) []ui.Node {
	labelled, iconOnly, disabled := []ui.Button{}, []ui.Button{}, []ui.Button{}
	for _, st := range buttonStyles {
		name := string(st)
		labelled = append(labelled, ui.EventButton("btn-"+name, name, "click").WithStyle(st).WithData("labelled "+name))
		iconOnly = append(iconOnly, ui.EventButton("btn-icon-"+name, name+" (icon only)", "click").WithStyle(st).WithIcon(styleIcons[st]).AsIconOnly().WithData("icon "+name))
		b := ui.EventButton("btn-off-"+name, name, "click").WithStyle(st).WithIcon(styleIcons[st])
		b.Disabled = true
		disabled = append(disabled, b)
	}
	withIcon := []ui.Button{}
	for _, st := range buttonStyles {
		withIcon = append(withIcon, ui.EventButton("btn-wi-"+string(st), string(st), "click").WithStyle(st).WithIcon(styleIcons[st]).WithData("with icon "+string(st)))
	}
	load := ui.EventButton("btn-load", "Load for 2 s", "load").WithIcon("refresh-cw").WithStyle(ui.ButtonOutline)
	if g.busyKey == "btn-load" {
		load.Busy, load.Label = true, "Loading…"
	}
	alwaysBusy := ui.EventButton("btn-busy", "Always busy", "click")
	alwaysBusy.Busy = true
	loadIcon := ui.EventButton("btn-load-icon", "Load (icon only)", "load").WithIcon("refresh-cw").AsIconOnly()
	if g.busyKey == "btn-load-icon" {
		loadIcon.Busy = true
	}

	last := []ui.Pair{ui.NewPair("last-none", "Last click", ui.Spans(ui.T("nothing yet").WithTone(ui.ToneMuted)))}
	if a := g.lastAct; a != nil {
		last = []ui.Pair{
			ui.NewPair("last-kind", "Kind", ui.Spans(ui.T(a.Kind).AsMono())),
			ui.NewPair("last-action", "Event", ui.Spans(ui.T(a.Action).AsMono())),
			ui.NewPair("last-key", "Key", ui.Spans(ui.T(a.Key).AsMono())),
			ui.NewPair("last-data", "Data", orNone(a.Data, true)),
			ui.NewPair("last-version", "View version", ui.Plain(fmt.Sprint(a.Version))),
		}
		last = append(last, g.actContextLocked()...)
	}
	counts := ui.NewPairs("pairs-counts",
		ui.NewPair("cnt-clicks", "Acts", ui.Plain(fmt.Sprint(g.clicks))),
		ui.NewPair("cnt-pings", "Pings", ui.Plain(fmt.Sprint(g.s.pings))),
		ui.NewPair("cnt-booms", "Booms", ui.Plain(fmt.Sprint(g.s.booms))))

	return []ui.Node{
		ui.NewSection("sec-styles", "Every style, labelled", ui.NewActions("bar-styles", labelled...)),
		ui.NewSection("sec-styles-icon", "With icons", ui.NewActions("bar-with-icon", withIcon...)),
		ui.NewSection("sec-icon-only", "Icon only (tooltip = label)", ui.NewActions("bar-icon-only", iconOnly...)),
		ui.NewSection("sec-disabled", "Disabled", ui.NewActions("bar-disabled", disabled...)),
		ui.NewSection("sec-loading", "Loading state (busy)", ui.NewActions("bar-loading", load, loadIcon, alwaysBusy)),
		ui.NewSection("sec-commands", "Commands (the host runs them with this panel's context)",
			ui.NewActions("bar-commands",
				ui.CommandButton("cmd-ping", "Ping", "plugin.showcase.ping", nil).WithIcon("zap"),
				ui.CommandButton("cmd-boom", "Blow up the gallery", "plugin.showcase.boom", map[string]string{"what": "the gallery"}).WithStyle(ui.ButtonDestructive).WithIcon("trash-2"),
				ui.CommandButton("cmd-flag", "Flag this thread", "plugin.showcase.flag", map[string]string{"level": "attention", "reason": "flagged from the Actions tab"}).WithIcon("flag"),
				ui.CommandButton("cmd-unflag", "Clear flag", "plugin.showcase.flag", map[string]string{"level": "none"}),
				ui.CommandButton("cmd-url", "Open URL (host command)", "view.open.url", map[string]string{"url": "https://github.com/alexwaumann/code-foundry-plugin-showcase"}).WithIcon("globe"),
			)),
		ui.NewSection("sec-host-actions", "Done by the host (never reach the plugin)",
			ui.Node{Key: "bar-host", Actions: &ui.Actions{Align: ui.AlignEnd, Buttons: []ui.Button{
				ui.URLButton("btn-url", "Open URL", "https://github.com/alexwaumann/code-foundry").WithIcon("external-link"),
				ui.CopyButton("btn-copy", "Copy text", "copied from the showcase plugin").WithIcon("copy"),
				ui.CopyButton("btn-copy-icon", "Copy (icon only)", "icon copy").WithIcon("copy").AsIconOnly(),
				ui.URLButton("btn-url-link", "A link-style URL", "https://github.com/alexwaumann/code-foundry").WithStyle(ui.ButtonLink),
			}}},
			ui.NewText("host-align", ui.Spans(ui.T("This bar is align: end.").WithTone(ui.ToneMuted)))),
		ui.NewSection("sec-events", "Event buttons with data",
			ui.NewActions("bar-events",
				ui.EventButton("ev-a", "Data a", "click").WithData("a"),
				ui.EventButton("ev-json", "Data JSON", "click").WithData(`{"run":7,"job":"test"}`),
				ui.EventButton("ev-none", "No data", "click"),
				ui.EventButton("ev-6", "Six", "click").WithData("6"),
				ui.EventButton("ev-unknown", "Unknown event (errors)", "nope-not-handled"),
				ui.EventButton("ev-long", "A long label that has to wrap or truncate at 280 px", "click").WithData("long"),
			)),
		ui.NewSection("sec-last", "Last act", ui.NewPairs("pairs-last", last...), counts),
	}
}
