package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// AWS4-HMAC-SHA256 is the algorithm identifier
	Algorithm = "AWS4-HMAC-SHA256"
	// TimeFormat is the AWS timestamp format
	TimeFormat = "20060102T150405Z"
	// DateFormat is the AWS date format
	DateFormat = "20060102"
	// MaxRequestAge is the maximum allowed age for a request (15 minutes)
	MaxRequestAge = 15 * time.Minute
)

// IsThroughProxy detects if the request came through a reverse proxy
// by checking common proxy headers (Cloudflare, Traefik, nginx, etc.)
func IsThroughProxy(r *http.Request) bool {
	return r.Header.Get("CF-Ray") != "" || // Cloudflare
		r.Header.Get("X-Forwarded-For") != "" || // Generic proxy
		r.Header.Get("X-Real-IP") != "" // Traefik/nginx
}

// AuthorizationHeader represents parsed AWS4-HMAC-SHA256 Authorization header
type AuthorizationHeader struct {
	Algorithm     string
	Credential    string
	SignedHeaders string
	Signature     string
	AccessKeyID   string
	Date          string
	Region        string
	Service       string
}

var authHeaderRegex = regexp.MustCompile(`^AWS4-HMAC-SHA256\s+Credential=([^,]+),\s*SignedHeaders=([^,]+),\s*Signature=([a-f0-9]+)$`)

// ParseAuthorizationHeader parses an AWS4-HMAC-SHA256 Authorization header
// Example header:
// AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20231129/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890
// Returns an AuthorizationHeader struct or an error if parsing fails
// Parameters: header - The raw Authorization header string to parse
// Returns: *AuthorizationHeader - A pointer to the parsed AuthorizationHeader struct, or an error if parsing fails
// Example usage:
//
//	authHeader, err := ParseAuthorizationHeader(r.Header.Get("Authorization"))
//	if err != nil {
//		// Handle error (e.g., return 401 Unauthorized)
//	}
func ParseAuthorizationHeader(header string) (*AuthorizationHeader, error) {
	matches := authHeaderRegex.FindStringSubmatch(header)
	if matches == nil {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	credential := matches[1]
	signedHeaders := matches[2]
	signature := matches[3]

	// Parse credential: AccessKeyID/Date/Region/Service/aws4_request
	credParts := strings.Split(credential, "/")
	if len(credParts) != 5 {
		return nil, fmt.Errorf("invalid credential format")
	}

	if credParts[4] != "aws4_request" {
		return nil, fmt.Errorf("invalid credential terminator")
	}

	return &AuthorizationHeader{
		Algorithm:     Algorithm,
		Credential:    credential,
		SignedHeaders: signedHeaders,
		Signature:     signature,
		AccessKeyID:   credParts[0],
		Date:          credParts[1],
		Region:        credParts[2],
		Service:       credParts[3],
	}, nil
}

// CredentialScope returns the credential scope string
// Example: "20231129/us-east-1/s3/aws4_request"
// Parameters: None (uses fields from the AuthorizationHeader struct)
// Returns: string - The credential scope string in the format "Date/Region/Service/aws4_request"
// Example usage:
//
//	scope := authHeader.CredentialScope()
func (a *AuthorizationHeader) CredentialScope() string {
	return fmt.Sprintf("%s/%s/%s/aws4_request", a.Date, a.Region, a.Service)
}

// BuildCanonicalRequest builds the canonical request string from an HTTP request
// Parameters:
//   - r: The HTTP request to build the canonical request from
//   - signedHeaders: A semicolon-separated list of headers that were signed (from the Authorization header)
//   - payloadHash: The SHA256 hash of the request payload (or "UNSIGNED-PAYLOAD" if not signed)
//
// Returns: string - The canonical request string to be used in signature calculation
// Example usage:
//
//	canonicalRequest := BuildCanonicalRequest(r, authHeader.SignedHeaders, payloadHash)
func BuildCanonicalRequest(r *http.Request, signedHeaders string, payloadHash string) string {
	// HTTP Method
	httpMethod := r.Method

	// Canonical URI (path, must be URI-encoded)
	canonicalURI := r.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	// Canonical Query String (sorted by parameter name)
	canonicalQueryString := buildCanonicalQueryString(r.URL.Query())

	// Canonical Headers (lowercase, trimmed, sorted)
	headerList := strings.Split(signedHeaders, ";")
	canonicalHeaders := buildCanonicalHeaders(r, headerList)

	// Build the canonical request
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)
}

