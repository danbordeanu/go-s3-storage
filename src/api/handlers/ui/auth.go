package ui

import (
	"fmt"
	"net/http"

	"github.com/danbordeanu/go-logger"
	"github.com/danbordeanu/go-stats/concurrency"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"s3-storage/api/middleware"
	"s3-storage/auth"
	"s3-storage/templates"
)

// AuthHandler handles authentication UI routes
type AuthHandler struct {
	templates    *templates.PageTemplates
	localAuth    *auth.LocalProvider
	sessionStore *auth.SessionStore
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(tmpl *templates.PageTemplates, localAuth *auth.LocalProvider, sessionStore *auth.SessionStore) *AuthHandler {
	return &AuthHandler{
		templates:    tmpl,
		localAuth:    localAuth,
		sessionStore: sessionStore,
	}
}

// LoginPage renders the login page
func (h *AuthHandler) LoginPage(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "LoginPage")

	var (
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "LoginPage handler")
	defer span.End()

	log.Debugf("Rendering login page for correlation ID: %s", correlationId)

	span.AddEvent("LoginPage",
		oteltrace.WithAttributes(attribute.String("CorrelationId", correlationId)))

	// If already logged in, redirect to dashboard
	sessionID, err := c.Cookie(middleware.SessionCookieName)
	if err == nil && sessionID != "" {
		if _, err := h.sessionStore.Get(sessionID); err == nil {
			c.Redirect(http.StatusFound, "/ui/dashboard")
			return
		}
	}

	// Generate a temporary CSRF token for the login form
	csrfToken := "login-" + generateCSRFToken()

	h.templates.ExecuteTemplate(c.Writer, "login", gin.H{
		"Title":     "Login",
		"CSRFToken": csrfToken,
		"Error":     c.Query("error"),
		"Username":  "",
	})
}

// Login handles login form submission
func (h *AuthHandler) Login(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "Login")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "Login handler")
	defer span.End()

	username := c.PostForm("username")

	log.Debugf("Login attempt for user: %s", username)

	span.AddEvent("Login",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("Username", username),
		))

	password := c.PostForm("password")

	// Authenticate user
	user, err := h.localAuth.Authenticate(ctx, map[string]string{
		auth.CredentialUsername: username,
		auth.CredentialPassword: password,
	})

	if err != nil {
		e = fmt.Errorf("authentication failed for user: %s", username)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		h.templates.ExecuteTemplate(c.Writer, "login", gin.H{
			"Title":     "Login",
			"CSRFToken": "login-" + generateCSRFToken(),
			"Error":     "Invalid username or password",
			"Username":  username,
		})
		return
	}

	// Create session
	session, err := h.sessionStore.Create(user)
	if err != nil {
		e = fmt.Errorf("failed to create session for user: %s", username)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		h.templates.ExecuteTemplate(c.Writer, "login", gin.H{
			"Title":     "Login",
			"CSRFToken": "login-" + generateCSRFToken(),
			"Error":     "Failed to create session",
			"Username":  username,
		})
		return
	}

	// Set session cookie
	c.SetCookie(
		middleware.SessionCookieName,
		session.ID,
		int(h.sessionStore.TTL().Seconds()),
		"/",
		"",    // Domain
		false, // Secure (set to true in production with HTTPS)
		true,  // HttpOnly
	)

	c.Redirect(http.StatusFound, "/ui/dashboard")
}

// Logout handles logout
func (h *AuthHandler) Logout(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "Logout")

	var (
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "Logout handler")
	defer span.End()

	log.Debugf("Logout for correlation ID: %s", correlationId)

	span.AddEvent("Logout",
		oteltrace.WithAttributes(attribute.String("CorrelationId", correlationId)))

	sessionID, err := c.Cookie(middleware.SessionCookieName)
	if err == nil && sessionID != "" {
		h.sessionStore.Delete(sessionID)
	}

	// Clear cookie
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", false, true)

	c.Redirect(http.StatusFound, "/ui/login")
}

// generateCSRFToken generates a simple CSRF token
func generateCSRFToken() string {
	return auth.GenerateRandomToken(16)
}
