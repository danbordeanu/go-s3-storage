package model

//go:generate msgp

// BucketPermission represents read/write access to a specific bucket
type BucketPermission struct {
	BucketName string `msg:"bucket_name" json:"bucket_name"`
	CanRead    bool   `msg:"can_read" json:"can_read"`
	CanWrite   bool   `msg:"can_write" json:"can_write"`
}

// User represents a user account in the system
type User struct {
	ID                string             `msg:"id" json:"id"`
	Username          string             `msg:"username" json:"username"`
	PasswordHash      string             `msg:"password_hash" json:"-"` // bcrypt hash, never expose in JSON
	DisplayName       string             `msg:"display_name" json:"display_name"`
	Roles             []string           `msg:"roles" json:"roles"`          // "admin", "user"
	Provider          string             `msg:"provider" json:"provider"`    // "local", "config", "azuread"
	ExternalID        string             `msg:"external_id" json:"-"`        // For Azure AD
	IsBootstrap       bool               `msg:"is_bootstrap" json:"-"`       // true for env-var admin
	CreatedAt         int64              `msg:"created_at" json:"created_at"`
	UpdatedAt         int64              `msg:"updated_at" json:"updated_at"`
	BucketPermissions []BucketPermission `msg:"bucket_permissions" json:"bucket_permissions"`
	S3AccessKeyID     string             `msg:"s3_access_key_id" json:"s3_access_key_id"`
	S3SecretAccessKey string             `msg:"s3_secret_access_key" json:"s3_secret_access_key"`
}

// UserPersistent is the internal representation for disk storage (includes password hash)
type UserPersistent struct {
	ID                string             `json:"id"`
	Username          string             `json:"username"`
	PasswordHash      string             `json:"password_hash"` // Included for disk storage
	DisplayName       string             `json:"display_name"`
	Roles             []string           `json:"roles"`
	Provider          string             `json:"provider"`
	ExternalID        string             `json:"external_id"`
	IsBootstrap       bool               `json:"is_bootstrap"`
	CreatedAt         int64              `json:"created_at"`
	UpdatedAt         int64              `json:"updated_at"`
	BucketPermissions []BucketPermission `json:"bucket_permissions"`
	S3AccessKeyID     string             `json:"s3_access_key_id"`
	S3SecretAccessKey string             `json:"s3_secret_access_key"`
}

// UserStore stores all users (for disk persistence)
type UserStore struct {
	Version int64            `json:"version"`
	Users   []UserPersistent `json:"users"`
}

// HasRole checks if the user has a specific role
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin checks if the user has admin role
func (u *User) IsAdmin() bool {
	return u.HasRole("admin")
}

// GetBucketPermission returns the permission for a specific bucket, or nil if none exists
func (u *User) GetBucketPermission(bucketName string) *BucketPermission {
	for i := range u.BucketPermissions {
		if u.BucketPermissions[i].BucketName == bucketName {
			return &u.BucketPermissions[i]
		}
	}
	return nil
}

// CanAccessBucket checks if the user can access a bucket with the specified permission level
func (u *User) CanAccessBucket(bucketName string, requireWrite bool) bool {
	perm := u.GetBucketPermission(bucketName)
	if perm == nil {
		return false
	}
	if requireWrite {
		return perm.CanWrite
	}
	return perm.CanRead
}
