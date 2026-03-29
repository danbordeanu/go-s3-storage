package middleware

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	"s3-storage/auth"
	"s3-storage/model"
	"s3-storage/services"
)

// SigV4Auth returns a Gin middleware that validates AWS SigV4 signatures
func SigV4Auth(credStore auth.CredentialStore, region string, userService *services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is already authenticated via session (from Web UI)
		if user, exists := c.Get(ContextKeyUser); exists && user != nil {
			// User is authenticated via session, skip SigV4 auth
			c.Next()
			return
		}

		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			sendAuthError(c, services.ErrMissingSecurityHeader, "Authorization header is required")
			return
		}

		// Check if this is an AWS4-HMAC-SHA256 request
		if !strings.HasPrefix(authHeader, auth.Algorithm) {
			sendAuthError(c, services.ErrAccessDenied, "Unsupported authorization method")
			return
		}

		// Parse the Authorization header
		parsedAuth, err := auth.ParseAuthorizationHeader(authHeader)
		if err != nil {
			sendAuthError(c, services.ErrAccessDenied, err.Error())
			return
		}

		// Look up the credential
		cred, err := credStore.GetCredential(parsedAuth.AccessKeyID)
		if err != nil {
			sendAuthError(c, services.ErrInvalidAccessKeyId, "")
			return
		}

		// Determine if request came through a proxy
		throughProxy := auth.IsThroughProxy(c.Request)

		// For large uploads (>10MB), skip body buffering to avoid memory exhaustion
		// The AWS signature verification in Authorization header is cryptographically secure
		const maxBodyBufferSize = 10 * 1024 * 1024 // 10MB
		skipBodyBuffering := throughProxy || c.Request.ContentLength > maxBodyBufferSize || c.Request.ContentLength < 0

		// Read and buffer the request body for signature verification
		var body []byte
		if !skipBodyBuffering && c.Request.Body != nil && c.Request.ContentLength != 0 {
			// Small file on direct connection: read into memory for full signature validation
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				sendAuthError(c, services.ErrAccessDenied, "Failed to read request body")
				return
			}
			// Restore the body for downstream handlers
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		// For proxied requests or large files, body remains nil and will use UNSIGNED-PAYLOAD for verification

		// Verify the signature
		err = auth.VerifySignature(c.Request, body, parsedAuth, cred.SecretAccessKey, region)
		if err != nil {
			// Determine specific error type
			if strings.Contains(err.Error(), "time") || strings.Contains(err.Error(), "skewed") {
				sendAuthError(c, services.ErrRequestTimeTooSkewed, "")
			} else if strings.Contains(err.Error(), "signature") {
				sendAuthError(c, services.ErrSignatureDoesNotMatch, "")
			} else {
				sendAuthError(c, services.ErrSignatureDoesNotMatch, err.Error())
			}
			return
		}

		// Store access key ID in context for potential logging/auditing
		c.Set("accessKeyId", parsedAuth.AccessKeyID)

		// Look up and set user context for ACL enforcement
		if memStore, ok := credStore.(*auth.MemoryStore); ok {
			if userID, err := memStore.GetUserIDByAccessKey(parsedAuth.AccessKeyID); err == nil {
				if user, err := userService.GetByID(userID); err == nil {
					c.Set(ContextKeyUser, user)
				}
			}
		}

		c.Next()
	}
}

// sendAuthError sends an S3-compatible XML error response
func sendAuthError(c *gin.Context, err error, detail string) {
	code := services.S3ErrorCode(err)
	message := services.S3ErrorMessage(err)
	if detail != "" {
		message = message + " " + detail
	}
	status := services.S3ErrorHTTPStatus(err)

	s3Error := model.S3Error{
		Code:      code,
		Message:   message,
		Resource:  c.Request.URL.Path,
		RequestID: c.GetHeader("X-Correlation-Id"),
	}

	c.Header("Content-Type", "application/xml")
	c.Writer.WriteHeader(status)
	xml.NewEncoder(c.Writer).Encode(s3Error)
	c.Abort()
}
