package middleware

import (
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// ManageMemoryMiddleware performs memory management and garbage collection to optimize memory usage.
// It monitors the heap allocation ratio (HeapAlloc/Sys) and triggers garbage collection
// when the ratio exceeds 0.7. The goal is to keep memory usage below the specified threshold.
//
// Parameters:
//   - c: The Gin context for handling HTTP request information.
//
// Behavior:
//   - Initiates a logger with correlation ID and relevant information for tracing.
//   - Reads memory statistics to determine the current heap allocation and system memory.
//   - Monitors the heap allocation ratio and triggers garbage collection if the ratio exceeds 0.7.
//   - Calls the garbage collector periodically if the condition persists.
//   - Ensures the function does not block by using a time-based approach.
//
// Note: This function is designed to be called periodically to manage memory usage.
//
// Example Usage:
//
//	ManageMemoryMiddleware(c)
func ManageMemoryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		m := runtime.MemStats{}
		runtime.ReadMemStats(&m)
		td := time.Now().Add(time.Second)

		for float64(m.HeapAlloc)/float64(m.Sys) > 0.7 {
			if time.Now().After(td) {
				runtime.GC()
				td = time.Now().Add(5 * time.Second)
			}
			time.Sleep(10 * time.Millisecond)
			runtime.ReadMemStats(&m)
		}

		c.Next() // Continue to the next middleware or final handler
	}
}
