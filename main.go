// Command showcase is a Code Foundry plugin that uses every plugin component: a
// "gallery" side-panel surface with one tab per family of view nodes (text, data, log,
// forms, status, actions, its own thread and terminal, host calls), a "gated" surface
// that exists only while the plugin's showcase.on fact holds, five commands, and the
// tick wake source. It is
// a reference for plugin authors and a smoke test for changes to the host's renderer.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alexwaumann/code-foundry/pluginsdk"
)

func main() {
	if err := pluginsdk.Serve(plugin()); err != nil {
		fmt.Fprintln(os.Stderr, "showcase:", err)
		os.Exit(1)
	}
}

// plugin serves everything plugin.json declares. Each call makes fresh state, so tests
// can start as many as they like.
func plugin() pluginsdk.Plugin { return newShowcase().plugin() }

// plugin is s as a pluginsdk.Plugin (tests tune s first).
func (s *showcase) plugin() pluginsdk.Plugin {
	return pluginsdk.Plugin{
		Init:    s.init,
		OnEvent: s.onEvent,
		Shutdown: func(context.Context, *pluginsdk.Host) {
			s.shutdown()
		},
		OnWakeSubscribed:   s.wakeSubscribed,
		OnWakeUnsubscribed: s.wakeUnsubscribed,
		Commands: map[string]pluginsdk.CommandFunc{
			"ping":      s.ping,
			"boom":      s.boom,
			"flag":      s.flag,
			"gate":      s.gate,
			"fire-tick": s.fireTickCmd,
		},
		Surfaces: map[string]pluginsdk.SurfaceFactory{
			"gallery": s.newGallery,
			"gated":   s.newGated,
		},
	}
}
