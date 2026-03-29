#!/bin/bash
#
# AWS SigV4 Curl Helper Script
# Generates and executes signed curl requests for S3-compatible APIs
#
# Usage:
#   ./sigv4-curl.sh <method> <path> [options]
#
# Examples:
#   ./sigv4-curl.sh GET /                    # List buckets
#   ./sigv4-curl.sh PUT /my-bucket           # Create bucket
#   ./sigv4-curl.sh DELETE /my-bucket        # Delete bucket
#   ./sigv4-curl.sh HEAD /my-bucket          # Check bucket exists
#
# Environment variables:
#   S3_ACCESS_KEY_ID     - Access key (default: testkey)
#   S3_SECRET_ACCESS_KEY - Secret key (default: testsecret)
#   S3_ENDPOINT          - Endpoint URL (default: http://localhost:8089)
#   S3_REGION            - AWS region (default: us-east-1)
#

set -e

# Configuration (can be overridden by environment variables)
ACCESS_KEY="${S3_ACCESS_KEY_ID:-testkey}"
SECRET_KEY="${S3_SECRET_ACCESS_KEY:-testsecret}"
ENDPOINT="${S3_ENDPOINT:-http://localhost:8089}"
REGION="${S3_REGION:-us-east-1}"
SERVICE="s3"

# Parse arguments
METHOD="${1:-GET}"
REQUEST_PATH="${2:-/}"

# Extract host from endpoint
PROTOCOL=$(echo "$ENDPOINT" | grep -oE '^https?')
HOST=$(echo "$ENDPOINT" | sed -E 's|^https?://||' | sed -E 's|/.*$||')
BASE_PATH=$(echo "$ENDPOINT" | sed -E 's|^https?://[^/]+||')

# Full path
FULL_PATH="${BASE_PATH}${REQUEST_PATH}"

# Timestamps
AMZ_DATE=$(date -u +"%Y%m%dT%H%M%SZ")
DATE_STAMP=$(date -u +"%Y%m%d")

# Calculate payload hash (empty payload for these requests)
PAYLOAD=""
PAYLOAD_HASH=$(echo -n "$PAYLOAD" | openssl dgst -sha256 | awk '{print $NF}')

# Create canonical request
CANONICAL_URI="$FULL_PATH"
CANONICAL_QUERYSTRING=""
CANONICAL_HEADERS="host:${HOST}\nx-amz-content-sha256:${PAYLOAD_HASH}\nx-amz-date:${AMZ_DATE}\n"
SIGNED_HEADERS="host;x-amz-content-sha256;x-amz-date"

CANONICAL_REQUEST="${METHOD}
${CANONICAL_URI}
${CANONICAL_QUERYSTRING}
host:${HOST}
x-amz-content-sha256:${PAYLOAD_HASH}
x-amz-date:${AMZ_DATE}

${SIGNED_HEADERS}
${PAYLOAD_HASH}"

# Create string to sign
ALGORITHM="AWS4-HMAC-SHA256"
CREDENTIAL_SCOPE="${DATE_STAMP}/${REGION}/${SERVICE}/aws4_request"
CANONICAL_REQUEST_HASH=$(echo -n "$CANONICAL_REQUEST" | openssl dgst -sha256 | awk '{print $NF}')

STRING_TO_SIGN="${ALGORITHM}
${AMZ_DATE}
${CREDENTIAL_SCOPE}
${CANONICAL_REQUEST_HASH}"

# Calculate signing key
hmac_sha256() {
    key="$1"
    data="$2"
    echo -n "$data" | openssl dgst -sha256 -mac HMAC -macopt "$key" | awk '{print $NF}'
}

hmac_sha256_hex() {
    key="$1"
    data="$2"
    echo -n "$data" | openssl dgst -sha256 -mac HMAC -macopt "hexkey:$key" | awk '{print $NF}'
}

# Derive signing key using HMAC chain
K_DATE=$(echo -n "$DATE_STAMP" | openssl dgst -sha256 -mac HMAC -macopt "key:AWS4${SECRET_KEY}" | awk '{print $NF}')
K_REGION=$(hmac_sha256_hex "$K_DATE" "$REGION")
K_SERVICE=$(hmac_sha256_hex "$K_REGION" "$SERVICE")
K_SIGNING=$(hmac_sha256_hex "$K_SERVICE" "aws4_request")

# Calculate signature
SIGNATURE=$(hmac_sha256_hex "$K_SIGNING" "$STRING_TO_SIGN")

# Create authorization header
AUTHORIZATION="${ALGORITHM} Credential=${ACCESS_KEY}/${CREDENTIAL_SCOPE}, SignedHeaders=${SIGNED_HEADERS}, Signature=${SIGNATURE}"

# Build and execute curl command
CURL_CMD="curl -X ${METHOD} \
  -H \"Authorization: ${AUTHORIZATION}\" \
  -H \"X-Amz-Date: ${AMZ_DATE}\" \
  -H \"X-Amz-Content-SHA256: ${PAYLOAD_HASH}\" \
  -H \"Host: ${HOST}\" \
  \"${PROTOCOL}://${HOST}${FULL_PATH}\""

echo "=== Request ===" >&2
echo "Method: ${METHOD}" >&2
echo "URL: ${PROTOCOL}://${HOST}${FULL_PATH}" >&2
echo "Access Key: ${ACCESS_KEY}" >&2
echo "Region: ${REGION}" >&2
echo "" >&2
echo "=== Executing ===" >&2
echo "$CURL_CMD" >&2
echo "" >&2
echo "=== Response ===" >&2

# Execute
curl -s -X "${METHOD}" \
  -H "Authorization: ${AUTHORIZATION}" \
  -H "X-Amz-Date: ${AMZ_DATE}" \
  -H "X-Amz-Content-SHA256: ${PAYLOAD_HASH}" \
  -H "Host: ${HOST}" \
  "${PROTOCOL}://${HOST}${FULL_PATH}"

echo "" >&2
