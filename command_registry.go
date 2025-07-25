package main

import (
	"fmt"
	"sort"
	"strings"
)

// CommandInfo holds metadata about a command
type CommandInfo struct {
	Name        string   // Primary command name
	Aliases     []string // Alternative names for the command
	Description string   // Short description of what the command does
	Usage       string   // Usage syntax (e.g., "open <context|folder>")
	Category    string   // Category for grouping (e.g., "Navigation", "Data Management")
	Examples    []string // Usage examples
}

// CommandRegistry manages commands with their metadata
type CommandRegistry[T any] struct {
	commands map[string]T           // Command name -> function
	info     map[string]CommandInfo // Command name -> metadata
	aliases  map[string]string      // Alias -> primary command name
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry[T any]() *CommandRegistry[T] {
	return &CommandRegistry[T]{
		commands: make(map[string]T),
		info:     make(map[string]CommandInfo),
		aliases:  make(map[string]string),
	}
}

// Register adds a command with its metadata to the registry
func (r *CommandRegistry[T]) Register(name string, fn T, info CommandInfo) {
	// Set the primary name if not already set
	if info.Name == "" {
		info.Name = name
	}

	// Store the command function and info
	r.commands[name] = fn
	r.info[name] = info

	// Register aliases
	for _, alias := range info.Aliases {
		r.commands[alias] = fn
		r.aliases[alias] = name // Map alias back to primary name
	}
}

// GetFunctionMap returns the command map in the format expected by existing code
func (r *CommandRegistry[T]) GetFunctionMap() map[string]T {
	return r.commands
}

// GetCommandInfo returns the metadata for a command (by name or alias)
func (r *CommandRegistry[T]) GetCommandInfo(name string) (CommandInfo, bool) {
	// Check if it's an alias first
	if primaryName, isAlias := r.aliases[name]; isAlias {
		name = primaryName
	}

	info, exists := r.info[name]
	return info, exists
}

// GetAllCommands returns all commands organized by category
func (r *CommandRegistry[T]) GetAllCommands() map[string][]CommandInfo {
	categories := make(map[string][]CommandInfo)

	// Group commands by category, avoiding duplicates from aliases
	for _, info := range r.info {
		categories[info.Category] = append(categories[info.Category], info)
	}

	// Sort commands within each category
	for category := range categories {
		sort.Slice(categories[category], func(i, j int) bool {
			return categories[category][i].Name < categories[category][j].Name
		})
	}

	return categories
}

// GetCommandNames returns all primary command names (no aliases)
func (r *CommandRegistry[T]) GetCommandNames() []string {
	var names []string
	for name := range r.info {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SuggestCommand returns command suggestions for a typo using simple fuzzy matching
func (r *CommandRegistry[T]) SuggestCommand(input string) []string {
	input = strings.ToLower(input)
	var suggestions []string

	// First pass: exact prefix matches
	for name := range r.info {
		if strings.HasPrefix(strings.ToLower(name), input) {
			suggestions = append(suggestions, name)
		}
	}

	// Second pass: contains matches if no prefix matches found
	if len(suggestions) == 0 {
		for name := range r.info {
			if strings.Contains(strings.ToLower(name), input) {
				suggestions = append(suggestions, name)
			}
		}
	}

	// Third pass: similar commands (edit distance or similar)
	if len(suggestions) == 0 {
		for name := range r.info {
			if levenshteinDistance(strings.ToLower(input), strings.ToLower(name)) <= 2 {
				suggestions = append(suggestions, name)
			}
		}
	}

	sort.Strings(suggestions)
	return suggestions
}

// FormatCommandHelp returns formatted help text for a specific command
func (r *CommandRegistry[T]) FormatCommandHelp(name string) string {
	info, exists := r.GetCommandInfo(name)
	if !exists {
		return fmt.Sprintf("Command '%s' not found", name)
	}

	var help strings.Builder
	help.WriteString(fmt.Sprintf("Command: %s\n", info.Name))

	if len(info.Aliases) > 0 {
		help.WriteString(fmt.Sprintf("Aliases: %s\n", strings.Join(info.Aliases, ", ")))
	}

	if info.Usage != "" {
		help.WriteString(fmt.Sprintf("Usage: %s\n", info.Usage))
	}

	if info.Description != "" {
		help.WriteString(fmt.Sprintf("Description: %s\n", info.Description))
	}

	if len(info.Examples) > 0 {
		help.WriteString("Examples:\n")
		for _, example := range info.Examples {
			help.WriteString(fmt.Sprintf("  %s\n", example))
		}
	}

	return help.String()
}

// FormatCategoryHelp returns formatted help for all commands in a category
func (r *CommandRegistry[T]) FormatCategoryHelp(category string) string {
	allCommands := r.GetAllCommands()
	commands, exists := allCommands[category]
	if !exists {
		return fmt.Sprintf("Category '%s' not found", category)
	}

	var help strings.Builder
	help.WriteString(fmt.Sprintf("%s Commands:\n\n", category))

	for _, cmd := range commands {
		help.WriteString(fmt.Sprintf("  %-15s %s\n", cmd.Name, cmd.Description))
		if len(cmd.Aliases) > 0 {
			help.WriteString(fmt.Sprintf("  %-15s (aliases: %s)\n", "", strings.Join(cmd.Aliases, ", ")))
		}
	}

	return help.String()
}

// FormatAllHelp returns formatted help for all commands, organized by category
func (r *CommandRegistry[T]) FormatAllHelp() string {
	var help strings.Builder
	help.WriteString("# Available Commands:\n\n")

	categories := r.GetAllCommands()

	// Sort categories for consistent output
	var categoryNames []string
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	for _, category := range categoryNames {
		commands := categories[category]
		help.WriteString(fmt.Sprintf("## %s:\n", category))

		for _, cmd := range commands {
			aliases := ""
			if len(cmd.Aliases) > 0 {
				aliases = fmt.Sprintf(" (%s)", strings.Join(cmd.Aliases, ", "))
			}
			help.WriteString(fmt.Sprintf("`  %-15s`%s - %s\n", cmd.Name, aliases, cmd.Description))
		}
		help.WriteString("\n")
	}

	help.WriteString("Use ':help <command>' for detailed help on a specific command.\n")
	help.WriteString("Use ':help <category>' for commands in a specific category.\n")

	return help.String()
}

// Simple Levenshtein distance calculation for command suggestions
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}

	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

// keyToDisplayName converts key sequences to human-readable format for help display
func keyToDisplayName(key string) string {
	keyMap := map[string]string{
		// Control characters using escape sequences
		"\x08": "Ctrl-H",
		"\x0c": "Ctrl-L",
		"\x0a": "Ctrl-J",
		"\x0b": "Ctrl-K",
		"\x02": "Ctrl-B",
		"\x05": "Ctrl-E",

		// Ctrl-W combinations
		"\x17L": "<C-w>L",
		"\x17J": "<C-w>J",
		"\x17=": "<C-w>=",
		"\x17_": "<C-w>_",
		"\x17-": "<C-w>-",
		"\x17+": "<C-w>+",
		"\x17>": "<C-w>>",
		"\x17<": "<C-w><",

		// Control characters using byte values
		string(0x4):  "Ctrl-D",
		string(0x1):  "Ctrl-A",
		string(0x18): "Ctrl-X",

		// Control characters using ctrlKey function
		string(ctrlKey('i')): "Ctrl-I",
		string(ctrlKey('l')): "Ctrl-L",
		string(ctrlKey('j')): "Ctrl-J",
		string(ctrlKey('k')): "Ctrl-K",
		string(ctrlKey('w')): "Ctrl-W",
		string(ctrlKey('z')): "Ctrl-Z",

		// Single characters
		":": ":",
		"m": "m",
	}

	if display, exists := keyMap[key]; exists {
		return display
	}

	// Handle leader combinations (leader is space character)
	if strings.HasPrefix(key, leader) {
		return "<leader>" + key[len(leader):]
	}

	// Return the original key if no mapping found
	return key
}
