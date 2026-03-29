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
	"s3-storage/model"
	"s3-storage/services"
	"s3-storage/templates"
)

// UsersHandler handles user management UI routes
type UsersHandler struct {
	templates   *templates.PageTemplates
	userService *services.UserService
}

// NewUsersHandler creates a new users handler
func NewUsersHandler(tmpl *templates.PageTemplates, userService *services.UserService) *UsersHandler {
	return &UsersHandler{
		templates:   tmpl,
		userService: userService,
	}
}

// UserInfo holds user display information
type UserInfo struct {
	ID                string
	Username          string
	DisplayName       string
	Roles             []string
	Provider          string
	IsBootstrap       bool
	S3AccessKeyID     string
	S3SecretAccessKey string
}

// UsersPage renders the users management page
func (h *UsersHandler) UsersPage(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "UsersPage")

	var (
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "UsersPage handler")
	defer span.End()

	log.Debugf("Rendering users page for correlation ID: %s", correlationId)

	span.AddEvent("UsersPage",
		oteltrace.WithAttributes(attribute.String("CorrelationId", correlationId)))

	user := middleware.GetUserFromContext(c)
	csrfToken := middleware.GetCSRFToken(c)

	// Get all users
	usersList := h.userService.List()

	users := make([]UserInfo, 0, len(usersList))
	for _, u := range usersList {
		users = append(users, UserInfo{
			ID:                u.ID,
			Username:          u.Username,
			DisplayName:       u.DisplayName,
			Roles:             u.Roles,
			Provider:          u.Provider,
			IsBootstrap:       u.IsBootstrap,
			S3AccessKeyID:     u.S3AccessKeyID,
			S3SecretAccessKey: u.S3SecretAccessKey,
		})
	}

	err := h.templates.ExecuteTemplate(c.Writer, "users", gin.H{
		"Title":      "Users",
		"ActivePage": "users",
		"User":       user,
		"CSRFToken":  csrfToken,
		"Users":      users,
	})
	if err != nil {
		e := fmt.Errorf("template error: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// CreateUserRequest is the request body for creating a user
type CreateUserRequest struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}

// CreateUser handles user creation API
func (h *UsersHandler) CreateUser(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "CreateUser")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "CreateUser handler")
	defer span.End()

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		e = fmt.Errorf("invalid request: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	log.Debugf("Creating user: %s", req.Username)

	span.AddEvent("CreateUser",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("Username", req.Username),
		))

	if req.Username == "" || req.Password == "" {
		e = fmt.Errorf("username and password are required")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	if len(req.Password) < 8 {
		e = fmt.Errorf("password must be at least 8 characters")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	roles := req.Roles
	if len(roles) == 0 {
		roles = []string{"user"}
	}

	user, err := h.userService.Create(req.Username, req.Password, displayName, roles)
	if err != nil {
		if err == services.ErrUserExists {
			e = fmt.Errorf("username already exists: %s", req.Username)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
			return
		}
		e = fmt.Errorf("error creating user: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"roles":        user.Roles,
	})
}

// UpdateUserRequest is the request body for updating a user
type UpdateUserRequest struct {
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}

// UpdateUser handles user update API
func (h *UsersHandler) UpdateUser(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "UpdateUser")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	userID := c.Param("id")

	// tracer
	ctx, span := tracer.Start(ctx, "UpdateUser handler")
	defer span.End()

	log.Debugf("Updating user: %s", userID)

	span.AddEvent("UpdateUser",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("UserID", userID),
		))

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		e = fmt.Errorf("invalid request: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := h.userService.Update(userID, req.DisplayName, req.Roles)
	if err != nil {
		if err == services.ErrUserNotFound {
			e = fmt.Errorf("user not found: %s", userID)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if err == services.ErrBootstrapUser {
			e = fmt.Errorf("cannot modify bootstrap admin: %s", userID)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot modify bootstrap admin"})
			return
		}
		e = fmt.Errorf("error updating user: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"roles":        user.Roles,
	})
}

// DeleteUser handles user deletion API
func (h *UsersHandler) DeleteUser(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "DeleteUser")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	userID := c.Param("id")

	// tracer
	ctx, span := tracer.Start(ctx, "DeleteUser handler")
	defer span.End()

	log.Debugf("Deleting user: %s", userID)

	span.AddEvent("DeleteUser",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("UserID", userID),
		))

	err := h.userService.Delete(userID)
	if err != nil {
		if err == services.ErrUserNotFound {
			e = fmt.Errorf("user not found: %s", userID)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if err == services.ErrBootstrapUser {
			e = fmt.Errorf("cannot delete bootstrap admin: %s", userID)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete bootstrap admin"})
			return
		}
		e = fmt.Errorf("error deleting user: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

// GetCurrentUser returns the current logged-in user
func GetCurrentUser(c *gin.Context) *model.User {
	return middleware.GetUserFromContext(c)
}

// ChangePasswordRequest is the request body for changing own password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangeOwnPassword handles password change for the current user
func (h *UsersHandler) ChangeOwnPassword(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "ChangeOwnPassword")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "ChangeOwnPassword handler")
	defer span.End()

	log.Debugf("Changing password for correlation ID: %s", correlationId)

	span.AddEvent("ChangeOwnPassword",
		oteltrace.WithAttributes(attribute.String("CorrelationId", correlationId)))

	user := middleware.GetUserFromContext(c)
	if user == nil {
		e = fmt.Errorf("unauthorized")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Bootstrap users cannot change their passwords
	if user.IsBootstrap {
		e = fmt.Errorf("cannot change password for bootstrap user")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot change password for bootstrap user"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		e = fmt.Errorf("invalid request: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		e = fmt.Errorf("current password and new password are required")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password and new password are required"})
		return
	}

	if len(req.NewPassword) < 8 {
		e = fmt.Errorf("new password must be at least 8 characters")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password must be at least 8 characters"})
		return
	}

	// Verify current password
	if !h.userService.VerifyPassword(user.ID, req.CurrentPassword) {
		e = fmt.Errorf("current password is incorrect")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}

	// Update password
	if err := h.userService.UpdatePassword(user.ID, req.NewPassword); err != nil {
		if err == services.ErrBootstrapUser {
			e = fmt.Errorf("cannot change password for bootstrap user")
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot change password for bootstrap user"})
			return
		}
		e = fmt.Errorf("error updating password: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

// AdminResetPasswordRequest is the request body for admin resetting a user's password
type AdminResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// AdminResetPassword handles password reset by admin
func (h *UsersHandler) AdminResetPassword(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "AdminResetPassword")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	userID := c.Param("id")

	// tracer
	ctx, span := tracer.Start(ctx, "AdminResetPassword handler")
	defer span.End()

	log.Debugf("Admin resetting password for user: %s", userID)

	span.AddEvent("AdminResetPassword",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("UserID", userID),
		))

	var req AdminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		e = fmt.Errorf("invalid request: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.NewPassword == "" {
		e = fmt.Errorf("new password is required")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password is required"})
		return
	}

	if len(req.NewPassword) < 8 {
		e = fmt.Errorf("password must be at least 8 characters")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	// Update password
	if err := h.userService.UpdatePassword(userID, req.NewPassword); err != nil {
		if err == services.ErrUserNotFound {
			e = fmt.Errorf("user not found: %s", userID)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if err == services.ErrBootstrapUser {
			e = fmt.Errorf("cannot reset password for bootstrap user: %s", userID)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot reset password for bootstrap user"})
			return
		}
		e = fmt.Errorf("error resetting password: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}

// SetS3CredentialsRequest is the request body for setting S3 credentials
type SetS3CredentialsRequest struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// SetS3Credentials handles setting S3 credentials for a user
func (h *UsersHandler) SetS3Credentials(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "SetS3Credentials")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	userID := c.Param("id")

	// tracer
	ctx, span := tracer.Start(ctx, "SetS3Credentials handler")
	defer span.End()

	log.Debugf("Setting S3 credentials for user: %s", userID)

	span.AddEvent("SetS3Credentials",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("UserID", userID),
		))

	var req SetS3CredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		e = fmt.Errorf("invalid request: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.AccessKeyID == "" || req.SecretAccessKey == "" {
		e = fmt.Errorf("access key ID and secret access key are required")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "access key ID and secret access key are required"})
		return
	}

	// Set credentials
	if err := h.userService.SetS3Credentials(userID, req.AccessKeyID, req.SecretAccessKey); err != nil {
		if err == services.ErrUserNotFound {
			e = fmt.Errorf("user not found: %s", userID)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if err == services.ErrBootstrapUser {
			e = fmt.Errorf("cannot modify bootstrap admin credentials: %s", userID)
			span.RecordError(e)
			span.SetStatus(codes.Error, e.Error())
			log.Errorf("%s", e)
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot modify bootstrap admin credentials"})
			return
		}
		e = fmt.Errorf("error setting S3 credentials: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "S3 credentials updated successfully"})
}

// SetOwnS3Credentials handles users setting their own S3 credentials
func (h *UsersHandler) SetOwnS3Credentials(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "SetOwnS3Credentials")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "SetOwnS3Credentials handler")
	defer span.End()

	user := middleware.GetUserFromContext(c)
	if user == nil {
		e = fmt.Errorf("unauthorized")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	log.Debugf("User %s setting their own S3 credentials", user.Username)

	span.AddEvent("SetOwnS3Credentials",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("UserID", user.ID),
		))

	// Bootstrap users cannot set their credentials (they use global credentials)
	if user.IsBootstrap {
		e = fmt.Errorf("bootstrap admin cannot set S3 credentials")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusForbidden, gin.H{"error": "bootstrap admin uses global S3 credentials"})
		return
	}

	var req SetS3CredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		e = fmt.Errorf("invalid request: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.AccessKeyID == "" || req.SecretAccessKey == "" {
		e = fmt.Errorf("access key ID and secret access key are required")
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "access key ID and secret access key are required"})
		return
	}

	// Set credentials for the current user
	if err := h.userService.SetS3Credentials(user.ID, req.AccessKeyID, req.SecretAccessKey); err != nil {
		e = fmt.Errorf("error setting S3 credentials: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "S3 credentials updated successfully"})
}
