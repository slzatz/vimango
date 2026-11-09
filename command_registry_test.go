package main

import (
	"strings"
	"testing"
	"unicode"
)

// :help <key> for an organizer normal command used to look the command up by
// its display name in a registry keyed by key bytes, so even :help Ctrl-D was
// "not found".
func TestHelpFindsNormalCommandsByName(t *testing.T) {
	o := &Organizer{}
	(&App{}).setOrganizerNormalCmds(o)
	r := o.normalCommandRegistry

	for name, want := range map[string]string{
		"Ctrl-D":     "Toggle delete status",
		"ctrl-d":     "Toggle delete status",
		"Space/PgDn": "down half a page",
		"Space":      "down half a page",
		"pgdn":       "down half a page",
		"b":          "up half a page",
		"PgUp":       "up half a page",
		"Ctrl-J":     "down one line",
		"Home":       "to top",
		"m":          "Toggle mark",
	} {
		key, ok := r.KeyFor(name)
		if !ok {
			t.Errorf("KeyFor(%q) found nothing", name)
			continue
		}
		if got := r.FormatCommandHelp(key); !strings.Contains(got, want) {
			t.Errorf(":help %s = %q, want it to mention %q", name, got, want)
		}
	}
	if _, ok := r.KeyFor("Ctrl-Z"); ok {
		t.Error("KeyFor found a command for Ctrl-Z")
	}
}

// The organizer's ex commands are checked first; none may shadow a normal
// command's name, or :help would show the ex command instead.
func TestNormalCommandNamesAreNotExCommands(t *testing.T) {
	o := &Organizer{}
	a := &App{}
	a.setOrganizerNormalCmds(o)
	a.setOrganizerExCmds(o)
	for _, cmds := range o.normalCommandRegistry.GetAllCommands() {
		for _, cmd := range cmds {
			for _, part := range strings.Split(cmd.Name, "/") {
				if _, ok := o.commandRegistry.GetCommandInfo(part); ok {
					t.Errorf("normal command %s: %q is also an ex command", cmd.Name, part)
				}
			}
		}
	}
}

// Normal commands' keys, aliases included, are bytes (or synthetic key codes
// such as PgDn's), not text: help shows only display names.
func TestNormalHelpShowsNoRawKeys(t *testing.T) {
	o := &Organizer{}
	(&App{}).setOrganizerNormalCmds(o)
	r := o.normalCommandRegistry

	texts := []string{r.FormatAllHelp(), r.FormatCategoryHelp("Navigation"), o.formatNormalModeHelp()}
	key, _ := r.KeyFor("Space")
	texts = append(texts, r.FormatCommandHelp(key))
	texts = append(texts, strings.Join(r.SuggestCommand("ctrl"), ","))
	for _, text := range texts {
		for _, c := range text {
			if c > unicode.MaxASCII || (c < ' ' && c != '\n') {
				t.Errorf("help text has raw key %q:\n%s", c, text)
				break
			}
		}
	}
	if got := r.SuggestCommand("ctrl-z"); !strings.Contains(strings.Join(got, ","), "Ctrl-") {
		t.Errorf("SuggestCommand(ctrl-z) = %q, want display names", got)
	}
}
