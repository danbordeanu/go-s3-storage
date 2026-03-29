package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// LimiterMiddleware is a Gin middleware that provides rate limiting for incoming requests
// based on the specified parameters for both requests per second and data transfer rate per second.
//
// Parameters:
//   - maxEventsPerSec: Maximum number of requests allowed per second.
//   - maxBurstSize: Maximum burst size for incoming requests.
//
// Returns a Gin middleware function that performs rate limiting and aborts requests that exceed
// the defined limits with a "Too Many Requests" response.
func LimiterMiddleware(maxEventsPerSec, maxBurstSize int) gin.HandlerFunc {
	// TODO implement also max rate limit
	requestLimiter := rate.NewLimiter(rate.Limit(float64(maxEventsPerSec)), maxBurstSize)

	return func(c *gin.Context) {
		if requestLimiter.Allow() {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Limit Exceeded"})
		return
	}
}
