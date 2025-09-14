package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ResearchManager struct {
	app           *App
	client        *http.Client
	apiKey        string
	queue         chan *ResearchTask
	running       map[string]*ResearchTask
	mutex         sync.RWMutex
	done          chan bool
	lastDebugInfo string // Store debug info for research note
	lastUsage     Usage  // Store usage statistics
	lastSearchCount int  // Count of web searches performed
	lastFetchCount  int  // Count of web fetches performed
}

type ResearchTask struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Prompt      string    `json:"prompt"`
	Status      string    `json:"status"` // pending, running, completed, failed
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Result      string    `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	SourceEntry int       `json:"source_entry"` // ID of the entry containing the research prompt
	DebugMode   bool      `json:"debug_mode"`   // Whether to include full debug info
}

type ClaudeRequest struct {
	Model     string      `json:"model"`
	MaxTokens int         `json:"max_tokens"`
	Messages  []Message   `json:"messages"`
	Tools     []Tool      `json:"tools"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Type             string        `json:"type"`
	Name             string        `json:"name"`
	MaxUses          int           `json:"max_uses,omitempty"`
	AllowedDomains   []string      `json:"allowed_domains,omitempty"`
	BlockedDomains   []string      `json:"blocked_domains,omitempty"`
	UserLocation     *UserLocation `json:"user_location,omitempty"`
	Citations        *Citations    `json:"citations,omitempty"`
	MaxContentTokens int           `json:"max_content_tokens,omitempty"`
}

type Citations struct {
	Enabled bool `json:"enabled"`
}

type UserLocation struct {
	Type     string `json:"type"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type ClaudeResponse struct {
	Content    []ContentBlock `json:"content"`
	Usage      Usage          `json:"usage"`
	StopReason string         `json:"stop_reason,omitempty"`
}

type ContentBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	ToolUse      *ToolUseBlock         `json:"tool_use,omitempty"`
	ToolResult   *ToolResultBlock      `json:"tool_result,omitempty"`
	Raw          map[string]interface{} `json:"-"` // For debugging unknown fields
}

type ToolUseBlock struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Input interface{} `json:"input"`
}

type ToolResultBlock struct {
	ToolUseID string        `json:"tool_use_id"`
	Content   []interface{} `json:"content"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func NewResearchManager(app *App, apiKey string) *ResearchManager {
	rm := &ResearchManager{
		app:     app,
		client:  &http.Client{Timeout: 300 * time.Second}, // 5 minute timeout for research
		apiKey:  apiKey,
		queue:   make(chan *ResearchTask, 10),
		running: make(map[string]*ResearchTask),
		done:    make(chan bool),
	}

	// Start background worker
	go rm.worker()

	return rm
}

// testAPIConnection performs a simple API test to validate key and permissions
func (rm *ResearchManager) testAPIConnection() {
	rm.logDebug("Testing Claude API connection and web search permissions...")
	
	// Simple test request with web search tool
	testRequest := ClaudeRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello, can you perform a web search? Just respond with a brief acknowledgment.",
			},
		},
		Tools: []Tool{
			{
				Type:    "web_search_20250305",
				Name:    "web_search",
				MaxUses: 1,
			},
		},
	}

	result, err := rm.callClaudeAPI(testRequest, true) // Enable debug mode for connection test
	if err != nil {
		rm.logDebug("API connection test FAILED: %v", err)
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "unauthorized") {
			rm.app.addNotification("⚠️ Research: API key may not have web search permissions")
		} else if strings.Contains(err.Error(), "invalid") {
			rm.app.addNotification("⚠️ Research: Invalid API key configuration")  
		} else {
			rm.app.addNotification("⚠️ Research: API connection test failed - check network/config")
		}
	} else {
		rm.logDebug("API connection test PASSED: %d characters returned", len(result))
		rm.app.addNotification("✅ Research: API connection and web search permissions verified")
	}
}

func (rm *ResearchManager) worker() {
	for {
		select {
		case task := <-rm.queue:
			rm.processTask(task)
		case <-rm.done:
			return
		}
	}
}

