package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexwaumann/code-foundry/pluginsdk"
	"github.com/alexwaumann/code-foundry/pluginsdk/ui"
)

var formsTab = tabDef{
	key: tabForms, label: "Forms",
	look:  "GUI checks required and pattern as you type; the plugin re-checks (try handle admin); drafts survive tab switches; Reset drops them.",
	nodes: formNodes,
	acts: map[string]actFunc{
		"profile-save":    saveProfile,
		"profile-reset":   resetProfile,
		"profile-example": exampleProfile,
		"profile-draft":   draftProfile,
	},
}

const tokenKey = "token"

var handlePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// profileForm is the plugin's model of the big form: the values the view shows (after
// a save or Fill example), the plugin's field errors, and what came back.
type profileForm struct {
	vals       map[string]any
	errs       map[string]string
	formErr    string
	busy       bool
	token      int // resetToken
	submitted  map[string]any
	submitAt   time.Time
	submits    int
	pinLen     int
	tokenSet   bool
	tokenSetAt time.Time
	draft      map[string]string // field → latest change value
}

func newProfileForm() profileForm {
	return profileForm{vals: defaultProfile(), errs: map[string]string{}, draft: map[string]string{}}
}

func defaultProfile() map[string]any {
	return map[string]any{
		"handle": "", "email": "ada@example.com", "colour": "", "plan": "free",
		"tags": []string{"go"}, "langs": []string{}, "project": "", "thread": "",
		"notify": true, "onFail": true, "onDone": false,
	}
}

func exampleValues() map[string]any {
	return map[string]any{
		"handle": "ada-l", "email": "ada@lovelace.dev", "colour": "green", "plan": "team",
		"tags": []string{"go", "ts"}, "langs": []string{"en", "fr"}, "project": "", "thread": "",
		"notify": false, "onFail": true, "onDone": true,
	}
}

func (p profileForm) hasErrors() bool { return len(p.errs) > 0 || p.formErr != "" }

func str(v any) string {
	s, _ := v.(string)
	return s
}

func strs(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	}
	return nil
}

func boolean(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	}
	return false
}

