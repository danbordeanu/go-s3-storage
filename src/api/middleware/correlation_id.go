package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CorrelationId is a middleware that checks for the presence of a correlation ID in the incoming request headers.
// If a correlation ID is not found, it generates a new one using the uuid package.
// The correlation ID is then stored in the Gin context for use in subsequent handlers and middleware.
func CorrelationId() gin.HandlerFunc {
	return func(c *gin.Context) {
		var correlationId string
		if correlationId = c.Request.Header.Get("X-Correlation-ID"); correlationId == "" {
			correlationId = uuid.New().String()
		}
		c.Set("correlation_id", correlationId)
		c.Next()
	}
}
