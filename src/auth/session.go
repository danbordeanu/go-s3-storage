package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"s3-storage/model"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

// Session represents a user session
type Session struct {
	ID        string
	UserID    string
	User      *model.User
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// SessionStore manages user sessions in memory
type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	ttl      time.Duration
}

// NewSessionStore creates a new session store
// ttlSeconds specifies how long sessions should last in seconds
// Parameters: ttlSeconds - Time to live for sessions in seconds
// Returns: *SessionStore - A pointer to the newly created SessionStore
// Example usage:
//
//	store := NewSessionStore(3600) // Sessions last for 1 hour
func NewSessionStore(ttlSeconds int32) *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*Session),
		ttl:      time.Duration(ttlSeconds) * time.Second,
	}

	// Start cleanup goroutine
	go store.cleanupLoop()

	return store
}

// Create creates a new session for a user
// This function generates a unique session ID and CSRF token, sets the creation and expiration times, and stores the session in memory.
// Parameters: user - A pointer to the User for whom the session is being created
// Returns: *Session - A pointer to the newly created Session, error - An error if session creation fails
// Example usage:
//
//	user := &model.User{ID: "123", Username: "johndoe"}
//	session, err := store.Create(user)
//	if err != nil {
//		// Handle error
//	}
func (s *SessionStore) Create(user *model.User) (*Session, error) {
	sessionID, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	csrfToken, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		User:      user,
		CSRFToken: csrfToken,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return session, nil
}

// Get retrieves a session by ID
// This function checks if the session exists and is not expired. If the session is expired, it deletes it and returns an error.
// Parameters: sessionID - The ID of the session to retrieve
// Returns: *Session - A pointer to the Session if found and valid, error - An error if the session is not found or expired
// Example usage:
//
//	session, err := store.Get("some-session-id")
//	if err != nil {
//		// Handle error (e.g., session not found or expired)
//	}
func (s *SessionStore) Get(sessionID string) (*Session, error) {
	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrSessionNotFound
	}

	if session.IsExpired() {
		s.Delete(sessionID)
		return nil, ErrSessionExpired
	}

	return session, nil
}

// Delete removes a session
// This function deletes the session with the given ID from the session store.
// Parameters: sessionID - The ID of the session to delete
// Returns: None
// Example usage:
//
//	store.Delete("some-session-id")
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// Refresh extends the session expiration
// This function updates the expiration time of the session to be the current time plus the TTL. It returns an error if the session is not found.
// Parameters: sessionID - The ID of the session to refresh
// Returns: error - An error if the session is not found
// Example usage:
//
//	err := store.Refresh("some-session-id")
func (s *SessionStore) Refresh(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	session.ExpiresAt = time.Now().Add(s.ttl)
	return nil
}

// ValidateCSRF checks if the CSRF token is valid for the session
// This function retrieves the session and compares the provided CSRF token with the one stored in the session. It returns true if they match, false otherwise.
// Parameters: sessionID - The ID of the session to validate, csrfToken - The CSRF token to validate
// Returns: bool - True if the CSRF token is valid for the session, false otherwise
// Example usage:
//
//	isValid := store.ValidateCSRF("some-session-id", "some-csrf-token")
func (s *SessionStore) ValidateCSRF(sessionID, csrfToken string) bool {
	session, err := s.Get(sessionID)
	if err != nil {
		return false
	}
	return session.CSRFToken == csrfToken
}

// cleanupLoop periodically removes expired sessions
// This function runs in a separate goroutine and periodically calls the cleanup method to remove expired sessions from the session store.
// It uses a ticker to trigger the cleanup every 5 minutes.
func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup removes all expired sessions
func (s *SessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

// Count returns the number of active sessions
func (s *SessionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// TTL returns the session TTL duration
func (s *SessionStore) TTL() time.Duration {
	return s.ttl
}

// generateToken generates a random hex token
// This function generates a random byte slice of the specified length and encodes it as a hex string. It returns the generated token or an error if token generation fails.
// Parameters: length - The length of the token in bytes
// Returns: string - The generated token as a hex string, error - An error if token generation fails
// Example usage:
//
//	token, err := generateToken(32)
//	if err != nil {
//		// Handle error
//	}
func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateRandomToken generates a random hex token of the specified byte length
// This function is a wrapper around generateToken that returns an empty string if token generation fails. It is useful for cases where token generation errors can be safely ignored.
// Parameters: length - The length of the token in bytes
// Returns: string - The generated token as a hex string, or an empty string if token generation fails
// Example usage:
//
//	token := GenerateRandomToken(32) // Generates a random token of 32 bytes (64 hex characters)
func GenerateRandomToken(length int) string {
	token, err := generateToken(length)
	if err != nil {
		return ""
	}
	return token
}