func oneOrNone(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func formNodes(g *gallery) []ui.Node {
	p := g.profile
	v := p.vals

	handle := ui.TextField("f-handle", "handle", "Handle")
	handle.Required = true
	handle.Description = ui.Spans(ui.T("Lower case, digits and dashes. "), ui.T("admin").AsMono(), ui.T(" is refused by the plugin, not the GUI."))
	handle.Input.Placeholder, handle.Input.Pattern, handle.Input.PatternMessage = "ada-l", handlePattern.String(), "lower case letters, digits and -, starting with a letter"
	handle.Input.MaxLength, handle.Input.Mono, handle.Input.Value = 24, true, str(v["handle"])
	handle.Error = p.errs["handle"]

	email := ui.TextField("f-email", "email", "Email")
	email.Input.Value, email.Input.Placeholder = str(v["email"]), "you@example.com"
	email.Description = ui.Plain("Sends a change event as you type: the read-back below shows the draft.")
	email.Change = ui.EventAction("profile-draft", "")
	email.Error = p.errs["email"]

	token := ui.SecretField("f-token", "token", "API token (stored)", tokenKey)
	token.Input.HasValue = p.tokenSet
	token.Description = ui.Plain("store: the host keeps it in the Keychain; the plugin sees {stored: true}.")

	pin := ui.SecretField("f-pin", "pin", "One-time PIN (not stored)", "")
	pin.Input.MaxLength = 8
	pin.Description = ui.Plain("No store: the value reaches the plugin in the submit only; the read-back shows its length.")

	colour := ui.SelectField("f-colour", "colour", "Colour (dropdown)",
		ui.Opt("o-red", "red", "Red"), ui.Opt("o-green", "green", "Green"), ui.Opt("o-blue", "blue", "Blue"),
		ui.Option{Key: "o-grey", Value: "grey", Label: "Grey (disabled)", Disabled: true})
	colour.Select.Placeholder, colour.Select.Value = "Pick a colour", oneOrNone(str(v["colour"]))

	plan := ui.SelectField("f-plan", "plan", "Plan (list)",
		ui.Option{Key: "o-free", Value: "free", Label: "Free", Description: ui.Plain("One project, community support.")},
		ui.Option{Key: "o-team", Value: "team", Label: "Team", Description: ui.Spans(ui.T("Ten projects, "), ui.T("SSO").AsStrong(), ui.T("."))},
		ui.Option{Key: "o-ent", Value: "enterprise", Label: "Enterprise", Description: ui.Plain("Not offered here: disabled."), Disabled: true})
	plan.Select.Style, plan.Select.Value = ui.SelectList, oneOrNone(str(v["plan"]))
	plan.Required = true

	tags := ui.SelectField("f-tags", "tags", "Tags (list, multiple)",
		ui.Opt("o-go", "go", "Go"), ui.Opt("o-ts", "ts", "TypeScript"), ui.Opt("o-py", "py", "Python"), ui.Opt("o-rs", "rs", "Rust"))
	tags.Select.Style, tags.Select.Multiple, tags.Select.Value = ui.SelectList, true, strs(v["tags"])
	tags.Description = ui.Plain("Pick up to three (the plugin checks).")
	tags.Error = p.errs["tags"]

	langs := ui.SelectField("f-langs", "langs", "Languages (dropdown, multiple)",
		ui.Opt("o-en", "en", "English"), ui.Opt("o-fr", "fr", "French"), ui.Opt("o-de", "de", "German"), ui.Opt("o-es", "es", "Spanish"))
	langs.Select.Multiple, langs.Select.Value, langs.Select.Placeholder = true, strs(v["langs"]), "Any"

	project := ui.Field{Key: "f-project", Name: "project", Label: "Project (optionsSource projects)",
		Select: &ui.Select{OptionsSource: ui.OptionsProjects, Placeholder: "Pick a project", Value: oneOrNone(str(v["project"]))}}
	thread := ui.Field{Key: "f-thread", Name: "thread", Label: "Thread (optionsSource threads)",
		Select: &ui.Select{OptionsSource: ui.OptionsThreads, Placeholder: "Pick a thread", Value: oneOrNone(str(v["thread"]))}}

	notify := ui.ToggleField("f-notify", "notify", "Email me a summary", boolean(v["notify"]))
	notify.Description = ui.Plain("A toggle with a description.")

	when := ui.NewFieldset("f-when", "Notify me when",
		ui.ToggleField("f-on-fail", "onFail", "A run fails", boolean(v["onFail"])),
		ui.ToggleField("f-on-done", "onDone", "A run finishes", boolean(v["onDone"])))
	when.Fieldset.Hint = ui.Plain("A fieldset: label, hint, and an error the plugin sets when both are off.")
	when.Fieldset.Error = p.errs["when"]

	disabled := ui.TextField("f-disabled", "disabled", "Disabled field")
	disabled.Disabled, disabled.Input.Value = true, "read only"

	save := ui.EventButton("profile-save-btn", "Save", "profile-save").WithStyle(ui.ButtonAttention).WithIcon("check")
	form := ui.NewForm("form-profile", &save, handle, email, token, pin, colour, plan, tags, langs, project, thread, notify, when, disabled)
	form.Form.Secondary = []ui.Button{
		ui.EventButton("profile-reset-btn", "Reset", "profile-reset").WithIcon("rotate-ccw"),
		ui.EventButton("profile-example-btn", "Fill example", "profile-example"),
	}
	form.Form.Error, form.Form.Busy, form.Form.ResetToken = p.formErr, p.busy, strconv.Itoa(p.token)

	boomWhat := ui.TextField("f-what", "what", "What to blow up")
	boomWhat.Required = true
	boomWhat.Input.Placeholder, boomWhat.Input.Value = "the demo", g.boomWhat
	boomWhat.Description = ui.Plain("The submit runs plugin.showcase.boom with this field as its what arg: the host's confirm dialog fills {what}.")
	boomBtn := ui.CommandButton("boom-submit", "Blow it up", "plugin.showcase.boom", nil).WithStyle(ui.ButtonDestructive).WithIcon("trash-2")
	boomForm := ui.NewForm("form-boom", &boomBtn, boomWhat)

	return []ui.Node{
		ui.NewSection("sec-form-profile", "Every field kind (submit: event)", form),
		ui.NewSection("sec-readback", "Read back", readBackNodes(g)...),
		ui.NewSection("sec-form-boom", "A form that runs a command", boomForm),
	}
}

func readBackNodes(g *gallery) []ui.Node {
	p := g.profile
	draftPairs := []ui.Pair{}
	keys := make([]string, 0, len(p.draft))
	for k := range p.draft {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		draftPairs = append(draftPairs, ui.Pair{Key: "draft-" + k, Label: "Draft " + k, Value: ui.Spans(ui.T(p.draft[k]).AsMono())})
	}
	if len(draftPairs) == 0 {
		draftPairs = append(draftPairs, ui.NewPair("draft-none", "Draft", ui.Spans(ui.T("type in Email to see change events").WithTone(ui.ToneMuted))))
	}
	tokenPair := ui.Pair{Key: "draft-token-set", Label: "Token", Pill: ui.NewPill("not set", ui.ToneMuted)}
	if p.tokenSet {
		tokenPair = ui.Pair{Key: "draft-token-set", Label: "Token", Value: ui.Spans(ui.T("set "), ui.At(p.tokenSetAt, ui.TimeRelative)), Pill: ui.NewPill("stored", ui.ToneOK)}
	}
	nodes := []ui.Node{ui.NewPairs("pairs-draft", append(draftPairs, tokenPair)...)}
	if p.submitted == nil {
		return append(nodes, ui.NewText("rb-none", ui.Spans(ui.T("Nothing submitted yet.").WithTone(ui.ToneMuted))))
	}
	// Only the form's own names: plugin act --values can send anything.
	names := []string{}
	for _, k := range profileNames {
		if _, ok := p.submitted[k]; ok {
			names = append(names, k)
		}
	}
	pairs := []ui.Pair{ui.NewPair("rb-at", "Submitted", ui.Spans(ui.At(p.submitAt, ui.TimeRelative), ui.T(fmt.Sprintf(" (#%d)", p.submits))))}
	for _, k := range names {
		pairs = append(pairs, ui.Pair{Key: "rb-" + k, Label: k, Value: ui.Spans(ui.T(describe(k, p.submitted[k], p.pinLen)).AsMono())})
	}
	return append(nodes, ui.NewPairs("pairs-readback", pairs...))
}

// describe prints a submitted value for the read-back, never a secret's value.
func describe(name string, v any, pinLen int) string {
	switch name {
	case "pin":
		if pinLen == 0 {
			return "(empty)"
		}
		return fmt.Sprintf("%d characters (value withheld)", pinLen)
	case "token":
		if m, ok := v.(map[string]any); ok {
			return fmt.Sprintf("stored under %v", m["key"])
		}
		return "(left empty: the stored one stays)"
	}
	switch t := v.(type) {
	case []any, []string:
		s := strs(t)
		if len(s) == 0 {
			return "[]"
		}
		return "[" + strings.Join(s, ", ") + "]"
	case bool:
		return strconv.FormatBool(t)
	case string:
		if t == "" {
			return `""`
		}
		return t
	case nil:
		return "(absent)"
	}
	return fmt.Sprint(v)
}

// validate is the plugin's own check of a submit: the GUI's checks are skipped by
// other clients (plugin act --values), so every rule is applied again here.
func validate(vals map[string]any) (map[string]string, string) {
	errs := map[string]string{}
	handle := strings.TrimSpace(str(vals["handle"]))
	switch {
	case handle == "":
		errs["handle"] = "A handle is required."
	case !handlePattern.MatchString(handle):
		errs["handle"] = "Lower case letters, digits and -, starting with a letter."
	case len(handle) > 24:
		errs["handle"] = "At most 24 characters."
	case handle == "admin" || handle == "root":
		errs["handle"] = handle + " is reserved: pick another."
	}
	if e := str(vals["email"]); e != "" && !strings.Contains(e, "@") {
		errs["email"] = "That is not an email address."
	}
	if len(strs(vals["tags"])) > 3 {
		errs["tags"] = "At most three tags."
	}
	if !boolean(vals["onFail"]) && !boolean(vals["onDone"]) {
		errs["when"] = "Pick at least one."
	}
	if str(vals["plan"]) == "enterprise" {
		errs["plan"] = "Enterprise is not offered."
	}
	formErr := ""
	if len(errs) > 0 {
		formErr = fmt.Sprintf("Nothing saved: %d field(s) need a fix.", len(errs))
	}
	return errs, formErr
}

func saveProfile(ctx context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	errs, formErr := validate(act.Values)
	pinLen := len(str(act.Values["pin"]))
	_, stored := act.Stored("token")
	g.s.mu.Lock()
	g.profile.submits++
	g.profile.submitted, g.profile.submitAt, g.profile.pinLen = act.Values, g.s.now(), pinLen
	g.profile.errs, g.profile.formErr = errs, formErr
	if formErr == "" {
		g.profile.busy = true
	}
	saveFor := g.s.saveFor
	g.s.mu.Unlock()
	if stored {
		g.refreshTokenSet(ctx)
	}
	if err := g.render(); err != nil || formErr != "" {
		return err
	}
	// A valid save: pretend it takes a moment (the form is busy), then show the saved
	// values and drop the drafts with a new resetToken.
	go func() {
		time.Sleep(saveFor)
		g.s.mu.Lock()
		vals := map[string]any{}
		for k, v := range act.Values {
			if k != "token" && k != "pin" && k != "disabled" {
				vals[k] = v
			}
		}
		g.profile.vals = vals
		g.profile.busy = false
		g.profile.token++
		g.profile.draft = map[string]string{}
		h := g.s.h
		g.s.mu.Unlock()
		_ = h.ToastIn(g.open.Instance, pluginsdk.ToastInfo, "Saved: the form shows the saved values")
		_ = g.render()
	}()
	return nil
}

func resetProfile(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	g.profile.vals = defaultProfile()
	g.profile.errs, g.profile.formErr = map[string]string{}, ""
	g.profile.draft = map[string]string{}
	g.profile.token++
	g.s.mu.Unlock()
	return g.render()
}

func exampleProfile(_ context.Context, g *gallery, _ pluginsdk.SurfaceAct) error {
	g.s.mu.Lock()
	g.profile.vals = exampleValues()
	g.profile.errs, g.profile.formErr = map[string]string{}, ""
	g.profile.token++
	g.s.mu.Unlock()
	return g.render()
}

func draftProfile(_ context.Context, g *gallery, act pluginsdk.SurfaceAct) error {
	if act.Field != "email" {
		return fmt.Errorf("no change event on %q", act.Field)
	}
	g.s.mu.Lock()
	g.profile.draft[act.Field] = fmt.Sprint(act.Value)
	g.s.mu.Unlock()
	return g.render()
}

// refreshTokenSet reads whether the stored token exists and when it was set (never its
// value) for the secret input's hasValue.
func (g *gallery) refreshTokenSet(ctx context.Context) {
	h := g.s.host()
	if !h.Granted().Secrets {
		return
	}
	list, err := h.Secrets().List(ctx)
	if err != nil {
		return
	}
	g.s.mu.Lock()
	defer g.s.mu.Unlock()
	g.profile.tokenSet = false
	for _, s := range list {
		if s.Key == tokenKey {
			g.profile.tokenSet, g.profile.tokenSetAt = true, s.UpdatedAt
		}
	}
}

// profileNames are the big form's field names in display order.
var profileNames = []string{"handle", "email", "token", "pin", "colour", "plan", "tags", "langs", "project", "thread", "notify", "onFail", "onDone", "disabled"}