// buildCanonicalQueryString builds the canonical query string by sorting parameters and URI-encoding them
// Parameters: values - The URL query parameters to build the canonical query string from
// Returns: string - The canonical query string with parameters sorted and URI-encoded
// Example usage:
//
//	canonicalQueryString := buildCanonicalQueryString(r.URL.Query())
func buildCanonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		vs := values[k]
		sort.Strings(vs)
		for _, v := range vs {
			pairs = append(pairs, fmt.Sprintf("%s=%s", uriEncode(k, true), uriEncode(v, true)))
		}
	}

	return strings.Join(pairs, "&")
}

// buildCanonicalHeaders builds the canonical headers string by lowercasing, trimming, and sorting the specified headers
// Parameters:
//   - r: The HTTP request to extract headers from
//   - headerList: A list of header names that were signed (from the Authorization header)
//
// Returns: string - The canonical headers string with specified headers lowercased, trimmed, and sorted
// Example usage:
//
//	canonicalHeaders := buildCanonicalHeaders(r, strings.Split(authHeader.SignedHeaders, ";"))
func buildCanonicalHeaders(r *http.Request, headerList []string) string {
	var headers []string
	for _, h := range headerList {
		h = strings.ToLower(strings.TrimSpace(h))
		var value string
		if h == "host" {
			value = r.Host
		} else {
			value = r.Header.Get(h)
		}
		// Trim and collapse whitespace
		value = strings.TrimSpace(value)
		headers = append(headers, fmt.Sprintf("%s:%s\n", h, value))
	}
	return strings.Join(headers, "")
}

