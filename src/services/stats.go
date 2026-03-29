package services

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"s3-storage/configuration"
	"s3-storage/model"
)

// BucketStats holds statistics for a single bucket
type BucketStats struct {
	Name         string `json:"name"`
	ObjectCount  int64  `json:"object_count"`
	TotalSize    int64  `json:"total_size"`
	CreationDate int64  `json:"creation_date"`
}

// ContentTypeStat holds statistics for a content type
type ContentTypeStat struct {
	ContentType string `json:"content_type"`
	Label       string `json:"label"` // Human-readable label
	Count       int64  `json:"count"`
	TotalSize   int64  `json:"total_size"`
}

// StorageStats holds overall storage statistics
type StorageStats struct {
	BucketCount    int           `json:"bucket_count"`
	ObjectCount    int64         `json:"object_count"`
	TotalSize      int64         `json:"total_size"`
	ShareCount     int           `json:"share_count"`
	Buckets        []BucketStats `json:"buckets"`
	QuotaBytes     int64         `json:"quota_bytes"`     // Storage quota in bytes (0 = unlimited)
	QuotaRemaining int64         `json:"quota_remaining"` // Remaining quota in bytes
}

// StatsService provides storage statistics
type StatsService struct {
	metaStore *MetaStore
}

// NewStatsService creates a new stats service
// Takes a MetaStore to access bucket metadata
// Parameters: metaStore - the MetaStore instance to use for accessing bucket metadata
// Returns: a new instance of StatsService
// Example usage:
//
//	metaStore := NewMetaStore()
//	statsService := NewStatsService(metaStore)
func NewStatsService(metaStore *MetaStore) *StatsService {
	return &StatsService{
		metaStore: metaStore,
	}
}

// GetStats returns overall storage statistics (admin only - shows all buckets)
// This method retrieves overall storage statistics, including the total number of buckets, total object count, total storage size, and the count of active share links. It also includes quota information if a storage quota is configured. This method is intended for admin users and will show statistics for all buckets in the system.
// Returns: StorageStats containing overall storage statistics
// Example usage:
//
//	stats := statsService.GetStats()
//	fmt.Printf("Total Buckets: %d, Total Objects: %d, Total Size: %d bytes", stats.BucketCount, stats.ObjectCount, stats.TotalSize)
func (s *StatsService) GetStats() StorageStats {
	buckets := s.metaStore.GetBuckets()
	shareCount := getActiveShareCount()

	stats := StorageStats{
		BucketCount: len(buckets),
		ShareCount:  shareCount,
		Buckets:     make([]BucketStats, 0, len(buckets)),
	}

	for _, bucket := range buckets {
		bucketStats := BucketStats{
			Name:         bucket.Name,
			ObjectCount:  bucket.ObjectCount,
			TotalSize:    bucket.TotalSize,
			CreationDate: bucket.CreationDate,
		}
		stats.Buckets = append(stats.Buckets, bucketStats)
		stats.ObjectCount += bucket.ObjectCount
		stats.TotalSize += bucket.TotalSize
	}

	// Add quota information
	conf := configuration.AppConfig()
	stats.QuotaBytes = conf.StorageQuotaBytes
	if stats.QuotaBytes > 0 {
		stats.QuotaRemaining = stats.QuotaBytes - stats.TotalSize
		if stats.QuotaRemaining < 0 {
			stats.QuotaRemaining = 0
		}
	}

	return stats
}

