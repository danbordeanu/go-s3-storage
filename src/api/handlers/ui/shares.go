package ui

import (
	"fmt"
	"net/http"
	"time"

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

// SharesHandler handles share link UI routes
type SharesHandler struct {
	templates *templates.PageTemplates
}

// NewSharesHandler creates a new shares handler
func NewSharesHandler(tmpl *templates.PageTemplates) *SharesHandler {
	return &SharesHandler{
		templates: tmpl,
	}
}

// ShareLinkInfo holds share link display information
type ShareLinkInfo struct {
	Token     string
	Bucket    string
	Key       string
	CreatedAt int64
	ExpiresAt int64
	IsExpired bool
	URL       string
}

// SharesPage renders the share links page
func (h *SharesHandler) SharesPage(c *gin.Context) {
	concurrency.GlobalWaitGroup.Add(1)
	defer concurrency.GlobalWaitGroup.Done()
	log := logger.SugaredLogger().WithContextCorrelationId(c).With("package", "handlers/ui", "action", "SharesPage")

	var (
		ctx           = c.Request.Context()
		correlationId = c.MustGet("correlation_id").(string)
	)

	// tracer
	ctx, span := tracer.Start(ctx, "SharesPage handler")
	defer span.End()

	log.Debugf("Rendering shares page for correlation ID: %s", correlationId)

	span.AddEvent("SharesPage",
		oteltrace.WithAttributes(attribute.String("CorrelationId", correlationId)))

	user := middleware.GetUserFromContext(c)
	csrfToken := middleware.GetCSRFToken(c)

	// Get all share links
	links := services.GetAllShareLinks()

	// Build share info with URLs
	baseURL := configuration.AppConfig().RequestBaseUrl
	now := time.Now().Unix()
	shares := make([]ShareLinkInfo, 0, len(links))
	for _, link := range links {
		isExpired := link.ExpiresAt > 0 && now > link.ExpiresAt
		shares = append(shares, ShareLinkInfo{
			Token:     link.Token,
			Bucket:    link.Bucket,
			Key:       link.Key,
			CreatedAt: link.CreatedAt,
			ExpiresAt: link.ExpiresAt,
			IsExpired: isExpired,
			URL:       baseURL + "/share/" + link.Token,
		})
	}

	err := h.templates.ExecuteTemplate(c.Writer, "shares", gin.H{
		"Title":      "Share Links",
		"ActivePage": "shares",
		"User":       user,
		"CSRFToken":  csrfToken,
		"Shares":     shares,
	})
	if err != nil {
		e := fmt.Errorf("template error: %v", err)
		span.RecordError(e)
		span.SetStatus(codes.Error, e.Error())
		log.Errorf("%s", e)
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}
