package model

import "encoding/xml"

// S3Error represents an S3-compatible error response
// @Description S3-compatible error response returned on failure
type S3Error struct {
	XMLName xml.Name `xml:"Error" swaggerignore:"true"`
	// Error code identifying the type of error
	Code string `xml:"Code" json:"code" example:"NoSuchBucket"`
	// Human-readable error message
	Message string `xml:"Message" json:"message" example:"The specified bucket does not exist"`
	// The resource associated with the error
	Resource string `xml:"Resource,omitempty" json:"resource,omitempty" example:"my-bucket"`
	// Unique request identifier for troubleshooting
	RequestID string `xml:"RequestId,omitempty" json:"requestId,omitempty" example:"4442587FB7D0A2F9"`
}
