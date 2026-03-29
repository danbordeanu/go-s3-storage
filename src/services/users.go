package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"s3-storage/auth"
	"s3-storage/model"
)

const (
	usersFileName = ".users"
	bcryptCost    = 12
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrUserExists    = errors.New("user already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrBootstrapUser = errors.New("cannot modify bootstrap user")
)

// UserService manages user accounts
type UserService struct {
	storageDir    string
	users         map[string]*model.User // keyed by ID
	usersByName   map[string]*model.User // keyed by username
	bootstrapUser *model.User
	credStore     *auth.MemoryStore
	mu            sync.RWMutex
}

// NewUserService creates a new user service
func NewUserService(storageDir string, bootstrapUser *model.User, credStore *auth.MemoryStore) (*UserService, error) {
	svc := &UserService{
		storageDir:    storageDir,
		users:         make(map[string]*model.User),
		usersByName:   make(map[string]*model.User),
		bootstrapUser: bootstrapUser,
		credStore:     credStore,
	}

	if err := svc.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Add bootstrap user to lookup maps (but not to persistent storage)
	if bootstrapUser != nil {
		svc.usersByName[bootstrapUser.Username] = bootstrapUser
	}

	return svc, nil
}

// GetByUsername looks up a user by username
func (s *UserService) GetByUsername(username string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.usersByName[username]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// GetByID looks up a user by ID
func (s *UserService) GetByID(id string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check bootstrap user
	if s.bootstrapUser != nil && s.bootstrapUser.ID == id {
		return s.bootstrapUser, nil
	}

	user, exists := s.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// List returns all users
func (s *UserService) List() []*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*model.User, 0, len(s.users)+1)

	// Add bootstrap user first
	if s.bootstrapUser != nil {
		users = append(users, s.bootstrapUser)
	}

	for _, user := range s.users {
		users = append(users, user)
	}

	return users
}

// Create creates a new user
func (s *UserService) Create(username, password, displayName string, roles []string) (*model.User, error) {
	if username == "" || password == "" {
		return nil, ErrInvalidInput
	}

	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if username exists
	if _, exists := s.usersByName[username]; exists {
		return nil, ErrUserExists
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	user := &model.User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  displayName,
		Roles:        roles,
		Provider:     "local",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.users[user.ID] = user
	s.usersByName[user.Username] = user

	if err := s.save(); err != nil {
		// Rollback
		delete(s.users, user.ID)
		delete(s.usersByName, user.Username)
		return nil, err
	}

	return user, nil
}

// Update updates a user
func (s *UserService) Update(id string, displayName string, roles []string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cannot update bootstrap user
	if s.bootstrapUser != nil && s.bootstrapUser.ID == id {
		return nil, ErrBootstrapUser
	}

	user, exists := s.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}

	user.DisplayName = displayName
	user.Roles = roles
	user.UpdatedAt = time.Now().Unix()

	if err := s.save(); err != nil {
		return nil, err
	}

	return user, nil
}

// UpdatePassword changes a user's password
func (s *UserService) UpdatePassword(id, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Cannot update bootstrap user password
	if s.bootstrapUser != nil && s.bootstrapUser.ID == id {
		return ErrBootstrapUser
	}

	user, exists := s.users[id]
	if !exists {
		return ErrUserNotFound
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hash)
	user.UpdatedAt = time.Now().Unix()

	return s.save()
}

// Delete removes a user
func (s *UserService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cannot delete bootstrap user
	if s.bootstrapUser != nil && s.bootstrapUser.ID == id {
		return ErrBootstrapUser
	}

	user, exists := s.users[id]
	if !exists {
		return ErrUserNotFound
	}

	// Remove S3 credentials from credential store
	if user.S3AccessKeyID != "" && s.credStore != nil {
		s.credStore.RemoveUserCredential(user.S3AccessKeyID)
	}

	delete(s.users, id)
	delete(s.usersByName, user.Username)

	return s.save()
}

// load reads users from disk
func (s *UserService) load() error {
	filePath := filepath.Join(s.storageDir, usersFileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var store model.UserStore
	if err := json.Unmarshal(data, &store); err != nil {
		return err
	}

	for i := range store.Users {
		persistent := &store.Users[i]
		// Convert from persistent format to runtime User
		user := &model.User{
			ID:                persistent.ID,
			Username:          persistent.Username,
			PasswordHash:      persistent.PasswordHash,
			DisplayName:       persistent.DisplayName,
			Roles:             persistent.Roles,
			Provider:          persistent.Provider,
			ExternalID:        persistent.ExternalID,
			IsBootstrap:       persistent.IsBootstrap,
			CreatedAt:         persistent.CreatedAt,
			UpdatedAt:         persistent.UpdatedAt,
			BucketPermissions: persistent.BucketPermissions,
			S3AccessKeyID:     persistent.S3AccessKeyID,
			S3SecretAccessKey: persistent.S3SecretAccessKey,
		}
		s.users[user.ID] = user
		s.usersByName[user.Username] = user
	}

	// Sync all credentials to the credential store
	if err := s.SyncCredentialsToStore(); err != nil {
		return err
	}

	return nil
}

// save writes users to disk
func (s *UserService) save() error {
	store := model.UserStore{
		Version: 1,
		Users:   make([]model.UserPersistent, 0, len(s.users)),
	}

	for _, user := range s.users {
		// Convert from runtime User to persistent format
		persistent := model.UserPersistent{
			ID:                user.ID,
			Username:          user.Username,
			PasswordHash:      user.PasswordHash,
			DisplayName:       user.DisplayName,
			Roles:             user.Roles,
			Provider:          user.Provider,
			ExternalID:        user.ExternalID,
			IsBootstrap:       user.IsBootstrap,
			CreatedAt:         user.CreatedAt,
			UpdatedAt:         user.UpdatedAt,
			BucketPermissions: user.BucketPermissions,
			S3AccessKeyID:     user.S3AccessKeyID,
			S3SecretAccessKey: user.S3SecretAccessKey,
		}
		store.Users = append(store.Users, persistent)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(s.storageDir, usersFileName)
	tmpPath := filePath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, filePath)
}

// Count returns the total number of users
func (s *UserService) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := len(s.users)
	if s.bootstrapUser != nil {
		count++
	}
	return count
}

// SetBucketPermission sets or updates bucket permissions for a user
func (s *UserService) SetBucketPermission(userID, bucketName string, canRead, canWrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cannot set permissions for bootstrap user
	if s.bootstrapUser != nil && s.bootstrapUser.ID == userID {
		return ErrBootstrapUser
	}

	user, exists := s.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	// Find existing permission or create new one
	found := false
	for i := range user.BucketPermissions {
		if user.BucketPermissions[i].BucketName == bucketName {
			if !canRead && !canWrite {
				// Remove permission if both are false
				user.BucketPermissions = append(user.BucketPermissions[:i], user.BucketPermissions[i+1:]...)
			} else {
				user.BucketPermissions[i].CanRead = canRead
				user.BucketPermissions[i].CanWrite = canWrite
			}
			found = true
			break
		}
	}

	if !found && (canRead || canWrite) {
		user.BucketPermissions = append(user.BucketPermissions, model.BucketPermission{
			BucketName: bucketName,
			CanRead:    canRead,
			CanWrite:   canWrite,
		})
	}

	user.UpdatedAt = time.Now().Unix()
	return s.save()
}

// RemoveBucketPermissions removes all permissions for a bucket from all users
// This should be called when a bucket is deleted
func (s *UserService) RemoveBucketPermissions(bucketName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	modified := false
	for _, user := range s.users {
		for i := len(user.BucketPermissions) - 1; i >= 0; i-- {
			if user.BucketPermissions[i].BucketName == bucketName {
				user.BucketPermissions = append(user.BucketPermissions[:i], user.BucketPermissions[i+1:]...)
				modified = true
			}
		}
	}

	if modified {
		return s.save()
	}
	return nil
}

// GetUsersWithBucketAccess returns all non-admin users with their bucket permissions
func (s *UserService) GetUsersWithBucketAccess(bucketName string) []*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*model.User, 0)
	for _, user := range s.users {
		// Skip admin users
		if user.IsAdmin() {
			continue
		}
		users = append(users, user)
	}
	return users
}

