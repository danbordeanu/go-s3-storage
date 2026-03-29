package services

import (
	"os"
	"path/filepath"
	"regexp"
	"s3-storage/configuration"
	"strings"

	"s3-storage/model"
)

var (
	metaStore  *MetaStore
	storageDir string
)

// InitBucketService initializes the bucket service with dependencies
func InitBucketService(ms *MetaStore, sd string) {
	metaStore = ms
	storageDir = sd
}

// GetMetaStore returns the global metaStore instance
func GetMetaStore() *MetaStore {
	return metaStore
}

// bucketNameRegex validates bucket names according to S3 rules
var bucketNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
var ipAddressRegex = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)

// ValidateBucketName validates a bucket name according to S3 rules
// S3 bucket naming rules (simplified):
// - Must be between 3 and 63 characters long
// - Must be a series of one or more labels separated by periods (.)
// - Each label must start and end with a lowercase letter or number
// - Labels can contain lowercase letters, numbers, and hyphens (-)
// - Must not be formatted as an IP address (e.g.,
// Parameters: - name: the bucket name to validate
// Returns an error if the bucket name is invalid, nil otherwise
// Example usage:
//
//	if err := ValidateBucketName("my.bucket-name"); err != nil {
//		// Handle invalid bucket name
//	}
func ValidateBucketName(name string) error {
	// Length check: 3-63 characters
	if len(name) < 3 || len(name) > 63 {
		return ErrInvalidBucketName
	}

	// Must not be formatted as an IP address
	if ipAddressRegex.MatchString(name) {
		return ErrInvalidBucketName
	}

	// Must match pattern: lowercase letters, numbers, hyphens, periods
	if !bucketNameRegex.MatchString(name) {
		return ErrInvalidBucketName
	}

	// Must not contain consecutive periods
	if strings.Contains(name, "..") {
		return ErrInvalidBucketName
	}

	// Must not contain period adjacent to hyphen
	if strings.Contains(name, ".-") || strings.Contains(name, "-.") {
		return ErrInvalidBucketName
	}

	return nil
}

// ListBuckets returns all buckets
// Admins get all buckets, non-admins get buckets they own + buckets they have read access to (filtering is done in handlers)
// Returns a list of BucketMeta objects representing the buckets
// Parameters: none
// Example usage:
//
//	buckets := ListBuckets()
func ListBuckets() []model.BucketMeta {
	return metaStore.GetBuckets()
}

// CreateBucket creates a new bucket
// Validates bucket name, creates metadata entry, and creates bucket directory
// Parameters:
// - name: the name of the bucket to create
// Returns an error if the bucket name is invalid or if there was a problem creating the bucket
// Example usage:
//
//	if err := CreateBucket("my-new-bucket"); err != nil {
//		// Handle error creating bucket
//	}
func CreateBucket(name string) error {
	// Validate bucket name
	if err := ValidateBucketName(name); err != nil {
		return err
	}

	// Create bucket in metadata
	if err := metaStore.CreateBucket(name); err != nil {
		return err
	}

	// Create bucket directory
	bucketPath := filepath.Join(storageDir, name)
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		// Rollback metadata on failure
		metaStore.DeleteBucket(name)
		return err
	}

	return nil
}

// DeleteBucket deletes a bucket (must be empty)
// Checks if bucket exists, checks if bucket directory is empty, deletes metadata, and deletes bucket directory
// Parameters:
// - name: the name of the bucket to delete
// Returns an error if the bucket does not exist, if the bucket is not empty, or if there was a problem deleting the bucket
// Example usage:
//
//	if err := DeleteBucket("my-old-bucket"); err != nil {
//		// Handle error deleting bucket
//	}
func DeleteBucket(name string) error {
	// Check if bucket exists
	if !metaStore.BucketExists(name) {
		return ErrNoSuchBucket
	}

	// Check if bucket directory is empty
	bucketPath := filepath.Join(storageDir, name)
	entries, err := os.ReadDir(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist, just remove from metadata
			return metaStore.DeleteBucket(name)
		}
		return err
	}

	if len(entries) > 0 {
		return ErrBucketNotEmpty
	}

	// Delete metadata first
	if err = metaStore.DeleteBucket(name); err != nil {
		return err
	}

	// Delete bucket directory
	if err = os.Remove(bucketPath); err != nil && !os.IsNotExist(err) {
		// Recreate metadata on failure (best effort rollback)
		metaStore.CreateBucket(name)
		return err
	}

	return nil
}

