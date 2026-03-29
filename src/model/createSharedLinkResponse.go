package model

// CreateShareLinkResponse represents the response for creating a share link
type CreateShareLinkResponse struct {
	Token     string `json:"token" xml:"Token"`
	ShareURL  string `json:"share_url" xml:"ShareURL"`
	Bucket    string `json:"bucket" xml:"Bucket"`
	Key       string `json:"key" xml:"Key"`
	ExpiresIn int64  `json:"expires_in" xml:"ExpiresIn"`
	ExpiresAt int64  `json:"expires_at,omitempty" xml:"ExpiresAt,omitempty"`
}
