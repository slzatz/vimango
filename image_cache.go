package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CacheEntry represents a single cached image entry
type CacheEntry struct {
	URL          string    `json:"url"`
	Filename     string    `json:"filename"`
	Created      time.Time `json:"created"`
	LastAccessed time.Time `json:"last_accessed"`
	SizeBytes    int64     `json:"size_bytes"`
	Width        int       `json:"width"`  // Image width in pixels
	Height       int       `json:"height"` // Image height in pixels
	// Kitty graphics metadata
	ImageID     uint32 `json:"image_id,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"` // Content hash or mtime+size signature
	LastCols    int    `json:"last_cols,omitempty"`   // Last terminal cols used with this ID
	LastRows    int    `json:"last_rows,omitempty"`   // Last terminal rows used with this ID
}

// CacheIndex represents the cache metadata structure
type CacheIndex struct {
	Version     int                   `json:"version"`
	Entries     map[string]CacheEntry `json:"entries"`
	NextImageID uint32                `json:"next_image_id,omitempty"`
	KittyWindow string                `json:"kitty_window,omitempty"`
}

// ImageCache manages the disk-based image cache
type ImageCache struct {
	cacheDir   string
	indexFile  string
	maxEntries int
	mutex      sync.RWMutex
	index      CacheIndex
}

// NewImageCache creates a new image cache instance
func NewImageCache() (*ImageCache, error) {
	// Use local project directory for cache
	cacheDir := filepath.Join(".", "image_cache")
	indexFile := filepath.Join(cacheDir, "cache_index.json")

	cache := &ImageCache{
		cacheDir:   cacheDir,
		indexFile:  indexFile,
		maxEntries: 50, // Default to 50 cached images
		index: CacheIndex{
			Version: 1,
			Entries: make(map[string]CacheEntry),
		},
	}

	// Create cache directory if it doesn't exist
	if err := cache.ensureCacheDirectory(); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %v", err)
	}

	// Load existing cache index
	if err := cache.loadIndex(); err != nil {
		log.Printf("Warning: Failed to load cache index, starting with empty cache: %v", err)
		// Continue with empty cache rather than failing
	}

	return cache, nil
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ensureCacheDirectory creates the cache directory with proper permissions
func (c *ImageCache) ensureCacheDirectory() error {
	// Create directory with read/write/execute for user, read/execute for group and others
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", c.cacheDir, err)
	}

	// Verify directory is writable
	testFile := filepath.Join(c.cacheDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("cache directory %s is not writable: %v", c.cacheDir, err)
	}
	os.Remove(testFile) // Clean up test file

	return nil
}

// generateCacheKey creates a hash-based key from a Google Drive URL
func (c *ImageCache) generateCacheKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	// Use first 16 characters of hex for shorter filenames while avoiding collisions
	return hex.EncodeToString(hash[:])[:16]
}

// loadIndex loads the cache index from disk
func (c *ImageCache) loadIndex() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if index file exists
	if _, err := os.Stat(c.indexFile); os.IsNotExist(err) {
		// No existing index, start with empty cache
		return nil
	}

	// Read and parse index file
	data, err := os.ReadFile(c.indexFile)
	if err != nil {
		return fmt.Errorf("failed to read index file: %v", err)
	}

	if err := json.Unmarshal(data, &c.index); err != nil {
		return fmt.Errorf("failed to parse index file: %v", err)
	}

	// Validate cache files exist and remove stale entries
	validEntries := make(map[string]CacheEntry)
	for key, entry := range c.index.Entries {
		cacheFile := filepath.Join(c.cacheDir, entry.Filename)
		if _, err := os.Stat(cacheFile); err == nil {
			validEntries[key] = entry
		} else {
			log.Printf("Removing stale cache entry for missing file: %s", entry.Filename)
		}
	}
	c.index.Entries = validEntries

	// Initialize NextImageID to avoid collisions across runs
	if c.index.NextImageID == 0 {
		var maxID uint32 = 50 // start above the early hard-coded 40 range
		for _, entry := range c.index.Entries {
			if entry.ImageID > maxID {
				maxID = entry.ImageID
			}
		}
		c.index.NextImageID = maxID + 1
	}

	return nil
}

// saveIndex saves the cache index to disk
func (c *ImageCache) saveIndex() error {
	// Note: Caller should hold write lock
	if cw := os.Getenv("KITTY_WINDOW_ID"); cw != "" {
		c.index.KittyWindow = cw
	}
	data, err := json.MarshalIndent(c.index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %v", err)
	}

	// Atomic write: write to temp file then rename
	tempFile := c.indexFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp index file: %v", err)
	}

	if err := os.Rename(tempFile, c.indexFile); err != nil {
		os.Remove(tempFile) // Clean up temp file on failure
		return fmt.Errorf("failed to rename temp index file: %v", err)
	}

	return nil
}

// NextKittyImageID reserves and returns the next kitty image ID, persisted in the index.
func (c *ImageCache) NextKittyImageID() uint32 {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.index.NextImageID == 0 {
		c.index.NextImageID = 50
	}
	id := c.index.NextImageID
	c.index.NextImageID++
	_ = c.saveIndex()
	return id
}

// GetKittyMeta returns the cache entry (including kitty fields) if present.
func (c *ImageCache) GetKittyMeta(url string) (CacheEntry, bool) {
	key := c.generateCacheKey(url)

	c.mutex.RLock()
	entry, exists := c.index.Entries[key]
	c.mutex.RUnlock()
	return entry, exists
}

