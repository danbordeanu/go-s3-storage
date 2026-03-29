package model

//go:generate msgp

// MetaData is the root structure stored in the .meta file
type MetaData struct {
	Version    int64         `msg:"version"`
	DiskUUID   string        `msg:"disk_uuid"`  // Unique identifier for this storage disk
	UpdatedAt  int64         `msg:"updated_at"` // Unix timestamp
	Buckets    []BucketMeta  `msg:"buckets"`
	Multiparts []Multipart   `msg:"multiparts"` // placeholder for future use
	Healing    []HealingLock `msg:"healing"`    // placeholder for future use
}

// BucketMeta stores bucket metadata
type BucketMeta struct {
	Name         string `msg:"name"`
	CreationDate int64  `msg:"creation_date"` // Unix timestamp
	TotalSize    int64  `msg:"total_size"`    // Sum of all object sizes in bytes
	ObjectCount  int64  `msg:"object_count"`  // Number of objects in bucket
	Owner        string `msg:"owner"`         // User ID of bucket creator (empty = system bucket, admin only)
}

// Multipart represents an in-progress multipart upload (placeholder)
type Multipart struct {
	UploadID  string `msg:"upload_id"`
	Bucket    string `msg:"bucket"`
	Key       string `msg:"key"`
	Initiated int64  `msg:"initiated"`
}

// HealingLock represents a healing operation lock (placeholder)
type HealingLock struct {
	ID         string `msg:"id"`
	Path       string `msg:"path"`
	AcquiredAt int64  `msg:"acquired_at"`
	ExpiresAt  int64  `msg:"expires_at"`
}
