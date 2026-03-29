package model

import "encoding/xml"

// Owner represents the bucket owner
// @Description Owner information for S3 buckets
type Owner struct {
	// Unique identifier of the owner
	ID string `xml:"ID" json:"id" example:"s3-storage"`
	// Display name of the owner
	DisplayName string `xml:"DisplayName" json:"displayName" example:"s3-storage"`
}

// ListAllMyBucketsResult is the XML response for listing buckets
// @Description Success response for ListBuckets operation containing owner info and bucket list
type ListAllMyBucketsResult struct {
	XMLName xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListAllMyBucketsResult" swaggerignore:"true"`
	// Owner of the buckets
	Owner Owner `xml:"Owner" json:"owner"`
	// Container for the list of buckets
	Buckets Buckets `xml:"Buckets" json:"buckets"`
}

// Buckets is a container for bucket list
// @Description Container for the list of buckets
type Buckets struct {
	// List of buckets
	Bucket []Bucket `xml:"Bucket" json:"bucket"`
}

// Bucket represents an S3 bucket in the response
// @Description Individual bucket information
type Bucket struct {
	// Name of the bucket
	Name string `xml:"Name" json:"name" example:"my-bucket"`
	// Date and time when the bucket was created in RFC3339 format
	CreationDate string `xml:"CreationDate" json:"creationDate" example:"2024-01-15T10:30:00Z"`
}

// ListBucketResult is the XML response for ListObjectsV2
// @Description Success response for ListObjects operation (S3 ListObjectsV2 compatible)
type ListBucketResult struct {
	XMLName xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult" swaggerignore:"true"`
	// Name of the bucket
	Name string `xml:"Name" json:"name" example:"my-bucket"`
	// Prefix used to filter objects
	Prefix string `xml:"Prefix" json:"prefix" example:"documents/"`
	// Delimiter used to group common prefixes
	Delimiter string `xml:"Delimiter,omitempty" json:"delimiter,omitempty" example:"/"`
	// Maximum number of keys returned
	MaxKeys int `xml:"MaxKeys" json:"maxKeys" example:"1000"`
	// Indicates if the response is truncated
	IsTruncated bool `xml:"IsTruncated" json:"isTruncated" example:"false"`
	// Number of keys returned
	KeyCount int `xml:"KeyCount" json:"keyCount" example:"10"`
	// List of objects
	Contents []ObjectContent `xml:"Contents" json:"contents"`
	// Common prefixes (virtual folders) when using delimiter
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes,omitempty" json:"commonPrefixes,omitempty"`
}

// ObjectContent represents an object in the ListBucketResult
// @Description Individual object information in list response
type ObjectContent struct {
	// Object key (path)
	Key string `xml:"Key" json:"key" example:"documents/file.txt"`
	// Last modification time in RFC3339 format
	LastModified string `xml:"LastModified" json:"lastModified" example:"2024-01-15T10:30:00Z"`
	// ETag (hash) of the object
	ETag string `xml:"ETag" json:"etag" example:"\"d41d8cd98f00b204e9800998ecf8427e\""`
	// Size in bytes
	Size int64 `xml:"Size" json:"size" example:"1024"`
	// Storage class
	StorageClass string `xml:"StorageClass" json:"storageClass" example:"STANDARD"`
}

// CommonPrefix represents a common prefix (virtual folder)
// @Description Common prefix representing a virtual folder
type CommonPrefix struct {
	// The prefix (folder path)
	Prefix string `xml:"Prefix" json:"prefix" example:"documents/"`
}