func (rm *ResearchManager) processTask(task *ResearchTask) {
	// Add panic recovery for the entire task processing
	defer func() {
		if r := recover(); r != nil {
			rm.logDebug("PANIC in processTask for task %s: %v", task.ID, r)
			
			rm.mutex.Lock()
			task.Status = "failed"
			task.Error = fmt.Sprintf("Task processing panic: %v", r)
			task.EndTime = time.Now()
			delete(rm.running, task.ID)
			rm.mutex.Unlock()
			
			rm.notifyCompletion(task)
		}
	}()

	rm.logDebug("Starting processing for task %s: %s", task.ID, task.Title)

	rm.mutex.Lock()
	task.Status = "running"
	task.StartTime = time.Now()
	rm.running[task.ID] = task
	rm.mutex.Unlock()

	rm.logDebug("Task %s marked as running, performing research...", task.ID)

	// Perform the research with additional validation
	result, err := rm.performResearch(task.Prompt, task.DebugMode)
	
	rm.logDebug("Research completed for task %s, result length: %d, error: %v", 
		task.ID, len(result), err != nil)

	rm.mutex.Lock()
	task.EndTime = time.Now()
	
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		rm.logDebug("Task %s marked as failed: %s", task.ID, task.Error)
	} else {
		task.Status = "completed"
		task.Result = result
		rm.logDebug("Task %s marked as completed, creating research note...", task.ID)
		
		// Create new note with research results
		// Run in separate goroutine but with error handling
		go func(t *ResearchTask) {
			defer func() {
				if r := recover(); r != nil {
					rm.logDebug("PANIC in research note creation goroutine: %v", r)
				}
			}()
			rm.createResearchNote(t)
		}(task)
	}
	
	// Remove from running tasks
	delete(rm.running, task.ID)
	rm.mutex.Unlock()

	rm.logDebug("Task %s processing completed, sending notification", task.ID)

	// Notify user
	rm.notifyCompletion(task)
}

func (rm *ResearchManager) performResearch(prompt string, debugMode bool) (string, error) {
	// Enhanced research prompt for comprehensive investigation
	researchPrompt := fmt.Sprintf(`You are conducting deep research on the following topic. Please provide a comprehensive analysis with multiple perspectives, current information, and proper citations.

Research Topic/Instructions:
%s

Please structure your response as a detailed markdown document with:
1. Executive Summary
2. Key Findings (with sections and subsections as appropriate)  
3. Different Perspectives/Viewpoints
4. Current Status/Latest Developments
5. Implications and Analysis
6. Sources and References

IMPORTANT - You have access to both web search and web fetch tools:
1. **First**: Use web search to discover and evaluate sources on multiple aspects of this topic
2. **Then**: Use web fetch to access complete content from the most promising sources for deeper analysis
3. **Prioritize web fetch for**: Scientific papers, botanical databases, official plant guides, comprehensive articles, authoritative sources with detailed information
4. **Combine tools effectively**: Search provides breadth, fetch provides depth - use both for comprehensive coverage
5. **Citations**: Cite your sources clearly with URLs, especially for fetched content
6. **Goal**: Provide a thorough analysis that combines broad discovery (search) with deep document analysis (fetch) suitable for someone who needs complete understanding of this topic.

Use web fetch liberally on high-quality sources to provide the most comprehensive research possible.`, prompt)

	// Configure web search tool with better parameters
	webSearchTool := Tool{
		Type:    "web_search_20250305",
		Name:    "web_search",
		MaxUses: 15, // Increased for more thorough research
		UserLocation: &UserLocation{
			Type:    "approximate",
			Country: "US",
			Timezone: "America/New_York",
		},
	}

	// Configure web fetch tool for deep content analysis
	webFetchTool := Tool{
		Type:             "web_fetch_20250910",
		Name:             "web_fetch",
		MaxUses:          8, // Balanced number for comprehensive analysis
		Citations:        &Citations{Enabled: true}, // Enable citations for fetched content
		MaxContentTokens: 100000, // Reasonable limit for large documents
	}

	request := ClaudeRequest{
		Model:     "claude-sonnet-4-20250514", // Using Sonnet 4 with web fetch support
		MaxTokens: 4000,
		Messages: []Message{
			{
				Role:    "user",
				Content: researchPrompt,
			},
		},
		Tools: []Tool{webSearchTool, webFetchTool},
	}

	rm.logDebug("Starting research with combined web search and fetch tools")

	result, err := rm.callClaudeAPI(request, debugMode)
	if err != nil {
		// Check for common API permission issues
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "unauthorized") {
			return "", fmt.Errorf("Claude API key may not have web search or web fetch permissions enabled. Please check your API key settings at console.anthropic.com. Original error: %w", err)
		}

		// Check for web fetch specific errors
		if strings.Contains(err.Error(), "web_fetch") || strings.Contains(err.Error(), "web-fetch") {
			return "", fmt.Errorf("web fetch tool may not be enabled for your API key or the model may not support it. Ensure you have web fetch permissions and are using a supported model (claude-sonnet-4). Original error: %w", err)
		}

		if strings.Contains(err.Error(), "beta") || strings.Contains(err.Error(), "web-fetch-2025-09-10") {
			return "", fmt.Errorf("web fetch beta feature may not be available. Check that the beta header is correctly set and your API key has beta access. Original error: %w", err)
		}

		if strings.Contains(err.Error(), "tool_use") {
			return "", fmt.Errorf("tool use may require a different conversation approach. This could indicate API limitations, configuration issues, or model compatibility problems. Original error: %w", err)
		}

		return "", fmt.Errorf("research failed: %w", err)
	}

	// Validate that we got substantial content
	if len(strings.TrimSpace(result)) < 200 {
		return "", fmt.Errorf("research returned insufficient content (%d characters). This may indicate web search is not functioning properly", len(result))
	}

	rm.logDebug("Research completed successfully, %d characters returned", len(result))
	return result, nil
}

