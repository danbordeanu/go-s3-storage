package model

//go:generate msgp

// ObjectMeta is the structure stored in the xl.meta file
type ObjectMeta struct {
	Version      int    `msg:"version"`       // xl.meta format version
	Size         int64  `msg:"size"`          // Object size in bytes
	ETag         string `msg:"etag"`          // SHA256 hash (hex)
	LastModified int64  `msg:"last_modified"` // Unix timestamp
	ContentType  string `msg:"content_type"`  // MIME type from Magika
	DiskUUID     string `msg:"disk_uuid"`     // UUID of the disk directory
	Parts        []Part `msg:"parts"`         // Part list (single part.1)
}

// Part represents a part of an object
type Part struct {
	Number int    `msg:"number"` // Always 1 for simple upload
	Size   int64  `msg:"size"`   // Part size
	ETag   string `msg:"etag"`   // Part ETag (same as object for single part)
}