// UpdateKittyMeta updates kitty-related fields and persists the index.
func (c *ImageCache) UpdateKittyMeta(url string, imageID uint32, cols, rows int, fingerprint string) error {
	key := c.generateCacheKey(url)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, exists := c.index.Entries[key]
	if !exists {
		return fmt.Errorf("cache entry not found for url: %s", url)
	}

	entry.ImageID = imageID
	entry.LastCols = cols
	entry.LastRows = rows
	if fingerprint != "" {
		entry.Fingerprint = fingerprint
	}
	entry.LastAccessed = time.Now()
	c.index.Entries[key] = entry

	// Ensure monotonic NextImageID
	if imageID >= c.index.NextImageID {
		c.index.NextImageID = imageID + 1
	}

	return c.saveIndex()
}

// evictOldestEntry removes the oldest cache entry (FIFO)
// Note: Caller must hold write lock
func (c *ImageCache) evictOldestEntry() error {
	if len(c.index.Entries) == 0 {
		return nil
	}

	// Find oldest entry by creation time
	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range c.index.Entries {
		if first || entry.Created.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Created
			first = false
		}
	}

	// Remove cache file
	if entry, exists := c.index.Entries[oldestKey]; exists {
		cacheFile := filepath.Join(c.cacheDir, entry.Filename)
		if err := os.Remove(cacheFile); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: Failed to remove cache file %s: %v", cacheFile, err)
		}
	}

	// Remove from index
	delete(c.index.Entries, oldestKey)

	return nil
}

// GetCacheStats returns basic cache statistics
func (c *ImageCache) GetCacheStats() (int, int64) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var totalSize int64
	for _, entry := range c.index.Entries {
		totalSize += entry.SizeBytes
	}

	return len(c.index.Entries), totalSize
}

// GetCachedImage retrieves cached data by URL (for webview data URIs)
// Returns (base64Data, true) if found, ("", false) if not found
func (c *ImageCache) GetCachedImage(url string) (string, bool) {
	data, _, _, found := c.GetCachedImageData(url)
	return data, found
}

// StoreCachedImage stores base64 data in cache (for webview data URIs without dimensions)
func (c *ImageCache) StoreCachedImage(url, base64Data string) error {
	return c.StoreCachedImageData(url, base64Data, 0, 0)
}

// GetCachedImageData retrieves cached base64 image data and pixel dimensions by URL
// Returns (base64Data, width, height, exists)
// Width and height are the original image dimensions in pixels (not terminal cells)
// Only stores data for Google Drive images (local files are fast to re-read)
func (c *ImageCache) GetCachedImageData(url string) (string, int, int, bool) {
	key := c.generateCacheKey(url)

	c.mutex.RLock()
	entry, exists := c.index.Entries[key]
	c.mutex.RUnlock()

	if !exists {
		return "", 0, 0, false
	}

	// Check if cache file still exists
	cacheFile := filepath.Join(c.cacheDir, entry.Filename)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		// File missing or unreadable - remove from index
		c.mutex.Lock()
		delete(c.index.Entries, key)
		c.saveIndex() // Best effort, ignore errors
		c.mutex.Unlock()
		return "", 0, 0, false
	}

	// Update last accessed time
	c.mutex.Lock()
	entry.LastAccessed = time.Now()
	c.index.Entries[key] = entry
	c.saveIndex() // Best effort, ignore errors
	c.mutex.Unlock()

	return string(data), entry.Width, entry.Height, true
}

// StoreCachedImageData stores base64 image data and pixel dimensions in cache
// width and height are the original image dimensions in pixels (not terminal cells)
// Only call this for Google Drive images (local files don't need caching)
func (c *ImageCache) StoreCachedImageData(url, base64Data string, width, height int) error {
	key := c.generateCacheKey(url)
	filename := key + ".b64"
	cacheFile := filepath.Join(c.cacheDir, filename)
	fingerprint := hashString(base64Data)

	// Write image data to cache file atomically
	tempFile := cacheFile + ".tmp"
	if err := os.WriteFile(tempFile, []byte(base64Data), 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %v", err)
	}

	if err := os.Rename(tempFile, cacheFile); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("failed to rename cache file: %v", err)
	}

	// Get file size for index
	fileInfo, err := os.Stat(cacheFile)
	if err != nil {
		return fmt.Errorf("failed to get cache file size: %v", err)
	}

	// Update cache index
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if we need to evict entries before adding new one
	if len(c.index.Entries) >= c.maxEntries {
		if err := c.evictOldestEntry(); err != nil {
			log.Printf("Warning: Failed to evict cache entry: %v", err)
			// Continue anyway - better to have oversized cache than fail
		}
	}

	// Add new entry
	now := time.Now()
	c.index.Entries[key] = CacheEntry{
		URL:          url,
		Filename:     filename,
		Created:      now,
		LastAccessed: now,
		SizeBytes:    fileInfo.Size(),
		Width:        width,
		Height:       height,
		Fingerprint:  fingerprint,
	}

	// Save updated index
	if err := c.saveIndex(); err != nil {
		// If index save fails, try to clean up cache file
		os.Remove(cacheFile)
		return fmt.Errorf("failed to update cache index: %v", err)
	}

	return nil
}
