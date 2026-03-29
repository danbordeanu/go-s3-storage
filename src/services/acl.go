package services

import (
	"s3-storage/model"
)

// CanAccessBucket checks if a user can access a bucket with the specified permission level.
// Admins always have full access.
// Bucket owners always have full access to their own buckets.
// Other users need explicit permissions.
// Empty owner (system buckets) = admin only.
// Parameters:
// - user: the user to check access for
// - bucketName: the name of the bucket to check
// - requireWrite: if true, checks for write permission; if false, checks for read permission
// - metaStore: the MetaStore to retrieve bucket ownership information
// Returns true if the user has the required access, false otherwise.
// Example usage:
//
//	if CanAccessBucket(currentUser, "my-bucket", true, metaStore) {
//		// User can write to the bucket
//	} else {
//		// User does not have write access to the bucket
//	}
func CanAccessBucket(user *model.User, bucketName string, requireWrite bool, metaStore *MetaStore) bool {
	// Admins always have access
	if user.IsAdmin() {
		return true
	}

	// Get bucket owner
	owner, err := metaStore.GetBucketOwner(bucketName)
	if err != nil {
		return false
	}

	// Empty owner means system bucket - admin only
	if owner == "" {
		return false
	}

	// Owner always has full access
	if owner == user.ID {
		return true
	}

	// Check explicit permissions
	return user.CanAccessBucket(bucketName, requireWrite)
}

// FilterBucketsForUser filters a list of buckets to only those the user can access.
// Admins see all buckets.
// Non-admins see: buckets they own + buckets they have read permission for.
// Parameters:
// - user: the user to filter buckets for
// - buckets: the list of buckets to filter
// Returns a filtered list of buckets the user can access.
// Example usage:
//
//	accessibleBuckets := FilterBucketsForUser(currentUser, allBuckets)
func FilterBucketsForUser(user *model.User, buckets []model.BucketMeta) []model.BucketMeta {
	// Admins see all buckets
	if user.IsAdmin() {
		return buckets
	}

	filtered := make([]model.BucketMeta, 0)
	for _, bucket := range buckets {
		// User owns the bucket
		if bucket.Owner == user.ID {
			filtered = append(filtered, bucket)
			continue
		}

		// User has explicit read permission
		if user.CanAccessBucket(bucket.Name, false) {
			filtered = append(filtered, bucket)
		}
	}

	return filtered
}

// BucketAccessInfo contains access information for a bucket
type BucketAccessInfo struct {
	IsOwner  bool
	CanRead  bool
	CanWrite bool
}

// GetBucketAccessInfo returns access information for a user on a specific bucket
// Admins have full access. Owners have full access. Other users may have explicit permissions.
// Parameters:
// - user: the user to check access for
// - bucket: the bucket to check access on
// Returns a BucketAccessInfo struct with the access details.
// Example usage:
//
//	accessInfo := GetBucketAccessInfo(currentUser, bucket)
//	if accessInfo.CanWrite {
//		// User can write to the bucket
//	} else if accessInfo.CanRead {
//		// User can read from the bucket but not write
//	} else {
//		// User has no access to the bucket
//	}
func GetBucketAccessInfo(user *model.User, bucket model.BucketMeta) BucketAccessInfo {
	// Admins have full access
	if user.IsAdmin() {
		return BucketAccessInfo{
			IsOwner:  false,
			CanRead:  true,
			CanWrite: true,
		}
	}

	// Owner has full access
	if bucket.Owner == user.ID {
		return BucketAccessInfo{
			IsOwner:  true,
			CanRead:  true,
			CanWrite: true,
		}
	}

	// Check explicit permissions
	perm := user.GetBucketPermission(bucket.Name)
	if perm != nil {
		return BucketAccessInfo{
			IsOwner:  false,
			CanRead:  perm.CanRead,
			CanWrite: perm.CanWrite,
		}
	}

	return BucketAccessInfo{
		IsOwner:  false,
		CanRead:  false,
		CanWrite: false,
	}
}
