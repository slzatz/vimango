package sync

import (
	"os"
	"strings"
)

// The environment variables that override the "postgres" block of
// config.json, in vimango, vimango-sync and vimango_hybrid alike.
var pgEnvOverrides = []string{"VIMANGO_PG_PASSWORD", "VIMANGO_PG_SSL_MODE", "VIMANGO_PG_SSL_CA_CERT"}

// EnvOverrideNote is a sentence for a failed connection's error message,
// naming the connection settings that came from the environment rather than
// config.json; "" when none did.
//
// An override is invisible: config.json holds the right password, the
// connection fails with "password authentication failed", and nothing says
// that a variable inherited from some shell profile or launcher was used
// instead. A program started from a terminal carries that terminal's
// variables, so the same Mac can sync from one window and fail from another.
func EnvOverrideNote() string {
	var set []string
	for _, name := range pgEnvOverrides {
		if os.Getenv(name) != "" {
			set = append(set, name)
		}
	}
	if len(set) == 0 {
		return ""
	}
	return "Note: " + strings.Join(set, " and ") + " is set in this program's environment and was used instead of config.json's value."
}
