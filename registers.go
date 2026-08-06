package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/slzatz/vimango/vim"
)

// Viminfo-style register persistence: registers are written to
// registers.json (cwd-relative, like config.json) when the app exits
// cleanly and restored into vim at startup, so a yank survives quitting
// one vimango instance and pasting in the next — the hybrid app runs a
// fresh `vimango --editor` per editing session, and without this every
// `:q` discarded the registers.
//
// The unnamed register is not storage — it is vim's y_previous pointer at
// whichever real register an operator wrote last, and reading it with
// y_previous unset falls back to register 0 (get_yank_register, libvim
// ops.c). setreg() deliberately preserves y_previous for every register
// except '"' itself, and setreg('"', ...) writes register 0's storage and
// points y_previous there (init_write_reg/finish_write_reg). So what gets
// persisted is which register the unnamed one aliased; on restore, an
// alias of register 0 — the common case, any plain yank — needs nothing,
// and any other alias (last op was a delete, or a yank into a named
// register) is replayed through setreg('"', ...), which makes p paste the
// right thing at the cost of register 0's storage holding a copy — the
// same meaning vim itself assigns to `:let @" = ...`.

const registersFile = "registers.json"

// A register can hold too much to be worth persisting (e.g. a whole-note
// delete of an image-heavy note); larger contents are skipped.
const maxRegisterBytes = 64 * 1024

// persistedRegister holds one register: content as getreg() returns it,
// type as getregtype() returns it ("v", "V", or "\x16<width>" for blockwise).
type persistedRegister struct {
	Content string `json:"content"`
	Type    string `json:"type"`
}

type persistedState struct {
	Registers map[string]persistedRegister `json:"registers"`
	// Unnamed names the register the unnamed register aliased at save
	// time ("" when it matched none, e.g. everything was empty).
	Unnamed string `json:"unnamed,omitempty"`
}

// persistedRegisterNames is the set worth carrying across runs —
// yank/delete history, small delete, named — matching vim's own viminfo
// set. Order matters: it is the priority order for deciding which
// register the unnamed one aliases when contents tie.
func persistedRegisterNames() []string {
	names := []string{}
	for c := '0'; c <= '9'; c++ {
		names = append(names, string(c))
	}
	names = append(names, `-`)
	for c := 'a'; c <= 'z'; c++ {
		names = append(names, string(c))
	}
	return names
}

// saveRegisters snapshots the registers to registers.json. Called from
// Cleanup, so every clean exit path (editor :q/:x in --editor mode,
// organizer :quit) passes through it; a killed process loses the yank,
// same as vim without a viminfo write.
func saveRegisters() {
	state := persistedState{Registers: make(map[string]persistedRegister)}
	for _, name := range persistedRegisterNames() {
		content := vim.EvaluateExpression("getreg('" + name + "')")
		if content == "" || len(content) > maxRegisterBytes || strings.ContainsRune(content, 0) {
			continue
		}
		state.Registers[name] = persistedRegister{
			Content: content,
			Type:    vim.EvaluateExpression("getregtype('" + name + "')"),
		}
	}

	// Which register does the unnamed one alias? There is no eval API for
	// y_previous, but its target's contents are getreg('"') — find the
	// persisted register that matches.
	unnamedContent := vim.EvaluateExpression(`getreg('"')`)
	unnamedType := vim.EvaluateExpression(`getregtype('"')`)
	if unnamedContent != "" {
		for _, name := range persistedRegisterNames() {
			if reg, ok := state.Registers[name]; ok && reg.Content == unnamedContent && reg.Type == unnamedType {
				state.Unnamed = name
				break
			}
		}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(registersFile, data, 0600)
}

// restoreRegisters loads registers.json (if any) into vim's registers.
// Any failure is silent: missing or unreadable saved registers must never
// interfere with startup.
func restoreRegisters() {
	data, err := os.ReadFile(registersFile)
	if err != nil {
		return
	}
	var state persistedState
	if json.Unmarshal(data, &state) != nil {
		return
	}

	for _, name := range persistedRegisterNames() {
		if reg, ok := state.Registers[name]; ok {
			setRegister(name, reg)
		}
	}

	// Reading the unnamed register falls back to register 0, so an alias
	// of 0 (or none) is already right; any other alias is replayed
	// through '"' so p pastes what it pasted before the quit.
	if state.Unnamed != "" && state.Unnamed != "0" {
		if reg, ok := state.Registers[state.Unnamed]; ok {
			setRegister(`"`, reg)
		}
	}
}

func setRegister(name string, reg persistedRegister) {
	vim.ExecuteCommand(`call setreg('` + name + `', "` + escapeVimString(reg.Content) + `", "` + escapeVimString(reg.Type) + `")`)
}

// escapeVimString escapes s for a double-quoted vim string inside an ex
// command: backslash, double quote, and every control character (a raw
// newline would end the ex command; \xNN covers the rest, including the
// CTRL-V byte that prefixes a blockwise register type).
func escapeVimString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n':
			b.WriteString(`\n`)
		case r < 0x20:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
