# code-foundry-plugin-showcase

A [Code Foundry](https://github.com/alexwaumann/code-foundry) plugin that uses **every
plugin component**: each view node, field kind, span, button style and host method, in
one side-panel gallery. Two audiences:

- **Plugin authors**: see what each primitive looks like in the real panel (dark and
  light, 400 px and 280 px) and read the Go that makes it (one file per tab).
- **Host changes**: a smoke test. After touching the plugin renderer, the protocol or a
  host capability, open the gallery and walk the tabs; each tab's header sub-line says
  what to look for.

```
code-foundry plugin install showcase --enable
```

or from a clone: `make build && code-foundry plugin dev "$PWD" --yes`.

## What it shows

A **Showcase** tab in the side panel of every thread and page (scope `global`: one
instance, shared), with eight tabs:

| Tab | Components | Look for |
|---|---|---|
| **Text** | `text` in every size (base, `sm`, `lg` copyable), copyable mono; spans in every tone, strong, mono, links; times (relative past/future, elapsed, countdown, expired) and durations (450 ms, 2.5 s, 95 s, 3 h); pills in every tone; `markdown` with lists, tasks, code, quote, table, raw HTML | times tick without new views; long tokens and code blocks do not widen the panel |
| **Data** | `pairs` (plain, mono copyable, pill, value + pill, times, link, toned, 40-character label, long value); `list` rows with every status glyph, `titleMeta`, pill, `line2`, `trailing`, up to 3 buttons, row action and `selected`; icon glyphs in every tone (and an unknown icon); a dense list; an empty list; `chips` in every status with meta, selected and clickable | click a row or chip to select it; dense hides line 2 and labelled buttons |
| **Log** | a text log (wrap, title, 10 rows), a lines log in every tone (no wrap, `truncated`), a `noFollow` log; Stream (20 lines by `AppendLog`, button busy, tab badge *live*), Append toned lines (`AppendLogLines`), Start over | the tail follows; switching tabs and back keeps the output |
| **Forms** | every field: text (required, pattern, placeholder, hint, max length, mono), text with a `change` event, secret with `store`, secret without, dropdown, list, multiple list, multiple dropdown, `optionsSource` projects and threads, toggle, fieldset with hint and error, disabled; submit, secondary Reset and Fill example, form error, busy, `resetToken`; a second form whose submit runs a command | GUI checks as you type; the plugin re-checks (handle `admin` is refused); drafts survive tab switches; the read-back shows what arrived (secret lengths only) |
| **Status** | `progress` determinate, indeterminate, with meta, 0, 1, bare, failed; `notice` neutral, attention, error, body only; `empty` with glyph and buttons, status glyph, title only; `section` titled with meta, pill and buttons, collapsible (both initial states), stale, cards in each tone, nested two deep | stale dims; your collapse survives a re-render |
| **Actions** | every button style labelled, with icons, icon only and disabled; busy (a 2 s load and an always-busy one); command buttons (the plugin's own commands, one with confirm, and the host's `view.open.url`); open URL and copy (done by the host); event buttons with data, an unknown event (an error) and a long label; `align: end` | no filled buttons; amber outline only on attention; the last act shows its kind, key, data, version and the panel's thread |
| **Owned** | a `thread` node (the plugin's *Showcase helper*, editable, 420 px) and a read-only one (*Showcase narrator*, 240 px), both with cwd = the plugin's data dir, an `--add-dir` holding a `showcase-hello` skill, and env; a `terminal` node running a one-second ticker with Restart and Stop beside Open in bottom panel; the plugin's `thread/list` and `terminal/list` | neither shows in the sidebar; Ask / Narrate send through `SendThread` |
| **Host** | `SetFlag` attention, error, until seen, clear (on the panel's thread); `Toast` and `ToastIn` at each level; `SetContext` (`showcase.on` per thread or everywhere) which offers the **Gated** tab; `SetEnvironment` on, off and a bad push with issues; secrets stored by a form, read (length only), set, deleted, listed with "set at …"; `Subscribe` / `Unsubscribe` to `threads`, `project`, `gh` with counts; `Invoke` of `pr.checks`, `thread.send` and a refused command | flags in the sidebar, the Gated tab appearing, `code-foundry plugin env showcase` |

**Gated** (scope `thread`, `when: showcase.on`): a tiny tab that exists only where the
plugin set the fact, with a button to turn it off again.

**Commands** (palette under *Showcase*, CLI `code-foundry plugin showcase …`):

| Command | Does |
|---|---|
| `ping` | `pong <n>`: no args, no confirm |
| `boom [what]` | asks "Blow up {what}? Nothing is really deleted." then pretends (`--yes` on the CLI) |
| `flag [attention\|error\|none] [--reason r] [--until-seen] [--thread id]` | flags the thread or clears the flag |
| `gate [on\|off] [--scope thread\|global] [--thread id]` | sets or removes `showcase.on` |

## Permissions and why

| Permission | Why |
|---|---|
| `commands: thread.send, view.open.url, pr.checks` | the Host tab's Invoke buttons and the Actions tab's host command button |
| `events: threads, project, gh` | the Host tab's subscribe buttons and counters |
| `threadStatus` | the flag command and the Host tab's flag buttons |
| `sessionEnv` | the Host tab's environment buttons (`SHOWCASE_ENV`, a PATH entry and an `--add-dir`, for every new thread while on) |
| `secrets` | the Forms tab's stored token and the Host tab's demo secrets (your login Keychain, service `code-foundry-plugin`, accounts `showcase/<key>`) |
| `threads` | the Owned tab's two plugin threads (they cost what a message costs only when you send one) |
| `terminals` | the Owned tab's ticker terminal |

It never asks for `api`. Nothing it does leaves your Mac except what Claude does in its
own threads when you send them a message.

## Development

```
make build && code-foundry plugin dev "$PWD" --yes   # link and run it
make test                                           # plugintest: every tab valid, every act
code-foundry plugin surface showcase gallery        # the first view, and the elements that act
code-foundry plugin act showcase gallery tab --key tab-host --context-thread <id>
code-foundry plugin logs showcase --lines 50
```

One file per tab (`tab_*.go`), registered in `gallery.go`; `state.go` holds what tabs
share; `commands.go` the manifest's commands; `gated.go` the gated surface.

## License

Apache-2.0
