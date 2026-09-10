// Package docs embeds the repository markdown so `gonetsim docs <topic>`
// works fully offline (air-gapped labs, Windows analyst boxes, anywhere).
// These same files are the source for the live website, so one edit lands in
// both the binary and the site.
package docs

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.md
var fsys embed.FS

// docOrder is the reading order for `gonetsim docs`. Topics not listed here are
// appended in sorted order, so a new markdown file still surfaces on its own.
var docOrder = []string{"quickstart", "flags", "capture", "lua-api", "handlers", "tls-airgap"}

// Names lists the available doc topics in reading order.
func Names() []string {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".md")
		if name != "" {
			present[name] = true
		}
	}
	names := make([]string, 0, len(present))
	for _, n := range docOrder {
		if present[n] {
			names = append(names, n)
			delete(present, n)
		}
	}
	extra := make([]string, 0, len(present))
	for n := range present {
		extra = append(extra, n)
	}
	sort.Strings(extra)
	return append(names, extra...)
}

// Read returns the markdown content for a topic.
func Read(topic string) (string, error) {
	data, err := fs.ReadFile(fsys, topic+".md")
	if err != nil {
		return "", fmt.Errorf("unknown docs topic %q (see: gonetsim docs)", topic)
	}
	return string(data), nil
}
