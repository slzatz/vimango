package main

import (
	"fmt"
	"os"

	"github.com/slzatz/vimango/auth"
	"google.golang.org/api/drive/v3"
)

// CheckForGDriveAuth reports whether --gdrive-auth was given.
func CheckForGDriveAuth(args []string) bool {
	for _, arg := range args {
		if arg == "--gdrive-auth" {
			return true
		}
	}
	return false
}

// RunGDriveAuth signs in to Google Drive and exits: it gives token setup a
// name, so hosts (the hybrid app) and image placeholders can say "run
// vimango --gdrive-auth" instead of relying on the TUI boot path prompting
// as a side effect. Verifies an existing token.json with a live API call
// and re-runs the interactive OAuth flow if the token no longer works
// (expired or revoked). Returns a process exit code.
func RunGDriveAuth() int {
	if _, err := os.Stat("go_credentials.json"); err != nil {
		fmt.Println("go_credentials.json not found — the OAuth client secret is required first.")
		fmt.Println("Copy it from another machine, or create one following README.md → Google Drive Setup.")
		return 1
	}

	// Runs the interactive flow (URL + code prompt) if token.json is absent.
	srv, err := auth.GetDriveService()
	if err == nil {
		if email, verifyErr := driveAccountEmail(srv); verifyErr == nil {
			fmt.Printf("Google Drive sign-in OK — token.json is valid (account: %s).\n", email)
			return 0
		} else {
			fmt.Printf("Stored token.json no longer works (%v) — starting a fresh sign-in.\n", verifyErr)
		}
	} else {
		fmt.Printf("Stored credentials unusable (%v) — starting a fresh sign-in.\n", err)
	}

	if err := os.Remove("token.json"); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Could not remove stale token.json: %v\n", err)
		return 1
	}
	srv, err = auth.GetDriveService()
	if err != nil {
		fmt.Printf("Google Drive sign-in failed: %v\n", err)
		return 1
	}
	email, verifyErr := driveAccountEmail(srv)
	if verifyErr != nil {
		fmt.Printf("Sign-in completed but the token does not work: %v\n", verifyErr)
		return 1
	}
	fmt.Printf("Google Drive sign-in OK — token.json saved (account: %s).\n", email)
	return 0
}

// driveAccountEmail makes a lightweight API call to prove the token works,
// returning the signed-in account for the confirmation message.
func driveAccountEmail(srv *drive.Service) (string, error) {
	about, err := srv.About.Get().Fields("user(emailAddress)").Do()
	if err != nil {
		return "", err
	}
	return about.User.EmailAddress, nil
}
