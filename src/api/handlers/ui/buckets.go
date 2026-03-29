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
	"s3-storage/services"
	"s3-storage/templates"
)

// BucketsHandler handles bucket UI routes
type BucketsHandler struct {
	templates   *templates.PageTemplates
	userService *services.UserService
	metaStore   *services.MetaStore
}

// NewBucketsHandler creates a new buckets handler
func NewBucketsHandler(tmpl *templates.PageTemplates, userService *services.UserService, metaStore *services.MetaStore) *BucketsHandler {
	return &BucketsHandler{
		templates:   tmpl,
		userService: userService,
		metaStore:   metaStore,
	}
}

// BucketInfo holds bucket display information
type BucketInfo struct {
	Name         string
	ObjectCount  int64
	TotalSize    int64
	CreationDate int64
	Owner        string
	IsOwner      bool
	CanWrite     bool
}

// BucketsPage renders the buckets list page
func (h *BucketsHandler) BucketsPage(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "BucketsPage")

	var (
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "BucketsPage handler")
	defer span.End()

	log.Debugf("Rendering buckets page for correlation ID: %s", correlationId)

	span.AddEvent("BucketsPage",
		oteltrace.WithAttributes(attribute.String("CorrelationId", correlationId)))

	user := middleware.GetUserFromContext(c)
	csrfToken := middleware.GetCSRFToken(c)

	// Get buckets from metastore
	bucketMetas := services.ListBuckets()

	// Filter buckets based on user permissions
	filteredBuckets := services.FilterBucketsForUser(user, bucketMetas)

	buckets := make([]BucketInfo, 0, len(filteredBuckets))
	for _, b := range filteredBuckets {
		accessInfo := services.GetBucketAccessInfo(user, b)
		buckets = append(buckets, BucketInfo{
			Name:         b.Name,
			ObjectCount:  b.ObjectCount,
			TotalSize:    b.TotalSize,
			CreationDate: b.CreationDate,
			Owner:        b.Owner,
			IsOwner:      accessInfo.IsOwner,
			CanWrite:     accessInfo.CanWrite,
		})
	}

	err := h.templates.ExecuteTemplate(c.Writer, "buckets", gin.H{
		"Title":      "Buckets",
		"ActivePage": "buckets",
		"User":       user,
		"CSRFToken":  csrfToken,
		"Buckets":    buckets,
	})
	if err != nil {
		e := fmt.Errorf("template error: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// CreateBucket creates a new bucket with the current user as owner
func (h *BucketsHandler) CreateBucket(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "CreateBucket")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "CreateBucket handler")
	defer span.End()

	user := middleware.GetUserFromContext(c)

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		e = fmt.Errorf("invalid request: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	log.Debugf("Creating bucket: %s", req.Name)

	span.AddEvent("CreateBucket",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("BucketName", req.Name),
		))

	// Create bucket with user as owner
	if err := services.CreateBucketWithOwner(req.Name, user.ID); err != nil {
		e = fmt.Errorf("error creating bucket: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bucket created successfully"})
}