// uriEncode performs AWS-style URI encoding
// Parameters:
//   - s: The string to URI-encode
//   - encodeSlash: Whether to encode the '/' character (true for query parameters, false for path)
//
// Returns: string - The URI-encoded string according to AWS SigV4 rules
// Example usage:
//
//	encoded := uriEncode("my bucket/key", false) // For path
//	encodedQuery := uriEncode("value with spaces", true) // For query parameter
func uriEncode(s string, encodeSlash bool) string {
	var encoded strings.Builder
	for _, b := range []byte(s) {
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') ||
			(b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '~' {
			encoded.WriteByte(b)
		} else if b == '/' && !encodeSlash {
			encoded.WriteByte(b)
		} else {
			encoded.WriteString(fmt.Sprintf("%%%02X", b))
		}
	}
	return encoded.String()
}

// BuildStringToSign creates the string to sign for AWS SigV4
// Parameters:
//   - timestamp: The time of the request (from X-Amz-Date header)
//   - credentialScope: The credential scope string (Date/Region/Service/aws4_request)
//   - canonicalRequest: The canonical request string built from the HTTP request
//
// Returns: string - The string to sign for AWS SigV4 signature calculation
// Example usage:
//
//	stringToSign := BuildStringToSign(timestamp, authHeader.CredentialScope(), canonicalRequest)
func BuildStringToSign(timestamp time.Time, credentialScope string, canonicalRequest string) string {
	canonicalRequestHash := sha256Hex([]byte(canonicalRequest))
	return fmt.Sprintf("%s\n%s\n%s\n%s",
		Algorithm,
		timestamp.UTC().Format(TimeFormat),
		credentialScope,
		canonicalRequestHash,
	)
}

// DeriveSigningKey derives the signing key using HMAC-SHA256 chain
// Parameters:
//   - secretKey: The AWS secret access key
//   - date: The date from the credential (YYYYMMDD)
//   - region: The AWS region (e.g., us-east-1)
//   - service: The AWS service (e.g., s3)
//
// Returns: []byte - The derived signing key to be used for signature calculation
// Example usage:
//
//	signingKey := DeriveSigningKey(secretKey, authHeader.Date, authHeader.Region, authHeader.Service)
func DeriveSigningKey(secretKey string, date string, region string, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// CalculateSignature calculates the signature
// Parameters:
//   - signingKey: The derived signing key from DeriveSigningKey function
//   - stringToSign: The string to sign built from BuildStringToSign function
//
// Returns: string - The calculated signature as a hexadecimal string
// Example usage:
//
//	signature := CalculateSignature(signingKey, stringToSign)
func CalculateSignature(signingKey []byte, stringToSign string) string {
	return hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
}

// VerifySignature verifies the provided signature matches the calculated one
// Parameters:
//   - r: The HTTP request to verify
//   - body: The request body as a byte slice (used for payload hash calculation)
//   - auth: The parsed AuthorizationHeader struct containing signature details
//   - secretKey: The AWS secret access key for signature verification
//   - expectedRegion: The expected AWS region to validate against the credential
//
// Returns: error - An error if verification fails (e.g., signature mismatch, invalid headers), or nil if verification succeeds
// Example usage:
//
//	err := VerifySignature(r, bodyBytes, authHeader, secretKey, expectedRegion)
//	if err != nil {
//		// Handle error (e.g., return 403 Forbidden)
//	}
func VerifySignature(r *http.Request, body []byte, auth *AuthorizationHeader, secretKey string, expectedRegion string) error {
	// Validate region
	if auth.Region != expectedRegion {
		return fmt.Errorf("region mismatch: expected %s, got %s", expectedRegion, auth.Region)
	}

	// Validate service
	if auth.Service != "s3" {
		return fmt.Errorf("invalid service: expected s3, got %s", auth.Service)
	}

	// Get timestamp from X-Amz-Date header
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return fmt.Errorf("missing X-Amz-Date header")
	}

	timestamp, err := time.Parse(TimeFormat, amzDate)
	if err != nil {
		return fmt.Errorf("invalid X-Amz-Date format: %w", err)
	}

	// Check request age
	age := time.Since(timestamp)
	if age < 0 {
		age = -age
	}
	if age > MaxRequestAge {
		return fmt.Errorf("request time too skewed: %v", age)
	}

	// Validate date from credential matches X-Amz-Date
	expectedDate := timestamp.UTC().Format(DateFormat)
	if auth.Date != expectedDate {
		return fmt.Errorf("credential date mismatch: expected %s, got %s", expectedDate, auth.Date)
	}

	// Determine payload hash for signature verification
	contentSHA256 := r.Header.Get("X-Amz-Content-SHA256")
	var payloadHash string

	switch contentSHA256 {
	case "UNSIGNED-PAYLOAD":
		// Client explicitly requested unsigned payload
		payloadHash = "UNSIGNED-PAYLOAD"

	case "":
		// No hash provided (UI uploads) - calculate from body
		payloadHash = sha256Hex(body)

	default:
		// Client provided hash - validate for small files on direct connections
		// Skip validation for: 1) proxied requests (body modified), 2) large files (body empty)
		if !IsThroughProxy(r) && len(body) > 0 {
			calculatedHash := sha256Hex(body)
			if contentSHA256 != calculatedHash {
				return fmt.Errorf("payload hash mismatch")
			}
		}
		// Use client-provided hash for signature calculation (what client signed)
		payloadHash = contentSHA256
	}

	// Build canonical request
	canonicalRequest := BuildCanonicalRequest(r, auth.SignedHeaders, payloadHash)

	// Build string to sign
	stringToSign := BuildStringToSign(timestamp, auth.CredentialScope(), canonicalRequest)

	// Derive signing key
	signingKey := DeriveSigningKey(secretKey, auth.Date, auth.Region, auth.Service)

	// Calculate expected signature
	expectedSignature := CalculateSignature(signingKey, stringToSign)

	// Compare signatures using constant-time comparison
	if !hmac.Equal([]byte(expectedSignature), []byte(auth.Signature)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// hmacSHA256 calculates the HMAC-SHA256 of the data using the provided key
// Parameters:
//   - key: The secret key to use for HMAC calculation
//   - data: The data to be hashed
//
// Returns: []byte - The resulting HMAC-SHA256 hash as a byte slice
// Example usage:
//
//	hmacHash := hmacSHA256(signingKey, []byte(stringToSign))
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// sha256Hex calculates the SHA256 hash of the data and returns it as a hexadecimal string
// Parameters:
//   - data: The data to be hashed
//
// Returns: string - The SHA256 hash of the data as a hexadecimal string
// Example usage:
//
//	hash := sha256Hex(bodyBytes)
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
