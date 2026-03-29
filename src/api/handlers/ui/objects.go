package ui

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/danbordeanu/go-logger"
	"github.com/danbordeanu/go-stats/concurrency"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"s3-storage/api/middleware"
	"s3-storage/configuration"
	"s3-storage/services"
	"s3-storage/templates"
)

// ObjectsHandler handles object browser UI routes
type ObjectsHandler struct {
	templates *templates.PageTemplates
	metaStore *services.MetaStore
}

// NewObjectsHandler creates a new objects handler
func NewObjectsHandler(tmpl *templates.PageTemplates, metaStore *services.MetaStore) *ObjectsHandler {
	return &ObjectsHandler{
		templates: tmpl,
		metaStore: metaStore,
	}
}

// PathPart represents a path component for breadcrumbs
type PathPart struct {
	Name     string
	FullPath string
}

// ObjectsPage renders the object browser page
func (h *ObjectsHandler) ObjectsPage(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "ObjectsPage")

	var (
		e             error
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	bucket := c.Param("bucket")
	prefix := c.Query("prefix")

	// tracer
	ctx, span := tracer.Start(ctx, "ObjectsPage handler")
	defer span.End()

	log.Debugf("Rendering objects page for bucket: %s, prefix: %s", bucket, prefix)

	span.AddEvent("ObjectsPage",
		oteltrace.WithAttributes(
			attribute.String("CorrelationId", correlationId),
			attribute.String("BucketName", bucket),
			attribute.String("Prefix", prefix),
		))

	user := middleware.GetUserFromContext(c)
	csrfToken := middleware.GetCSRFToken(c)

	// Check if user can access this bucket (read permission)
	if !services.CanAccessBucket(user, bucket, false, h.metaStore) {
		e = fmt.Errorf("access denied to bucket: %s", bucket)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.String(http.StatusForbidden, "Access denied")
		return
	}

	// Check write permission for UI controls
	canWrite := services.CanAccessBucket(user, bucket, true, h.metaStore)

	// Get configuration
	conf := configuration.AppConfig()

	// Pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", strconv.Itoa(int(conf.UIObjectsPerPage))))

	// Validate per_page
	if perPage < 1 {
		perPage = int(conf.UIObjectsPerPage)
	}
	if perPage > int(conf.UIMaxObjectsPerPage) {
		perPage = int(conf.UIMaxObjectsPerPage)
	}

	// Sorting parameters
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")

	// Search parameter
	search := c.Query("search")

	// List objects with pagination, sorting, and search
	result, err := services.ListObjectsPaginated(bucket, prefix, "/", page, perPage, sortBy, sortOrder, search)
	if err != nil {
		e = fmt.Errorf("error listing objects: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		// Bucket may have been deleted - redirect to buckets page
		c.Redirect(http.StatusFound, "/ui/buckets")
		return
	}

	// Build path parts for breadcrumbs
	pathParts := make([]PathPart, 0)
	if prefix != "" {
		parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
		for i, part := range parts {
			if part != "" {
				fullPath := strings.Join(parts[:i+1], "/") + "/"
				pathParts = append(pathParts, PathPart{
					Name:     part,
					FullPath: fullPath,
				})
			}
		}
	}

	// Calculate parent prefix for ".." navigation
	parentPrefix := ""
	if prefix != "" {
		parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
		if len(parts) > 1 {
			parentPrefix = strings.Join(parts[:len(parts)-1], "/") + "/"
		}
	}

	// Build pagination data
	pagination := buildPagination(result.Page, result.TotalPages)

	// Available per-page options
	perPageOptions := []int{25, 50, 100, 250, 500}

	err = h.templates.ExecuteTemplate(c.Writer, "objects", gin.H{
		"Title":          bucket + " - Objects",
		"ActivePage":     "buckets",
		"User":           user,
		"CSRFToken":      csrfToken,
		"Bucket":         bucket,
		"Prefix":         prefix,
		"ParentPrefix":   parentPrefix,
		"PathParts":      pathParts,
		"Objects":        result.Objects,
		"CommonPrefixes": result.CommonPrefixes,
		"TotalObjects":   result.TotalObjects,
		"TotalFolders":   result.TotalFolders,
		"Page":           result.Page,
		"PerPage":        result.PerPage,
		"TotalPages":     result.TotalPages,
		"SortBy":         result.SortBy,
		"SortOrder":      result.SortOrder,
		"Search":         result.Search,
		"Pagination":     pagination,
		"PerPageOptions": perPageOptions,
		"CanWrite":       canWrite,
	})
	if err != nil {
		e = fmt.Errorf("template error: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// PaginationItem represents a pagination link
type PaginationItem struct {
	Page       int
	Label      string
	IsCurrent  bool
	IsEllipsis bool
}

// buildPagination creates pagination items for the template
func buildPagination(currentPage, totalPages int) []PaginationItem {
	items := make([]PaginationItem, 0)

	if totalPages <= 1 {
		return items
	}

	// Always show first page
	items = append(items, PaginationItem{Page: 1, Label: "1", IsCurrent: currentPage == 1})

	// Calculate range around current page
	start := currentPage - 2
	end := currentPage + 2

	if start < 2 {
		start = 2
	}
	if end > totalPages-1 {
		end = totalPages - 1
	}

	// Add ellipsis after first page if needed
	if start > 2 {
		items = append(items, PaginationItem{IsEllipsis: true, Label: "..."})
	}

	// Add pages around current page
	for i := start; i <= end; i++ {
		items = append(items, PaginationItem{Page: i, Label: strconv.Itoa(i), IsCurrent: currentPage == i})
	}

	// Add ellipsis before last page if needed
	if end < totalPages-1 {
		items = append(items, PaginationItem{IsEllipsis: true, Label: "..."})
	}

	// Always show last page if more than 1 page
	if totalPages > 1 {
		items = append(items, PaginationItem{Page: totalPages, Label: strconv.Itoa(totalPages), IsCurrent: currentPage == totalPages})
	}

	return items
}
