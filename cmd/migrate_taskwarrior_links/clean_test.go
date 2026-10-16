package main

import (
	"reflect"
	"strings"
	"testing"
)

const tw = "8a719afd-84d9-4f60-a962-56b511569559"

func TestCleanTaskNote(t *testing.T) {
	open := "[Open task in Taskchamp](taskchampdev://task/" + tw + ")"
	cases := []struct {
		name, note, want string
	}{
		{
			name: "the shape every client wrote",
			note: "---\ntaskwarrior: " + tw + "\nproject: \"work\"\n---\n\n" + open + "\n\n\nI'd like Jey to track Jeev down.\n\n",
			want: "I'd like Jey to track Jeev down.\n\n",
		},
		{
			name: "no project line",
			note: "---\ntaskwarrior: " + tw + "\n---\n\n" + open + "\n\nHey, before I get to it\n",
			want: "Hey, before I get to it\n",
		},
		{
			name: "other keys keep the block",
			note: "---\ntitle: x\ntaskwarrior: " + tw + "\ndate: 2026-10-11\n---\n\n" + open + "\n\nbody\n",
			want: "---\ntitle: x\ndate: 2026-10-11\n---\n\nbody\n",
		},
		{
			name: "CRLF keeps its endings",
			note: "---\r\ntitle: x\r\ntaskwarrior: " + tw + "\r\n---\r\n\r\n" + open + "\r\n\r\nbody\r\n",
			want: "---\r\ntitle: x\r\n---\r\n\r\nbody\r\n",
		},
		{
			name: "uppercase uuid, open line further down",
			note: "---\ntaskwarrior: " + strings.ToUpper(tw) + "\n---\n\nintro\n\n" + open + "\n\nmore\n",
			want: "intro\n\nmore\n",
		},
		{
			name: "nothing but the link",
			note: "---\ntaskwarrior: " + tw + "\n---\n\n" + open + "\n\n",
			want: "",
		},
		{
			name: "no trailing newline",
			note: "---\ntaskwarrior: " + tw + "\n---\n\n" + open,
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := cleanTaskNote(c.note)
			if !ok {
				t.Fatal("not recognized as a task note")
			}
			if got.uuid != tw {
				t.Errorf("uuid = %q, want %q", got.uuid, tw)
			}
			if got.note != c.want {
				t.Errorf("note =\n%q\nwant\n%q", got.note, c.want)
			}
		})
	}
}

func TestCleanTaskNoteRemovedLines(t *testing.T) {
	note := "---\ntaskwarrior: " + tw + "\nproject: \"work\"\n---\n\n[Open task in Taskchamp](taskchampdev://task/" + tw + ")\n\nbody\n"
	got, _ := cleanTaskNote(note)
	want := []string{"---", "taskwarrior: " + tw, "project: \"work\"", "---",
		"[Open task in Taskchamp](taskchampdev://task/" + tw + ")"}
	if !reflect.DeepEqual(got.removed, want) {
		t.Errorf("removed = %q, want %q", got.removed, want)
	}
}

// The notes the clients never treated as linked are left alone.
func TestCleanTaskNoteLeavesUnlinkedNotes(t *testing.T) {
	for _, note := range []string{
		"taskwarrior: " + tw + "\n",                            // no frontmatter
		"intro\n---\ntaskwarrior: " + tw + "\n---\n",           // block not at the top
		"---\ntaskwarrior: " + tw + "\n",                       // never closed
		"---\nparent:\n  taskwarrior: " + tw + "-extra\n---\n", // not a uuid
		"---\ntitle: x\n---\n\ntaskwarrior: " + tw + "\n",      // only mentioned below
		"---- Task Type: URL ----\ntaskwarrior: " + tw + "\n",  // not a fence
	} {
		if _, ok := cleanTaskNote(note); ok {
			t.Errorf("treated as linked: %q", note)
		}
	}
}

// A project line only goes when it is the link's companion, and an indented
// one (some other key's child) stays.
func TestCleanTaskNoteKeepsNestedProject(t *testing.T) {
	note := "---\ntaskwarrior: " + tw + "\nmeta:\n  project: keep\n---\nbody\n"
	got, _ := cleanTaskNote(note)
	if want := "---\nmeta:\n  project: keep\n---\nbody\n"; got.note != want {
		t.Errorf("note = %q, want %q", got.note, want)
	}
}

func TestIsConflictCopy(t *testing.T) {
	for title, want := range map[string]bool{
		"Today (conflict 2026-10-13)":   true,
		"Today":                         false,
		"Today (conflict)":              false,
		"(conflict 2026-10-13) Today":   false,
		"Today (conflict 2026-10-13) x": false,
	} {
		if got := isConflictCopy(title); got != want {
			t.Errorf("isConflictCopy(%q) = %v, want %v", title, got, want)
		}
	}
}
