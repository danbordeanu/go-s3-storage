package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"s3-storage/auth"
	"s3-storage/model"
)

const (
	// SessionCookieName is the name of the session cookie
	SessionCookieName = "s3_session"

	// ContextKeyUser is the key for storing user in context
	ContextKeyUser = "user"

	// ContextKeySession is the key for storing session in context
	ContextKeySession = "session"

	// ContextKeyCSRFToken is the key for storing CSRF token in context
	ContextKeyCSRFToken = "csrf_token"
)

// SessionAuth creates a middleware that validates session cookies
func SessionAuth(sessionStore *auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(SessionCookieName)
		if err != nil || sessionID == "" {
			c.Redirect(http.StatusFound, "/ui/login")
			c.Abort()
			return
		}

		session, err := sessionStore.Get(sessionID)
		if err != nil {
			// Clear invalid/expired cookie
			c.SetCookie(SessionCookieName, "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/ui/login")
			c.Abort()
			return
		}

		// Refresh session on each request
		sessionStore.Refresh(sessionID)

		// Store session and user in context
		c.Set(ContextKeySession, session)
		c.Set(ContextKeyUser, session.User)
		c.Set(ContextKeyCSRFToken, session.CSRFToken)

		c.Next()
	}
}

// OptionalSessionAuth attempts to load session but doesn't require it
// This allows S3 API routes to work with either session auth (UI) or SigV4 auth (API clients)
func OptionalSessionAuth(sessionStore *auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(SessionCookieName)
		if err != nil || sessionID == "" {
			// No session cookie, continue without setting user context
			c.Next()
			return
		}

		session, err := sessionStore.Get(sessionID)
		if err != nil {
			// Invalid/expired session, continue without setting user context
			c.Next()
			return
		}

		// Refresh session on each request
		sessionStore.Refresh(sessionID)

		// Store session and user in context
		c.Set(ContextKeySession, session)
		c.Set(ContextKeyUser, session.User)
		c.Set(ContextKeyCSRFToken, session.CSRFToken)

		c.Next()
	}
}

// RequireRole creates a middleware that checks if user has required role
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get(ContextKeyUser)
		if !exists {
			c.Redirect(http.StatusFound, "/ui/login")
			c.Abort()
			return
		}

		u, ok := user.(*model.User)
		if !ok || !u.HasRole(role) {
			c.HTML(http.StatusForbidden, "error", gin.H{
				"Title":   "Access Denied",
				"Message": "You don't have permission to access this page",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CSRFProtection validates CSRF tokens for state-changing requests
func CSRFProtection(sessionStore *auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF check for safe methods
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Get session
		session, exists := c.Get(ContextKeySession)
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no session"})
			return
		}

		sess, ok := session.(*auth.Session)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid session"})
			return
		}

		// Check CSRF token from header or form
		csrfToken := c.GetHeader("X-CSRF-Token")
		if csrfToken == "" {
			csrfToken = c.PostForm("csrf_token")
		}

		if csrfToken != sess.CSRFToken {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid CSRF token"})
			return
		}

		c.Next()
	}
}

// GetUserFromContext retrieves the user from gin context
func GetUserFromContext(c *gin.Context) *model.User {
	user, exists := c.Get(ContextKeyUser)
	if !exists {
		return nil
	}
	u, ok := user.(*model.User)
	if !ok {
		return nil
	}
	return u
}

// GetSessionFromContext retrieves the session from gin context
func GetSessionFromContext(c *gin.Context) *auth.Session {
	session, exists := c.Get(ContextKeySession)
	if !exists {
		return nil
	}
	s, ok := session.(*auth.Session)
	if !ok {
		return nil
	}
	return s
}

// GetCSRFToken retrieves the CSRF token from gin context
func GetCSRFToken(c *gin.Context) string {
	token, exists := c.Get(ContextKeyCSRFToken)
	if !exists {
		return ""
	}
	t, ok := token.(string)
	if !ok {
		return ""
	}
	return t
}
