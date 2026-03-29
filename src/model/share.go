package model

//go:generate msgp

// ShareLink represents a public share link for an object
type ShareLink struct {
	Token     string `msg:"token"`      // Unique random token
	Bucket    string `msg:"bucket"`     // Bucket name
	Key       string `msg:"key"`        // Object key
	CreatedAt int64  `msg:"created_at"` // Unix timestamp
	ExpiresAt int64  `msg:"expires_at"` // Unix timestamp (0 means no expiration)
}

// ShareLinkStore stores all share links
type ShareLinkStore struct {
	Version int64       `msg:"version"`
	Links   []ShareLink `msg:"links"`
}
