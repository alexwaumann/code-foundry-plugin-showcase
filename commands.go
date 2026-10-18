package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alexwaumann/code-foundry/pluginsdk"
)

// ping answers pong with a running count: no args, no confirm.
func (s *showcase) ping(_ context.Context, _ *pluginsdk.Host, req pluginsdk.CommandRequest) (pluginsdk.CommandResult, error) {
	s.mu.Lock()
	s.pings++
	n := s.pings
	s.mu.Unlock()
	go s.renderAll()
	js, _ := json.Marshal(map[string]any{"pong": n, "thread": req.Context.ThreadID})
	return pluginsdk.CommandResult{Message: fmt.Sprintf("pong %d", n), JSON: js}, nil
}

// boom pretends to destroy something. The manifest's confirm text asks first; by the
// time the plugin runs, the user said yes.
func (s *showcase) boom(_ context.Context, _ *pluginsdk.Host, req pluginsdk.CommandRequest) (pluginsdk.CommandResult, error) {
	what := strings.TrimSpace(req.Arg("what"))
	if what == "" {
		return pluginsdk.CommandResult{}, pluginsdk.NewError(pluginsdk.CodeInvalidParams, "say what to blow up")
	}
	s.mu.Lock()
	s.booms++
	n := s.booms
	s.mu.Unlock()
	go s.renderAll()
	js, _ := json.Marshal(map[string]any{"what": what, "booms": n})
	return pluginsdk.CommandResult{Message: fmt.Sprintf("Boom: %s is gone (not really, boom %d)", what, n), JSON: js}, nil
}

// flag raises or clears the plugin's flag on a thread.
func (s *showcase) flag(ctx context.Context, h *pluginsdk.Host, req pluginsdk.CommandRequest) (pluginsdk.CommandResult, error) {
	thread := req.Arg("thread")
	if thread == "" {
		thread = req.Context.ThreadID
	}
	if thread == "" {
		return pluginsdk.CommandResult{}, pluginsdk.NewError(pluginsdk.CodeInvalidParams, "no thread: run it beside a thread or with --thread <id>")
	}
	level := req.Arg("level")
	reason := strings.TrimSpace(req.Arg("reason"))
	if level != pluginsdk.FlagNone && (reason == "" || len(reason) > 80 || strings.ContainsAny(reason, "\r\n")) {
		return pluginsdk.CommandResult{}, pluginsdk.NewError(pluginsdk.CodeInvalidParams, "the reason is one line of 1 to 80 characters")
	}
	until := req.Arg("until-seen") == "true"
	msg, err := s.setFlag(ctx, h, thread, level, reason, until)
	if err != nil {
		return pluginsdk.CommandResult{}, err
	}
	return pluginsdk.CommandResult{Message: msg}, nil
}

// setFlag is shared by the command and the Host tab's "until seen" button.
func (s *showcase) setFlag(ctx context.Context, h *pluginsdk.Host, thread, level, reason string, until bool) (string, error) {
	if level == pluginsdk.FlagNone {
		reason = ""
	}
	var changed bool
	err := h.SetFlag(ctx, pluginsdk.Flag{ThreadID: thread, Level: level, Reason: reason, UntilSeen: until})
	if err == nil {
		changed = true
	}
	res := flagResult{Level: level, Reason: reason, UntilSeen: until, Changed: changed, At: s.now()}
	if err != nil {
		res.Err = err.Error()
	}
	s.mu.Lock()
	s.flags[thread] = res
	s.mu.Unlock()
	go s.renderAll()
	if err != nil {
		return "", fmt.Errorf("flagging %s: %w", short(thread), err)
	}
	if level == pluginsdk.FlagNone {
		return "Cleared the flag on " + short(thread), nil
	}
	return fmt.Sprintf("Flagged %s: %s (%s)", short(thread), reason, level), nil
}

// gate sets or removes the showcase.on fact, for one thread or everywhere.
func (s *showcase) gate(_ context.Context, h *pluginsdk.Host, req pluginsdk.CommandRequest) (pluginsdk.CommandResult, error) {
	on := req.Arg("state") == "on"
	global := req.Arg("scope") == "global"
	thread := req.Arg("thread")
	if thread == "" {
		thread = req.Context.ThreadID
	}
	if !global && thread == "" {
		return pluginsdk.CommandResult{}, pluginsdk.NewError(pluginsdk.CodeInvalidParams, "no thread: run it beside a thread, with --thread <id>, or with --scope global")
	}
	var value any // nil removes the fact
	if on {
		value = true
	}
	p := pluginsdk.ContextParams{Key: "on", Value: value}
	if !global {
		p.ThreadID = thread
	}
	if err := h.SetContext(p); err != nil {
		return pluginsdk.CommandResult{}, fmt.Errorf("setting showcase.on: %w", err)
	}
	s.mu.Lock()
	if global {
		s.gates.Global = on
	} else if on {
		s.gates.Threads[thread] = true
	} else {
		delete(s.gates.Threads, thread)
	}
	s.gates.SetAt = s.now()
	s.saveLocked()
	s.mu.Unlock()
	go s.renderAll()
	where := "everywhere"
	if !global {
		where = "for " + short(thread)
	}
	if on {
		return pluginsdk.CommandResult{Message: "Gate on " + where + ": the Gated tab is offered there"}, nil
	}
	return pluginsdk.CommandResult{Message: "Gate off " + where}, nil
}

// short is the last 6 characters of an id, as the sidebar shows them.
func short(id string) string {
	if len(id) > 6 {
		return id[len(id)-6:]
	}
	return id
}