// ListNonAdminUsers returns all non-admin users
func (s *UserService) ListNonAdminUsers() []*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*model.User, 0)
	for _, user := range s.users {
		if !user.IsAdmin() {
			users = append(users, user)
		}
	}
	return users
}

// VerifyPassword checks if the provided password matches the user's stored password hash
func (s *UserService) VerifyPassword(id, password string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Bootstrap users cannot have their passwords verified through this method
	if s.bootstrapUser != nil && s.bootstrapUser.ID == id {
		return false
	}

	user, exists := s.users[id]
	if !exists {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// SetS3Credentials sets S3 credentials for a user
func (s *UserService) SetS3Credentials(userID, accessKeyID, secretAccessKey string) error {
	if accessKeyID == "" || secretAccessKey == "" {
		return ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Cannot set credentials for bootstrap user
	if s.bootstrapUser != nil && s.bootstrapUser.ID == userID {
		return ErrBootstrapUser
	}

	user, exists := s.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	// Check if access key ID is already in use by another user
	if s.credStore != nil {
		if existingUserID, err := s.credStore.GetUserIDByAccessKey(accessKeyID); err == nil {
			if existingUserID != userID {
				return fmt.Errorf("access key ID already in use by another user")
			}
		}
	}

	// Remove old credentials from store if they exist
	if user.S3AccessKeyID != "" && user.S3AccessKeyID != accessKeyID && s.credStore != nil {
		s.credStore.RemoveUserCredential(user.S3AccessKeyID)
	}

	// Update user credentials
	user.S3AccessKeyID = accessKeyID
	user.S3SecretAccessKey = secretAccessKey
	user.UpdatedAt = time.Now().Unix()

	// Add to credential store
	if s.credStore != nil {
		s.credStore.AddUserCredential(userID, accessKeyID, secretAccessKey)
	}

	return s.save()
}

// RemoveS3Credentials removes S3 credentials for a user
func (s *UserService) RemoveS3Credentials(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cannot modify bootstrap user
	if s.bootstrapUser != nil && s.bootstrapUser.ID == userID {
		return ErrBootstrapUser
	}

	user, exists := s.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	// Remove from credential store
	if user.S3AccessKeyID != "" && s.credStore != nil {
		s.credStore.RemoveUserCredential(user.S3AccessKeyID)
	}

	// Clear credentials
	user.S3AccessKeyID = ""
	user.S3SecretAccessKey = ""
	user.UpdatedAt = time.Now().Unix()

	return s.save()
}

// SyncCredentialsToStore loads all user credentials into the credential store
// This is called on startup after loading users from disk
func (s *UserService) SyncCredentialsToStore() error {
	if s.credStore == nil {
		return nil
	}

	// This is called within load() which already holds the lock
	// So we don't need to lock here

	for _, user := range s.users {
		if user.S3AccessKeyID != "" && user.S3SecretAccessKey != "" {
			s.credStore.AddUserCredential(user.ID, user.S3AccessKeyID, user.S3SecretAccessKey)
		}
	}

	return nil
}
