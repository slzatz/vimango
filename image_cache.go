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
}

// CacheIndex represents the cache metadata structure
type CacheIndex struct {
	Version int                    `json:"version"`
	Entries map[string]CacheEntry  `json:"entries"`
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

	return nil
}

// saveIndex saves the cache index to disk
func (c *ImageCache) saveIndex() error {
	// Note: Caller should hold write lock
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

// GetCachedImage retrieves a cached image by URL
// Returns (base64Data, true) if found, ("", false) if not found
func (c *ImageCache) GetCachedImage(url string) (string, bool) {
	key := c.generateCacheKey(url)

	c.mutex.RLock()
	entry, exists := c.index.Entries[key]
	c.mutex.RUnlock()

	if !exists {
		return "", false
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
		return "", false
	}

	// Update last accessed time
	c.mutex.Lock()
	entry.LastAccessed = time.Now()
	c.index.Entries[key] = entry
	c.saveIndex() // Best effort, ignore errors
	c.mutex.Unlock()

	return string(data), true
}

// StoreCachedImage stores a base64 image in the cache
func (c *ImageCache) StoreCachedImage(url, base64Data string) error {
	key := c.generateCacheKey(url)
	filename := key + ".b64"
	cacheFile := filepath.Join(c.cacheDir, filename)

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
	}

	// Save updated index
	if err := c.saveIndex(); err != nil {
		// If index save fails, try to clean up cache file
		os.Remove(cacheFile)
		return fmt.Errorf("failed to update cache index: %v", err)
	}

	return nil
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

// GetCachedImageData retrieves cached base64 image data by URL
// Returns (base64Data, exists)
// Only stores data for Google Drive images (local files are fast to re-read)
func (c *ImageCache) GetCachedImageData(url string) (string, bool) {
	key := c.generateCacheKey(url)

	c.mutex.RLock()
	entry, exists := c.index.Entries[key]
	c.mutex.RUnlock()

	if !exists {
		return "", false
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
		return "", false
	}

	// Update last accessed time
	c.mutex.Lock()
	entry.LastAccessed = time.Now()
	c.index.Entries[key] = entry
	c.saveIndex() // Best effort, ignore errors
	c.mutex.Unlock()

	return string(data), true
}

// StoreCachedImageData stores base64 image data in cache
// Only call this for Google Drive images (local files don't need caching)
func (c *ImageCache) StoreCachedImageData(url, base64Data string) error {
	key := c.generateCacheKey(url)
	filename := key + ".b64"
	cacheFile := filepath.Join(c.cacheDir, filename)

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
	}

	// Save updated index
	if err := c.saveIndex(); err != nil {
		// If index save fails, try to clean up cache file
		os.Remove(cacheFile)
		return fmt.Errorf("failed to update cache index: %v", err)
	}

	return nil
}