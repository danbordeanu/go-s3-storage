package ui

import (
	"encoding/json"
	"fmt"
	"html/template"
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

// DashboardHandler handles dashboard UI routes
type DashboardHandler struct {
	templates    *templates.PageTemplates
	statsService *services.StatsService
	metaStore    *services.MetaStore
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(tmpl *templates.PageTemplates, statsService *services.StatsService, metaStore *services.MetaStore) *DashboardHandler {
	return &DashboardHandler{
		templates:    tmpl,
		statsService: statsService,
		metaStore:    metaStore,
	}
}

// Dashboard renders the dashboard page
func (h *DashboardHandler) Dashboard(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "Dashboard")

	var (
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "Dashboard handler")
	defer span.End()

	log.Debugf("Rendering dashboard for correlation ID: %s", correlationId)

	span.AddEvent("Dashboard",
		oteltrace.WithAttributes(attribute.String("CorrelationId", correlationId)))

	user := middleware.GetUserFromContext(c)
	csrfToken := middleware.GetCSRFToken(c)
	// Get stats filtered by user permissions (non-admins only see buckets they have access to)
	stats := h.statsService.GetStatsForUser(user)
	contentTypeStats := h.statsService.GetContentTypeStatsForUser(user)

	// Convert buckets to JSON for chart rendering
	bucketsJSON, _ := json.Marshal(stats.Buckets)
	contentTypesJSON, _ := json.Marshal(contentTypeStats)

	err := h.templates.ExecuteTemplate(c.Writer, "dashboard", gin.H{
		"Title":            "Dashboard",
		"ActivePage":       "dashboard",
		"User":             user,
		"CSRFToken":        csrfToken,
		"Stats":            stats,
		"BucketsJSON":      template.JS(bucketsJSON),
		"ContentTypesJSON": template.JS(contentTypesJSON),
		"ContentTypeStats": contentTypeStats,
	})
	if err != nil {
		e := fmt.Errorf("template error: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// StatsAPI returns statistics as JSON (filtered by user permissions)
func (h *DashboardHandler) StatsAPI(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "StatsAPI")

	var (
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "StatsAPI handler")
	defer span.End()

	log.Debugf("Getting stats for correlation ID: %s", correlationId)

	span.AddEvent("StatsAPI",
		oteltrace.WithAttributes(attribute.String("CorrelationId", correlationId)))

	user := middleware.GetUserFromContext(c)
	stats := h.statsService.GetStatsForUser(user)
	c.JSON(http.StatusOK, stats)
}

// BucketStatsAPI returns statistics for a specific bucket
func (h *DashboardHandler) BucketStatsAPI(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "BucketStatsAPI")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	bucketName := c.Param("bucket")

	// tracer
	ctx, span := tracer.Start(ctx, "BucketStatsAPI handler")
	defer span.End()

	log.Debugf("Getting bucket stats for bucket: %s", bucketName)

	span.AddEvent("BucketStatsAPI",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("BucketName", bucketName),
		))

	user := middleware.GetUserFromContext(c)

	// Check if user can access this bucket
	if !services.CanAccessBucket(user, bucketName, false, h.metaStore) {
		e = fmt.Errorf("access denied to bucket: %s", bucketName)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	stats, err := h.statsService.GetBucketStats(bucketName)
	if err != nil {
		e = fmt.Errorf("bucket not found: %s", bucketName)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.JSON(http.StatusNotFound, gin.H{"error": "bucket not found"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