func (rm *ResearchManager) callClaudeAPI(request ClaudeRequest, debugMode bool) (string, error) {
	// Generate unique request ID for correlation
	requestID := fmt.Sprintf("research_%d", time.Now().UnixNano())
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Save request to debug file (only in debug mode)
	if debugMode {
		rm.saveDebugData(requestID+"_request.json", jsonData)
	}
	rm.logDebug("[%s] Making Claude API request with %d tools, %d tokens", requestID, len(request.Tools), request.MaxTokens)

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", rm.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "web-fetch-2025-09-10")

	// Log request headers (without API key)
	rm.logDebug("[%s] Request headers: Content-Type=%s, anthropic-version=%s", 
		requestID, req.Header.Get("Content-Type"), req.Header.Get("anthropic-version"))

	resp, err := rm.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for logging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Save raw response to debug file (only in debug mode)
	if debugMode {
		rm.saveDebugData(requestID+"_response_raw.json", body)
	}
	rm.logDebug("[%s] Received response: status=%d, content-length=%d", requestID, resp.StatusCode, len(body))

	if resp.StatusCode != http.StatusOK {
		rm.logDebug("[%s] API request failed with status %d: %s", requestID, resp.StatusCode, string(body))
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		rm.logDebug("[%s] Failed to decode JSON response: %s", requestID, string(body))
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Save parsed response to debug file (only in debug mode)
	if debugMode {
		parsedData, _ := json.MarshalIndent(claudeResp, "", "  ")
		rm.saveDebugData(requestID+"_response_parsed.json", parsedData)
	}

	// Log detailed response information
	rm.logDebug("[%s] Claude API Response - StopReason: '%s', ContentBlocks: %d, Usage: %d input/%d output tokens", 
		requestID, claudeResp.StopReason, len(claudeResp.Content), claudeResp.Usage.InputTokens, claudeResp.Usage.OutputTokens)

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	// Store usage information and search count
	rm.lastUsage = claudeResp.Usage

	// Detailed analysis of each content block
	var textParts []string
	var hasToolUse bool
	var searchCount int
	var fetchCount int
	var debugInfo []string

	debugInfo = append(debugInfo, fmt.Sprintf("=== API RESPONSE ANALYSIS [%s] ===", requestID))
	debugInfo = append(debugInfo, fmt.Sprintf("Stop Reason: %s", claudeResp.StopReason))
	debugInfo = append(debugInfo, fmt.Sprintf("Content Blocks: %d", len(claudeResp.Content)))
	debugInfo = append(debugInfo, fmt.Sprintf("Usage: %d input, %d output tokens", claudeResp.Usage.InputTokens, claudeResp.Usage.OutputTokens))
	debugInfo = append(debugInfo, "")

	for i, block := range claudeResp.Content {
		blockInfo := fmt.Sprintf("Block %d: Type='%s'", i, block.Type)
		
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
				blockInfo += fmt.Sprintf(" - Text Length: %d chars", len(block.Text))
				debugInfo = append(debugInfo, blockInfo)
				debugInfo = append(debugInfo, fmt.Sprintf("Text Preview: %s...", 
					truncateString(block.Text, 100)))
				rm.logDebug("[%s] Added text block %d (%d chars)", requestID, i, len(block.Text))
			} else {
				blockInfo += " - Empty text"
				debugInfo = append(debugInfo, blockInfo)
			}
		case "tool_use", "server_tool_use":
			hasToolUse = true
			// Tool use blocks indicate tool execution but don't reliably contain tool names
			// We count actual executions from the result blocks instead
			if block.ToolUse != nil {
				toolName := block.ToolUse.Name
				blockInfo += fmt.Sprintf(" - %s tool use detected (id: %s)", toolName, block.ToolUse.ID)
				rm.logDebug("[%s] Tool use block %d: %s (id: %s)", requestID, i, toolName, block.ToolUse.ID)
			} else {
				blockInfo += " - Tool use detected (awaiting results)"
				rm.logDebug("[%s] Tool use block %d: tool data not available", requestID, i)
			}
			debugInfo = append(debugInfo, blockInfo)
		case "web_search_tool_result":
			searchCount++ // Count each web search result block
			blockInfo += " - Web search results"
			if block.ToolResult != nil {
				blockInfo += fmt.Sprintf(" (tool_use_id: %s, content items: %d)",
					block.ToolResult.ToolUseID, len(block.ToolResult.Content))
			}
			debugInfo = append(debugInfo, blockInfo)
			rm.logDebug("[%s] Web search results block %d detected (search #%d)", requestID, i, searchCount)
		case "web_fetch_tool_result":
			fetchCount++ // Count each web fetch result block
			blockInfo += " - Web fetch results"
			if block.ToolResult != nil {
				blockInfo += fmt.Sprintf(" (tool_use_id: %s, content items: %d)",
					block.ToolResult.ToolUseID, len(block.ToolResult.Content))
			}
			debugInfo = append(debugInfo, blockInfo)
			rm.logDebug("[%s] Web fetch results block %d detected (fetch #%d)", requestID, i, fetchCount)
		default:
			blockInfo += fmt.Sprintf(" - Unknown type")
			debugInfo = append(debugInfo, blockInfo)
			rm.logDebug("[%s] Unknown content block type: %s", requestID, block.Type)
		}
	}

	// Store search and fetch counts and debug info for research note
	rm.lastSearchCount = searchCount
	rm.lastFetchCount = fetchCount
	debugInfo = append(debugInfo, "")
	debugInfo = append(debugInfo, "=== TOOL USAGE SUMMARY ===")
	debugInfo = append(debugInfo, fmt.Sprintf("Web Searches Performed: %d (counted from result blocks)", searchCount))
	debugInfo = append(debugInfo, fmt.Sprintf("Web Fetches Performed: %d (counted from result blocks)", fetchCount))
	debugInfo = append(debugInfo, fmt.Sprintf("Total Research Actions: %d", searchCount+fetchCount))
	rm.lastDebugInfo = strings.Join(debugInfo, "\n")

	if len(textParts) == 0 {
		errorMsg := fmt.Sprintf("No text content found in %d content blocks", len(claudeResp.Content))
		if hasToolUse && claudeResp.StopReason == "tool_use" {
			errorMsg += " - Response contains only tool use blocks, may require conversation continuation"
		}
		rm.logDebug("[%s] %s", requestID, errorMsg)
		return "", fmt.Errorf("%s", errorMsg)
	}

	// Combine all text parts
	finalResponse := strings.Join(textParts, "\n\n")
	rm.logDebug("[%s] Successfully extracted %d text blocks, total length: %d characters", 
		requestID, len(textParts), len(finalResponse))

	return finalResponse, nil
}