// GetStatsForUser returns storage statistics filtered by user permissions
// This method retrieves storage statistics for a specific user, filtering the buckets based on the user's permissions. Admin users will see statistics for all buckets, while non-admin users will only see statistics for buckets they own or have access to. The method also includes the count of active share links that the user can access and quota information if a storage quota is configured.
// Parameters: user - the User for whom to retrieve statistics
// Returns: StorageStats containing storage statistics filtered by user permissions
// Example usage:
//
//	user := authProvider.Authenticate(ctx, credentials)
//	stats := statsService.GetStatsForUser(user)
//	fmt.Printf("Buckets Accessible to User: %d, Total Objects: %d, Total Size: %d bytes", stats.BucketCount, stats.ObjectCount, stats.TotalSize)
func (s *StatsService) GetStatsForUser(user *model.User) StorageStats {
	// Admins see all stats
	if user.IsAdmin() {
		return s.GetStats()
	}

	allBuckets := s.metaStore.GetBuckets()
	filteredBuckets := FilterBucketsForUser(user, allBuckets)
	shareCount := getActiveShareCountForUser(user)

	stats := StorageStats{
		BucketCount: len(filteredBuckets),
		ShareCount:  shareCount,
		Buckets:     make([]BucketStats, 0, len(filteredBuckets)),
	}

	for _, bucket := range filteredBuckets {
		bucketStats := BucketStats{
			Name:         bucket.Name,
			ObjectCount:  bucket.ObjectCount,
			TotalSize:    bucket.TotalSize,
			CreationDate: bucket.CreationDate,
		}
		stats.Buckets = append(stats.Buckets, bucketStats)
		stats.ObjectCount += bucket.ObjectCount
		stats.TotalSize += bucket.TotalSize
	}

	// Add quota information (quota is system-wide, but we show total storage usage)
	conf := configuration.AppConfig()
	stats.QuotaBytes = conf.StorageQuotaBytes
	if stats.QuotaBytes > 0 {
		// For non-admin users, we still show the global quota and remaining space
		totalSystemUsage := s.metaStore.GetTotalStorageSize()
		stats.QuotaRemaining = stats.QuotaBytes - totalSystemUsage
		if stats.QuotaRemaining < 0 {
			stats.QuotaRemaining = 0
		}
	}

	return stats
}

// GetBucketStats returns statistics for a specific bucket
func (s *StatsService) GetBucketStats(bucketName string) (*BucketStats, error) {
	buckets := s.metaStore.GetBuckets()

	for _, bucket := range buckets {
		if bucket.Name == bucketName {
			return &BucketStats{
				Name:         bucket.Name,
				ObjectCount:  bucket.ObjectCount,
				TotalSize:    bucket.TotalSize,
				CreationDate: bucket.CreationDate,
			}, nil
		}
	}

	return nil, ErrNoSuchBucket
}

// GetContentTypeStats returns statistics grouped by content type (admin only - all buckets)
func (s *StatsService) GetContentTypeStats() []ContentTypeStat {
	buckets := s.metaStore.GetBuckets()
	return s.computeContentTypeStats(buckets)
}

// GetContentTypeStatsForUser returns statistics grouped by content type for buckets the user can access
func (s *StatsService) GetContentTypeStatsForUser(user *model.User) []ContentTypeStat {
	// Admins see all stats
	if user.IsAdmin() {
		return s.GetContentTypeStats()
	}

	allBuckets := s.metaStore.GetBuckets()
	filteredBuckets := FilterBucketsForUser(user, allBuckets)
	return s.computeContentTypeStats(filteredBuckets)
}

// computeContentTypeStats computes content type stats for a list of buckets
func (s *StatsService) computeContentTypeStats(buckets []model.BucketMeta) []ContentTypeStat {
	contentTypeMap := make(map[string]*ContentTypeStat)

	// Walk all buckets and collect content type stats
	for _, bucket := range buckets {
		bucketPath := filepath.Join(storageDir, bucket.Name)

		filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// Only process xl.meta files
			if info.IsDir() || info.Name() != "xl.meta" {
				return nil
			}

			// Read object metadata
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var meta model.ObjectMeta
			if _, err := meta.UnmarshalMsg(data); err != nil {
				return nil
			}

			// Get the main content type category
			contentType := meta.ContentType
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			// Create human-readable label from content type
			label := getContentTypeLabel(contentType)

			if stat, exists := contentTypeMap[label]; exists {
				stat.Count++
				stat.TotalSize += meta.Size
			} else {
				contentTypeMap[label] = &ContentTypeStat{
					ContentType: contentType,
					Label:       label,
					Count:       1,
					TotalSize:   meta.Size,
				}
			}

			return nil
		})
	}

	// Convert map to slice
	stats := make([]ContentTypeStat, 0, len(contentTypeMap))
	for _, stat := range contentTypeMap {
		stats = append(stats, *stat)
	}

	return stats
}

