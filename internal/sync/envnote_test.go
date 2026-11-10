package sync

import (
	"strings"
	"testing"
)

func TestEnvOverrideNote(t *testing.T) {
	for _, name := range pgEnvOverrides {
		t.Setenv(name, "")
	}
	if note := EnvOverrideNote(); note != "" {
		t.Errorf("with nothing set, the note is %q", note)
	}
	t.Setenv("VIMANGO_PG_PASSWORD", "not-the-real-one")
	note := EnvOverrideNote()
	if !strings.Contains(note, "VIMANGO_PG_PASSWORD") || strings.Contains(note, "VIMANGO_PG_SSL_MODE") {
		t.Errorf("with the password set, the note is %q", note)
	}
	if strings.Contains(note, "not-the-real-one") {
		t.Errorf("the note shows the value: %q", note)
	}
}