func (rm *ResearchManager) saveDebugData(filename string, data []byte) {
	// Add panic recovery for file operations
	defer func() {
		if r := recover(); r != nil {
			rm.logDebug("PANIC in saveDebugData: %v", r)
		}
	}()

	// Validate inputs
	if filename == "" {
		rm.logDebug("Warning: Empty filename provided to saveDebugData")
		return
	}
	if len(data) == 0 {
		rm.logDebug("Warning: Empty data provided for file %s", filename)
		return
	}

	// Create debug directory if it doesn't exist with error handling
	debugDir := "vimango_research_debug"
	err := os.MkdirAll(debugDir, 0755)
	if err != nil {
		rm.logDebug("Failed to create debug directory %s: %v", debugDir, err)
		return
	}
	
	// Validate and create safe file path
	fullPath := filepath.Join(debugDir, filepath.Base(filename)) // Use Base to prevent path traversal
	
	// Attempt to write file with proper error handling
	err = os.WriteFile(fullPath, data, 0644)
	if err != nil {
		rm.logDebug("Failed to save debug data to %s: %v", fullPath, err)
		// Don't fail the entire operation if debug file saving fails
	} else {
		rm.logDebug("Saved debug data to %s (%d bytes)", fullPath, len(data))
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getResearchQualityDescription(inputTokens, searchCount, fetchCount int) string {
	// Consider both search and fetch activity for quality assessment
	totalResearchActions := searchCount + fetchCount

	if inputTokens > 40000 && fetchCount >= 3 && searchCount >= 3 {
		return "Premium Deep Research (extensive search + full document analysis)"
	} else if inputTokens > 30000 && (fetchCount >= 2 || (searchCount >= 3 && fetchCount >= 1)) {
		return "Comprehensive (thorough web research with document analysis)"
	} else if inputTokens > 20000 && (fetchCount >= 1 || searchCount >= 2) {
		return "Detailed (moderate web research)"
	} else if inputTokens > 10000 && totalResearchActions >= 1 {
		return "Standard (basic research)"
	} else {
		return "Limited (minimal research)"
	}
}

// logDebug adds debug logging for research operations
func (rm *ResearchManager) logDebug(format string, args ...interface{}) {
	// Only add to notification queue - no console output to avoid interfering with terminal UI
	message := fmt.Sprintf("[Research Debug] "+format, args...)
	rm.app.addNotification(message)
}

func (rm *ResearchManager) createResearchNote(task *ResearchTask) {
	// Add panic recovery to prevent crashes
	defer func() {
		if r := recover(); r != nil {
			rm.logDebug("PANIC in createResearchNote: %v", r)
			rm.app.addNotification(fmt.Sprintf("⚠️ Research note creation failed: %v", r))
		}
	}()

	rm.logDebug("Starting research note creation for task %s", task.ID)

	// Safely create note title with validation
	if task.Title == "" {
		task.Title = "Untitled Research"
	}
	title := fmt.Sprintf("Research: %s (%s)", task.Title, time.Now().Format("2006-01-02 15:04"))
	rm.logDebug("Created research note title: %s", title)
	
	// Build sections based on debug mode
	usageSection := ""
	debugSection := ""

	// Always include basic usage statistics
	if rm.lastUsage.InputTokens > 0 || rm.lastUsage.OutputTokens > 0 {
		usageSection = fmt.Sprintf(`

## Research Statistics

- **Duration:** %v
- **Web Searches:** %d
- **Web Fetches:** %d
- **Claude Usage:** %d input tokens, %d output tokens
- **Research Quality:** %s

`,
			task.EndTime.Sub(task.StartTime).Round(time.Second),
			rm.lastSearchCount,
			rm.lastFetchCount,
			rm.lastUsage.InputTokens,
			rm.lastUsage.OutputTokens,
			getResearchQualityDescription(rm.lastUsage.InputTokens, rm.lastSearchCount, rm.lastFetchCount))
	}

	// Only include full debug info if in debug mode
	if task.DebugMode && rm.lastDebugInfo != "" {
		rm.logDebug("Building debug section, debug info length: %d", len(rm.lastDebugInfo))
		
		// Safe string formatting with validation
		taskID := "unknown"
		if task.ID != "" {
			taskID = task.ID
		}
		
		duration := time.Duration(0)
		if !task.EndTime.IsZero() && !task.StartTime.IsZero() {
			duration = task.EndTime.Sub(task.StartTime).Round(time.Second)
		}

		debugSection = fmt.Sprintf(`

## DEBUG INFORMATION

%s

### API Request Details
- Task ID: %s
- Research Duration: %v
- Source Entry ID: %d
- Start Time: %s
- End Time: %s

### Tool Configuration
- Web Search Tool: web_search_20250305 (Max Uses: 15)
- Web Fetch Tool: web_fetch_20250910 (Max Uses: 8)
- User Location: US, America/New_York
- API Version: anthropic-version 2023-06-01
- Beta Headers: web-fetch-2025-09-10

### Files Generated
Debug files saved in vimango_research_debug/ directory:
- [requestID]_request.json - Full API request JSON
- [requestID]_response_raw.json - Raw API response
- [requestID]_response_parsed.json - Parsed response structure

`, rm.lastDebugInfo, taskID, duration, task.SourceEntry,
			task.StartTime.Format("2006-01-02 15:04:05"),
			task.EndTime.Format("2006-01-02 15:04:05"))
	} else {
		rm.logDebug("No debug info available for research note")
	}
	
	// Safely validate task result
	result := task.Result
	if result == "" {
		result = "No research results were generated."
		rm.logDebug("Warning: Empty research result for task %s", task.ID)
	}
	
	// Create research note with appropriate level of information
	var troubleshootingNote string
	if task.DebugMode {
		troubleshootingNote = "\n\n**Debug Mode:** Full debugging information included above."
	} else {
		troubleshootingNote = "\n\n**Note:** For detailed debugging information, use `:researchdebug` instead of `:research`."
	}

	markdown := fmt.Sprintf(`# %s

%s

%s

---

*This research was generated automatically by vimango's deep research feature using Claude AI with web search capabilities.*%s
`, title, result, usageSection, troubleshootingNote)

	// Add debug section if in debug mode
	if task.DebugMode && debugSection != "" {
		markdown += fmt.Sprintf("\n\n---\n%s\n---", debugSection)
	}

	rm.logDebug("Created markdown content, length: %d characters", len(markdown))

	// Create a new row for the research result with validation
	row := &Row{
		id:    -1,
		title: title,
		star:  false,
		dirty: true,
	}
	rm.logDebug("Created row for research note")

	// Safely insert the new entry with error checking
	rm.logDebug("Attempting to insert research note title...")
	err := rm.app.Database.insertTitle(row, 1, 1)
	if err != nil {
		rm.logDebug("ERROR inserting research note title: %v", err)
		rm.app.addNotification(fmt.Sprintf("⚠️ Failed to create research note: %v", err))
		return
	}
	rm.logDebug("Successfully inserted research note with ID: %d", row.id)

	// Safely update the note content with validation
	rm.logDebug("Attempting to update note content for ID %d...", row.id)
	err = rm.app.Database.updateNote(row.id, markdown)
	if err != nil {
		rm.logDebug("ERROR updating research note content: %v", err)
		rm.app.addNotification(fmt.Sprintf("⚠️ Failed to update research note content: %v", err))
		return
	}
	
	rm.logDebug("Research note created successfully with ID %d, includes debug information", row.id)
	rm.app.addNotification(fmt.Sprintf("✅ Research note created successfully (ID: %d)", row.id))
}

func (rm *ResearchManager) notifyCompletion(task *ResearchTask) {
	// This will be implemented to integrate with vimango's notification system
	// For now, we'll use a simple approach
	message := ""
	if task.Status == "completed" {
		message = fmt.Sprintf("✓ Research completed: %s", task.Title)
	} else {
		message = fmt.Sprintf("✗ Research failed: %s (%s)", task.Title, task.Error)
	}
	
	// Add to a notification queue that the main app can check
	rm.app.addNotification(message)
}

func (rm *ResearchManager) StartResearch(title, prompt string, sourceEntryID int, debugMode bool) (string, error) {
	taskID := fmt.Sprintf("research_%d_%d", time.Now().Unix(), sourceEntryID)
	
	task := &ResearchTask{
		ID:          taskID,
		Title:       title,
		Prompt:      prompt,
		Status:      "pending",
		SourceEntry: sourceEntryID,
		DebugMode:   debugMode,
	}

	// Add to queue
	select {
	case rm.queue <- task:
		return taskID, nil
	default:
		return "", fmt.Errorf("research queue is full, please try again later")
	}
}

func (rm *ResearchManager) GetRunningTasks() []*ResearchTask {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	tasks := make([]*ResearchTask, 0, len(rm.running))
	for _, task := range rm.running {
		tasks = append(tasks, task)
	}
	return tasks
}

func (rm *ResearchManager) Stop() {
	close(rm.done)
}