// DeleteBucket deletes a bucket (owner or admin only)
func (h *BucketsHandler) DeleteBucket(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "DeleteBucket")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	bucketName := c.Param("bucket")

	// tracer
	ctx, span := tracer.Start(ctx, "DeleteBucket handler")
	defer span.End()

	log.Debugf("Deleting bucket: %s", bucketName)

	span.AddEvent("DeleteBucket",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("BucketName", bucketName),
		))

	user := middleware.GetUserFromContext(c)

	// Check permissions
	if !services.CanAccessBucket(user, bucketName, true, h.metaStore) {
		e = fmt.Errorf("access denied to bucket: %s", bucketName)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if force delete
	var req struct {
		Force bool `json:"force"`
	}
	c.ShouldBindJSON(&req)

	// Get bucket to check ownership
	bucket, bucketErr := services.GetBucket(bucketName)
	isOwner := bucketErr == nil && bucket.Owner == user.ID

	var err error
	if req.Force && (user.IsAdmin() || isOwner) {
		err = services.ForceDeleteBucket(bucketName)
	} else {
		err = services.DeleteBucket(bucketName)
	}

	if err != nil {
		e = fmt.Errorf("error deleting bucket: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Clean up bucket permissions from all users
	h.userService.RemoveBucketPermissions(bucketName)

	c.JSON(http.StatusOK, gin.H{"message": "Bucket deleted successfully"})
}

// UserPermission represents a user's permission for the permissions page
type UserPermission struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	CanRead     bool   `json:"can_read"`
	CanWrite    bool   `json:"can_write"`
}

// BucketPermissionsPage renders the bucket permissions management page (admin only)
func (h *BucketsHandler) BucketPermissionsPage(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "BucketPermissionsPage")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	bucketName := c.Param("bucket")

	// tracer
	ctx, span := tracer.Start(ctx, "BucketPermissionsPage handler")
	defer span.End()

	log.Debugf("Rendering bucket permissions page for bucket: %s", bucketName)

	span.AddEvent("BucketPermissionsPage",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("BucketName", bucketName),
		))

	user := middleware.GetUserFromContext(c)
	csrfToken := middleware.GetCSRFToken(c)

	// Get bucket info
	bucket, err := services.GetBucket(bucketName)
	if err != nil {
		e = fmt.Errorf("bucket not found: %s", bucketName)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.Redirect(http.StatusFound, "/ui/buckets")
		return
	}

	// Get all non-admin users with their permissions
	allUsers := h.userService.ListNonAdminUsers()
	permissions := make([]UserPermission, 0, len(allUsers))

	for _, u := range allUsers {
		perm := u.GetBucketPermission(bucketName)
		up := UserPermission{
			UserID:      u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			CanRead:     false,
			CanWrite:    false,
		}

		// Check if user is the owner
		if bucket.Owner == u.ID {
			up.CanRead = true
			up.CanWrite = true
		} else if perm != nil {
			up.CanRead = perm.CanRead
			up.CanWrite = perm.CanWrite
		}

		permissions = append(permissions, up)
	}

	err = h.templates.ExecuteTemplate(c.Writer, "bucket_permissions", gin.H{
		"Title":       bucketName + " - Permissions",
		"ActivePage":  "buckets",
		"User":        user,
		"CSRFToken":   csrfToken,
		"Bucket":      bucketName,
		"BucketOwner": bucket.Owner,
		"Permissions": permissions,
	})
	if err != nil {
		e = fmt.Errorf("template error: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// SetBucketPermission sets bucket permissions for a user (admin only)
func (h *BucketsHandler) SetBucketPermission(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "SetBucketPermission")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	bucketName := c.Param("bucket")

	// tracer
	ctx, span := tracer.Start(ctx, "SetBucketPermission handler")
	defer span.End()

	log.Debugf("Setting bucket permission for bucket: %s", bucketName)

	var req struct {
		UserID   string `json:"user_id"`
		CanRead  bool   `json:"can_read"`
		CanWrite bool   `json:"can_write"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		e = fmt.Errorf("invalid request: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	span.AddEvent("SetBucketPermission",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("BucketName", bucketName),
			attribute.String("UserID", req.UserID),
		))

	// Get bucket to check owner
	bucket, err := services.GetBucket(bucketName)
	if err != nil {
		e = fmt.Errorf("bucket not found: %s", bucketName)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusNotFound, gin.H{"error": "Bucket not found"})
		return
	}

	// Cannot modify permissions for bucket owner
	if bucket.Owner == req.UserID {
		e = fmt.Errorf("cannot modify owner permissions for bucket: %s", bucketName)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot modify owner permissions"})
		return
	}

	if err := h.userService.SetBucketPermission(req.UserID, bucketName, req.CanRead, req.CanWrite); err != nil {
		e = fmt.Errorf("error setting bucket permission: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permission updated successfully"})
}
