//go:build cgo && !windows

package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"

	"github.com/webview/webview_go"
)

var (
	// Mutex to ensure only one webview can run at a time
	webviewMutex sync.Mutex
	webviewRunning bool
	activeWebview webview.WebView
)

func init() {
	// Override the default - webview is available with CGO
	isWebviewAvailableDefault = true
}

// suppressStderr temporarily redirects stderr to /dev/null to prevent WebKit warnings
// from interfering with terminal applications
func suppressStderr() (func(), error) {
	// Save the original stderr
	originalStderr, err := syscall.Dup(int(os.Stderr.Fd()))
	if err != nil {
		return nil, err
	}
	
	// Open /dev/null
	devNull, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		syscall.Close(originalStderr)
		return nil, err
	}
	
	// Redirect stderr to /dev/null
	err = syscall.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		devNull.Close()
		syscall.Close(originalStderr)
		return nil, err
	}
	
	// Return cleanup function
	return func() {
		// Restore original stderr
		syscall.Dup2(originalStderr, int(os.Stderr.Fd()))
		devNull.Close()
		syscall.Close(originalStderr)
	}, nil
}

// IsWebviewAvailable returns true if webview is available
func IsWebviewAvailable() bool {
	return true
}

// IsWebviewAuthenticated checks if webview has Google Drive authentication
func IsWebviewAuthenticated() bool {
	webviewAuthMutex.RLock()
	defer webviewAuthMutex.RUnlock()
	
	if !authCheckPerformed {
		return false
	}
	
	return webviewAuthenticated
}

// OpenNoteInWebview opens a note in a webview window or updates existing one
func OpenNoteInWebview(title, htmlContent string) error {
	// Check if webview is already running (with minimal mutex usage)
	webviewMutex.Lock()
	if webviewRunning && activeWebview != nil {
		// Update existing webview content
		activeWebview.SetTitle(fmt.Sprintf("Vimango - %s", title))
		
		// TEST: Add JavaScript to check if we still have Google authentication
		// This will help us determine if SetHtml() destroys session state
		testHTML := `
		<!DOCTYPE html>
		<html>
		<head><title>Session Test</title></head>
		<body>
		<h2>Testing Session State</h2>
		<p>Checking if Google authentication cookies are still present...</p>
		<script>
		// Test if we can access Google domains
		fetch('https://drive.google.com/uc?id=test')
		.then(response => {
			if (response.status === 401 || response.status === 403) {
				document.body.innerHTML += '<p style="color: red;">❌ Authentication lost - cookies cleared by SetHtml()</p>';
			} else {
				document.body.innerHTML += '<p style="color: green;">✅ Authentication preserved - cookies maintained</p>';
			}
		})
		.catch(error => {
			document.body.innerHTML += '<p style="color: orange;">⚠️ Network error or CORS: ' + error.message + '</p>';
		});
		
		// Alternative test: check if document.cookie contains Google-related cookies
		setTimeout(() => {
			const cookies = document.cookie;
			if (cookies.includes('google') || cookies.length > 0) {
				document.body.innerHTML += '<p style="color: blue;">🍪 Cookies found: ' + cookies.substring(0, 100) + '...</p>';
			} else {
				document.body.innerHTML += '<p style="color: red;">🚫 No cookies found</p>';
			}
		}, 1000);
		</script>
		<hr>
		` + htmlContent + `
		</body>
		</html>`;
		
		activeWebview.SetHtml(testHTML)
		webviewMutex.Unlock()
		return nil
	}
	webviewRunning = true
	webviewMutex.Unlock()
	
	// Ensure we reset the flag when function exits
	defer func() {
		webviewMutex.Lock()
		webviewRunning = false
		activeWebview = nil
		webviewMutex.Unlock()
	}()
	
	// Suppress WebKit warnings to prevent terminal interference
	restoreStderr, err := suppressStderr()
	if err != nil {
		// Continue without stderr suppression if it fails
		log.Printf("Warning: Could not suppress stderr: %v", err)
	}
	defer func() {
		if restoreStderr != nil {
			restoreStderr()
		}
	}()
	
	w := webview.New(false) // false = not debug mode
	defer w.Destroy()

	// Store reference to active webview
	webviewMutex.Lock()
	activeWebview = w
	webviewMutex.Unlock()

	w.SetTitle(fmt.Sprintf("Vimango - %s", title))
	w.SetSize(1200, 800, webview.HintNone)
	
	// Add JavaScript for image loading diagnostics and authentication state
	w.Init(`
		(function() {
			// Track image loading success/failure for diagnostics
			var imageStats = { loaded: 0, failed: 0, total: 0 };
			
			function setupImageTracking() {
				var images = document.querySelectorAll('img');
				imageStats.total = images.length;
				
				images.forEach(function(img, index) {
					// Track load success
					img.addEventListener('load', function() {
						imageStats.loaded++;
						console.log('Image loaded successfully:', img.src);
						if (img.src.includes('drive.google.com')) {
							console.log('Google Drive image loaded via direct URL');
						}
					});
					
					// Track load failure
					img.addEventListener('error', function() {
						imageStats.failed++;
						console.log('Image failed to load:', img.src);
						if (img.src.includes('drive.google.com')) {
							console.log('Google Drive image failed - may need authentication');
							// Could trigger re-authentication here if needed
						}
					});
				});
			}
			
			// Run setup when DOM is ready
			if (document.readyState === 'loading') {
				document.addEventListener('DOMContentLoaded', setupImageTracking);
			} else {
				setupImageTracking();
			}
			
			// Expose stats to Go via binding
			window.getImageStats = function() {
				return JSON.stringify(imageStats);
			};
		})();
	`)

	// Bind JavaScript functions for diagnostics
	w.Bind("getImageStats", func() string {
		return ""  // Will be overridden by JavaScript
	})

	// Load the HTML content directly
	w.SetHtml(htmlContent)
	
	// Run the webview (this blocks until window is closed)
	// Mutex is NOT held during this call, allowing other operations
	w.Run()
	
	return nil
}

