package auth // In a file like auth/drive.go

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// GetDriveService creates and returns an authenticated Google Drive service client.
// If no stored token exists it walks the user through the interactive OAuth
// flow on stdin/stdout.
func GetDriveService() (*drive.Service, error) {
	return getDriveService(false)
}

// GetDriveServiceHeadless is GetDriveService for non-interactive contexts
// (e.g. --render-html spawned by a host app with pipes for stdio): if no
// usable token.json exists it returns an error instead of blocking forever
// on the interactive OAuth prompt.
func GetDriveServiceHeadless() (*drive.Service, error) {
	return getDriveService(true)
}

func getDriveService(headless bool) (*drive.Service, error) {
	ctx := context.Background()

	// 1. Read the client secret file from Google Cloud Console.
	b, err := ioutil.ReadFile("go_credentials.json")
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %w", err)
	}

	// 2. Configure the OAuth2 client with the required scopes.
	// The drive.DriveReadonlyScope is sufficient for searching and reading files.
	config, err := google.ConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
	}

	// 3. Get the token. The file token.json stores the user's access and
	// refresh tokens; it is created by the interactive web-based auth flow,
	// which headless callers must not fall into.
	tok, err := tokenFromFile("token.json")
	if err != nil {
		if headless {
			return nil, fmt.Errorf("no stored Google token (token.json): %w", err)
		}
		tok = getTokenFromWeb(config)
		saveToken("token.json", tok)
	}
	client := config.Client(ctx, tok)

	// 4. Use the authenticated client to create the Drive service.
	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Drive client: %w", err)
	}

	return srv, nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// GetGDriveFileInfo retrieves the filename and parent folder name for a Google Drive file.
// Returns (filename, folderName, error). If the file is in the root, folderName will be "My Drive".
func GetGDriveFileInfo(srv *drive.Service, fileID string) (string, string, error) {
	// Get the file's name and parent IDs
	file, err := srv.Files.Get(fileID).Fields("name, parents").Do()
	if err != nil {
		return "", "", fmt.Errorf("failed to get file metadata: %w", err)
	}

	fileName := file.Name
	folderName := "My Drive" // Default if no parent

	if len(file.Parents) > 0 {
		// Get the parent folder's name
		parentID := file.Parents[0]
		parent, err := srv.Files.Get(parentID).Fields("name").Do()
		if err != nil {
			// Return filename even if we can't get parent
			return fileName, "", fmt.Errorf("failed to get parent folder: %w", err)
		}
		folderName = parent.Name
	}

	return fileName, folderName, nil
}