// HeadBucket checks if a bucket exists
// Parameters:
// - name: the name of the bucket to check
// Returns an error if the bucket does not exist, nil otherwise
// Example usage:
//
//	if err := HeadBucket("my-bucket"); err != nil {
//		// Handle bucket not existing
//	}
func HeadBucket(name string) error {
	if !metaStore.BucketExists(name) {
		return ErrNoSuchBucket
	}
	return nil
}

// GetBucket returns bucket metadata
// Parameters:
// - name: the name of the bucket to retrieve
// Returns a BucketMeta object representing the bucket, or an error if the bucket does not exist
// Example usage:
//
//	bucketMeta, err := GetBucket("my-bucket")
//	if err != nil {
//		// Handle bucket not existing
//	}
func GetBucket(name string) (*model.BucketMeta, error) {
	return metaStore.GetBucket(name)
}

// CreateBucketWithOwner creates a new bucket with the specified owner
// Validates bucket name, creates metadata entry with owner, and creates bucket directory
// Parameters:
// - name: the name of the bucket to create
// - ownerID: the ID of the user who will own the bucket
// Returns an error if the bucket name is invalid, if there was a problem creating the bucket, or if the owner does not exist
// Example usage:
//
//	if err := CreateBucketWithOwner("my-new-bucket", currentUser.ID); err != nil {
//		// Handle error creating bucket
//	}
func CreateBucketWithOwner(name, ownerID string) error {
	// Validate bucket name
	if err := ValidateBucketName(name); err != nil {
		return err
	}

	// Create bucket in metadata with owner
	if err := metaStore.CreateBucketWithOwner(name, ownerID); err != nil {
		return err
	}

	// Create bucket directory
	bucketPath := filepath.Join(storageDir, name)
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		// Rollback metadata on failure
		metaStore.DeleteBucket(name)
		return err
	}

	return nil
}

// ForceDeleteBucket deletes a bucket and all its contents (admin only)
// Checks if bucket exists, deletes metadata, and recursively deletes bucket directory and all contents
// Parameters:
// - name: the name of the bucket to delete
// Returns an error if the bucket does not exist or if there was a problem deleting the bucket
// Example usage (admin only):
//
//	if err := ForceDeleteBucket("my-bucket"); err != nil {
//		// Handle error deleting bucket
//	}
func ForceDeleteBucket(name string) error {
	// Check if bucket exists
	if !metaStore.BucketExists(name) {
		return ErrNoSuchBucket
	}

	bucketPath := filepath.Join(storageDir, name)

	// Delete metadata first
	if err := metaStore.DeleteBucket(name); err != nil {
		return err
	}

	// Recursively delete bucket directory and all contents
	if err := os.RemoveAll(bucketPath); err != nil && !os.IsNotExist(err) {
		// Best effort - metadata is already deleted
		return nil
	}

	return nil
}

// CheckStorageQuota checks if uploading a file of the given size would exceed the configured storage quota
// Parameters:
// - uploadSize: the size of the file to be uploaded, in bytes
// Returns an error if the upload would exceed the storage quota, nil otherwise
// Example usage:
//
//	if err := CheckStorageQuota(fileSize); err != nil {
//		// Handle quota exceeded error
//	}
func CheckStorageQuota(uploadSize int64) error {
	// Get current total storage used
	conf := configuration.AppConfig()
	if conf.StorageQuotaBytes <= 0 {
		// No quota set, allow all uploads
		return nil // No quota set
	}
	// Get current total storage used
	currentUsage := metaStore.GetTotalStorageSize()

	// Check if new upload would exceed quota
	if currentUsage+uploadSize > conf.StorageQuotaBytes {
		return ErrQuotaExceeded
	}
	return nil
}