// IsWebviewRunning returns true if a webview is currently running
func IsWebviewRunning() bool {
	webviewMutex.Lock()
	defer webviewMutex.Unlock()
	return webviewRunning
}

// ShowWebviewUnavailableMessage shows a message when webview is not available
func ShowWebviewUnavailableMessage() {
	log.Println("Webview is available in this build")
}

// AuthenticateWebviewBrowser opens Google Drive in webview for browser-based signin
func AuthenticateWebviewBrowser() error {
	// Simple check - if already authenticated, we're done
	if IsWebviewAuthenticated() {
		return nil
	}
	
	// Suppress WebKit warnings to prevent terminal interference
	restoreStderr, err := suppressStderr()
	if err != nil {
		// Continue without stderr suppression if it fails
		log.Printf("Warning: Could not suppress stderr: %v", err)
	}
	defer func() {
		if restoreStderr != nil {
			restoreStderr()
		}
	}()
	
	// Create webview for authentication
	w := webview.New(false) // Disable debug mode to avoid terminal UI interference
	defer w.Destroy()

	// Set up webview
	w.SetTitle("Google Drive Sign In - Vimango")
	w.SetSize(900, 700, webview.HintNone)

	// Navigate to Google sign-in page directly - this should be more compatible with webview
	w.Navigate("https://accounts.google.com/signin")

	// Simple approach - user will manually complete authentication
	// No JavaScript injection into Google's pages

	// Set webview state for unified tracking
	webviewMutex.Lock()
	webviewRunning = true
	activeWebview = w
	webviewMutex.Unlock()

	// Ensure we reset the state when webview closes
	defer func() {
		webviewMutex.Lock()
		webviewRunning = false
		activeWebview = nil
		webviewMutex.Unlock()
	}()

	// Run webview - this blocks until window is closed
	// User will complete Google authentication manually in the webview
	// Then use :authdone command to set authentication state (while webview remains open)
	// Finally use <leader>w to display content in the same authenticated webview
	w.Run()

	// Webview closed by user - authentication workflow complete
	return nil
}

// OpenNoteInWebviewWithAuth opens a note in webview, handling authentication if needed
func OpenNoteInWebviewWithAuth(title, htmlContent string) error {
	// Open note in webview normally - authentication will be handled differently
	return OpenNoteInWebview(title, htmlContent)
}

// TriggerWebviewAuthentication prompts for webview browser authentication
func TriggerWebviewAuthentication() error {
	// Check if already authenticated
	if CheckWebviewAuthentication() {
		return nil
	}
	
	if err := AuthenticateWebviewBrowser(); err != nil {
		return err
	}
	
	return nil
}