// getContentTypeLabel returns a human-readable label for a content type
func getContentTypeLabel(contentType string) string {
	// Map common MIME types to friendly labels
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "Images"
	case strings.HasPrefix(contentType, "video/"):
		return "Videos"
	case strings.HasPrefix(contentType, "audio/"):
		return "Audio"
	case strings.HasPrefix(contentType, "text/"):
		return "Text"
	case strings.Contains(contentType, "pdf"):
		return "PDF"
	case strings.Contains(contentType, "zip") || strings.Contains(contentType, "tar") ||
		strings.Contains(contentType, "gzip") || strings.Contains(contentType, "compress"):
		return "Archives"
	case strings.Contains(contentType, "json"):
		return "JSON"
	case strings.Contains(contentType, "xml"):
		return "XML"
	case strings.Contains(contentType, "javascript") || strings.Contains(contentType, "ecmascript"):
		return "JavaScript"
	case strings.Contains(contentType, "html"):
		return "HTML"
	case strings.Contains(contentType, "css"):
		return "CSS"
	case strings.Contains(contentType, "word") || strings.Contains(contentType, "document"):
		return "Documents"
	case strings.Contains(contentType, "spreadsheet") || strings.Contains(contentType, "excel"):
		return "Spreadsheets"
	case strings.Contains(contentType, "presentation") || strings.Contains(contentType, "powerpoint"):
		return "Presentations"
	case contentType == "application/octet-stream":
		return "Binary"
	default:
		return "Other"
	}
}

// getActiveShareCount returns the count of non-expired share links
// This function iterates through all share links in the system and counts how many are currently active (i.e., not expired). It checks the expiration time of each share link against the current time and only counts those that are still valid. This provides an overall count of active share links across all buckets.
// Returns: the count of active share links in the system
func getActiveShareCount() int {
	if shareLinkManager == nil {
		return 0
	}

	shareLinkManager.mu.RLock()
	defer shareLinkManager.mu.RUnlock()

	now := time.Now().Unix()
	count := 0
	for _, link := range shareLinkManager.store.Links {
		// Count non-expired links
		if link.ExpiresAt == 0 || now <= link.ExpiresAt {
			count++
		}
	}
	return count
}

// getActiveShareCountForUser returns the count of non-expired share links for buckets the user can access
// This function counts the number of active (non-expired) share links that are associated with buckets the specified user can access. It iterates through all share links and checks if each link is still valid (not expired) and if the user has permission to access the bucket associated with the share link. This provides a count of active share links that are relevant to the user's accessible buckets.
// Parameters: user - the User for whom to count active share links
// Returns: the count of active share links that the user can access
func getActiveShareCountForUser(user *model.User) int {
	if shareLinkManager == nil {
		return 0
	}

	shareLinkManager.mu.RLock()
	defer shareLinkManager.mu.RUnlock()

	now := time.Now().Unix()
	count := 0
	for _, link := range shareLinkManager.store.Links {
		// Count non-expired links for buckets user can access
		if link.ExpiresAt == 0 || now <= link.ExpiresAt {
			// Check if user can access this bucket
			bucket, err := GetBucket(link.Bucket)
			if err != nil {
				continue
			}
			// User owns the bucket or has read permission
			if bucket.Owner == user.ID || user.CanAccessBucket(link.Bucket, false) {
				count++
			}
		}
	}
	return count
}

// GetAllShareLinks returns all share links
// This function retrieves all share links from the share link manager. It returns a slice of ShareLink objects representing all the share links currently stored in the system, regardless of their expiration status or associated buckets. This can be used for administrative purposes to view and manage all share links in the system.
// Parameters: none
// Returns: a slice of ShareLink objects representing all share links in the system
// Example usage:
//
//	shareLinks := statsService.GetAllShareLinks()
//	fmt.Printf("Total Share Links: %d", len(shareLinks))
func GetAllShareLinks() []model.ShareLink {
	if shareLinkManager == nil {
		return nil
	}

	shareLinkManager.mu.RLock()
	defer shareLinkManager.mu.RUnlock()

	// Return a copy of all links
	links := make([]model.ShareLink, len(shareLinkManager.store.Links))
	copy(links, shareLinkManager.store.Links)
	return links
}
