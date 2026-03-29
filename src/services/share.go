package services

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"

	"s3-storage/model"
)

const shareLinksFileName = ".shares"

// ShareLinkManager manages share links with in-memory caching
type ShareLinkManager struct {
	mu    sync.RWMutex
	path  string
	store model.ShareLinkStore
}

var shareLinkManager *ShareLinkManager

// InitShareLinkManager initializes the global share link manager
// storageDir is the directory where share links will be stored (e.g. config.StorageDirectory)
// Parameters:
//   - storageDir: Directory for storing share links (e.g. config.StorageDirectory)
//
// Returns:
//   - error: Any error encountered during initialization
//
// Example usage:
//
//	err := InitShareLinkManager(config.StorageDirectory)
//	if err != nil {
//	  // Handle error
//	}
func InitShareLinkManager(storageDir string) error {
	slm := &ShareLinkManager{
		path: filepath.Join(storageDir, shareLinksFileName),
	}
	if err := slm.Load(); err != nil {
		return err
	}
	shareLinkManager = slm
	return nil
}

// Load reads share links from disk, or initializes empty if not exists
// This should be called once at startup
// Returns:
//   - error: Any error encountered during loading
//
// Example usage:
//
//	err := shareLinkManager.Load()
//	if err != nil {
//	  // Handle error
//	}
func (slm *ShareLinkManager) Load() error {
	slm.mu.Lock()
	defer slm.mu.Unlock()

	data, err := os.ReadFile(slm.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize with empty store
			slm.store = model.ShareLinkStore{
				Version: 1,
				Links:   []model.ShareLink{},
			}
			return slm.saveUnlocked()
		}
		return err
	}

	// Decode msgpack
	_, err = slm.store.UnmarshalMsg(data)
	return err
}

// Save writes share links to disk atomically
// Returns:
//   - error: Any error encountered during saving
//
// Example usage:
//
//	err := shareLinkManager.Save()
//	if err != nil {
//	  // Handle error
//	}
func (slm *ShareLinkManager) Save() error {
	slm.mu.Lock()
	defer slm.mu.Unlock()
	return slm.saveUnlocked()
}

// saveUnlocked writes to disk without acquiring lock (caller must hold lock)
// Returns:
//   - error: Any error encountered during saving
//
// Example usage:
//
//	// Caller must hold slm.mu.Lock()
//	err := slm.saveUnlocked()
//	if err != nil {
//	  // Handle error
//	}
func (slm *ShareLinkManager) saveUnlocked() error {
	slm.store.Version++

	data, err := slm.store.MarshalMsg(nil)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(slm.path)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Atomic write: write to temp file, sync, then rename
	tmpPath := slm.path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err = f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	if err = f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	if err = f.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, slm.path)
}

// generateToken generates a random 32-character hex token
// Returns:
//   - string: The generated token
//   - error: Any error encountered during token generation
//
// Example usage:
//
//	token, err := generateToken()
//	if err != nil {
//	  // Handle error
//	}
func generateToken() (string, error) {
	bytes := make([]byte, 16) // 16 bytes = 32 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateShareLink creates a new share link for an object
// expiresIn is duration in seconds (0 means no expiration)
// Returns:
//   - string: The generated share link token
//   - error: Any error encountered during share link creation
//
// Example usage:
//
//	token, err := CreateShareLink("my-bucket", "path/to/object", 3600)
//	if err != nil {
//	  // Handle error
//	}
func CreateShareLink(bucket, key string, expiresIn int64) (string, error) {
	if shareLinkManager == nil {
		return "", ErrInternalError
	}

	// Verify object exists
	if !ObjectExists(bucket, key) {
		return "", ErrNoSuchKey
	}

	// Generate unique token
	token, err := generateToken()
	if err != nil {
		return "", err
	}

	shareLinkManager.mu.Lock()
	defer shareLinkManager.mu.Unlock()

	// Calculate expiration
	var expiresAt int64
	if expiresIn > 0 {
		expiresAt = time.Now().Unix() + expiresIn
	}

	// Add share link
	shareLink := model.ShareLink{
		Token:     token,
		Bucket:    bucket,
		Key:       key,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: expiresAt,
	}

	shareLinkManager.store.Links = append(shareLinkManager.store.Links, shareLink)

	if err = shareLinkManager.saveUnlocked(); err != nil {
		return "", err
	}

	return token, nil
}

// GetShareLink retrieves the bucket and key for a given token
// Returns:
//   - string: The bucket name associated with the token
//   - string: The object key associated with the token
//   - error: Any error encountered during retrieval (e.g. not found, expired)
//
// Example usage:
//
//	bucket, key, err := GetShareLink(token)
//	if err != nil {
//	  // Handle error
//	}
func GetShareLink(token string) (string, string, error) {
	if shareLinkManager == nil {
		return "", "", ErrInternalError
	}

	shareLinkManager.mu.RLock()
	defer shareLinkManager.mu.RUnlock()

	now := time.Now().Unix()
	for _, link := range shareLinkManager.store.Links {
		if link.Token == token {
			// Check if expired
			if link.ExpiresAt > 0 && now > link.ExpiresAt {
				return "", "", ErrShareLinkExpired
			}
			return link.Bucket, link.Key, nil
		}
	}

	return "", "", ErrShareLinkNotFound
}

// DeleteShareLink deletes a share link by token
// Returns:
//   - error: Any error encountered during deletion (e.g. not found)
//
// Example usage:
//
//	err := DeleteShareLink(token)
//	if err != nil {
//	  // Handle error
//	}
func DeleteShareLink(token string) error {
	if shareLinkManager == nil {
		return ErrInternalError
	}

	shareLinkManager.mu.Lock()
	defer shareLinkManager.mu.Unlock()

	idx := -1
	for i, link := range shareLinkManager.store.Links {
		if link.Token == token {
			idx = i
			break
		}
	}

	if idx == -1 {
		return ErrShareLinkNotFound
	}

	// Remove link using swap-and-truncate pattern
	lastIdx := len(shareLinkManager.store.Links) - 1
	shareLinkManager.store.Links[idx] = shareLinkManager.store.Links[lastIdx]
	shareLinkManager.store.Links = shareLinkManager.store.Links[:lastIdx]

	return shareLinkManager.saveUnlocked()
}

// ListShareLinks returns all share links for a specific object
func ListShareLinks(bucket, key string) ([]model.ShareLink, error) {
	if shareLinkManager == nil {
		return nil, ErrInternalError
	}

	shareLinkManager.mu.RLock()
	defer shareLinkManager.mu.RUnlock()

	var result []model.ShareLink
	now := time.Now().Unix()

	for _, link := range shareLinkManager.store.Links {
		if link.Bucket == bucket && link.Key == key {
			// Skip expired links
			if link.ExpiresAt > 0 && now > link.ExpiresAt {
				continue
			}
			result = append(result, link)
		}
	}

	return result, nil
}

// CleanupExpiredLinks removes all expired share links
func CleanupExpiredLinks() error {
	if shareLinkManager == nil {
		return ErrInternalError
	}

	shareLinkManager.mu.Lock()
	defer shareLinkManager.mu.Unlock()

	now := time.Now().Unix()
	filtered := make([]model.ShareLink, 0)

	for _, link := range shareLinkManager.store.Links {
		// Keep non-expired links and links with no expiration
		if link.ExpiresAt == 0 || now <= link.ExpiresAt {
			filtered = append(filtered, link)
		}
	}

	if len(filtered) != len(shareLinkManager.store.Links) {
		shareLinkManager.store.Links = filtered
		return shareLinkManager.saveUnlocked()
	}

	return nil
}
