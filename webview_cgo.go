//go:build cgo && !windows

package main

import (
	"fmt"
	"log"
	"sync"
	"time"

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

// IsWebviewAvailable returns true if webview is available
func IsWebviewAvailable() bool {
	return true
}

// OpenNoteInWebview opens a note in a webview window or updates existing one
func OpenNoteInWebview(title, htmlContent string) error {
	// Check if webview is already running (with minimal mutex usage)
	webviewMutex.Lock()
	if webviewRunning && activeWebview != nil {
		// Update existing webview content
		activeWebview.SetTitle(fmt.Sprintf("Vimango - %s", title))
		activeWebview.SetHtml(htmlContent)
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
	// Ensure only one webview can run at a time
	webviewMutex.Lock()
	defer webviewMutex.Unlock()
	
	// If a webview is already running, don't start authentication
	if webviewRunning {
		return fmt.Errorf("cannot authenticate: webview already running")
	}
	
	// Create webview for authentication
	w := webview.New(false) // Disable debug mode to avoid terminal UI interference
	defer w.Destroy()

	// Set up webview
	w.SetTitle("Google Drive Sign In - Vimango")
	w.SetSize(900, 700, webview.HintNone)
	
	// Mark webview as running
	webviewRunning = true
	defer func() {
		webviewRunning = false
	}()

	// Navigate to Google sign-in page directly - this should be more compatible with webview
	w.Navigate("https://accounts.google.com/signin")

	// Track authentication state
	authCompleted := make(chan bool, 1)

	// Set up JavaScript to detect successful signin and help with navigation
	w.Init(`
		(function() {
			function checkAuthentication() {
				// If we successfully reach Google Drive after signin
				if (window.location.hostname === 'drive.google.com') {
					// Look for elements that indicate successful signin
					var userElements = [
						'[data-hovercard-id]',
						'img[aria-label*="Account"]', 
						'[aria-label*="profile"]',
						'[data-tooltip*="Account"]',
						'.gb_d.gb_Db.gb_i' // Google apps menu
					];
					
					for (var i = 0; i < userElements.length; i++) {
						var element = document.querySelector(userElements[i]);
						if (element) {
							window.external.invoke('auth_success');
							return;
						}
					}
				}
				
				// If we're on accounts.google.com, check if we can proceed to Drive
				if (window.location.hostname === 'accounts.google.com') {
					// Check if signin is complete by looking for redirect or success indicators
					var signedIn = document.querySelector('.gb_d') || // Google bar
					              document.querySelector('[data-account-menu]') ||
					              window.location.href.includes('continue=');
					              
					if (signedIn) {
						window.location.href = 'https://drive.google.com';
						return;
					}
				}
				
				// Check again in 2 seconds
				setTimeout(checkAuthentication, 2000);
			}
			
			// Start checking after page loads
			if (document.readyState === 'loading') {
				document.addEventListener('DOMContentLoaded', function() {
					setTimeout(checkAuthentication, 3000);
				});
			} else {
				setTimeout(checkAuthentication, 3000);
			}
		})();
	`)

	// Bind callback for successful authentication
	w.Bind("auth_success", func() {
		authCompleted <- true
	})

	// Add instruction HTML overlay
	instructionHTML := `
		<div id="vimango-instructions" style="
			position: fixed; 
			top: 10px; 
			right: 10px; 
			background: #4285f4; 
			color: white; 
			padding: 15px; 
			border-radius: 8px; 
			font-family: Arial, sans-serif; 
			font-size: 14px; 
			max-width: 300px; 
			z-index: 10000;
			box-shadow: 0 4px 8px rgba(0,0,0,0.2);
		">
			<h3 style="margin: 0 0 10px 0;">Vimango Authentication</h3>
			<p style="margin: 0 0 10px 0;">Please sign in with your Google account to enable direct image loading.</p>
			<p style="margin: 0 0 10px 0; font-size: 12px;">1. Click "Sign in" if button works<br>2. Or manually go to drive.google.com after signin</p>
			<p style="margin: 0; font-size: 12px;">Window will close automatically when authenticated.</p>
		</div>
	`

	// Inject instructions after page loads
	w.Eval(fmt.Sprintf(`
		document.addEventListener('DOMContentLoaded', function() {
			if (!document.getElementById('vimango-instructions')) {
				document.body.insertAdjacentHTML('beforeend', %q);
			}
		});
	`, instructionHTML))

	// Set up a timeout mechanism that works with blocking Run()
	go func() {
		time.Sleep(10 * time.Minute)
		w.Terminate() // This will cause Run() to exit
	}()

	// Run webview - this blocks until window is closed or terminated
	w.Run()

	// Check if authentication was successful
	select {
	case <-authCompleted:
		// Authentication successful
		SetWebviewAuthenticated(true)
		return nil
	default:
		// Authentication not completed (user closed window or timeout)
		return fmt.Errorf("authentication cancelled or timed out")
	}
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