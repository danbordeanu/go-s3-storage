# meta-decoder

A command-line utility for decoding S3-storage metadata files. This tool decodes MessagePack (msgpack) encoded metadata files into human-readable format and JSON.

## Purpose

The S3-storage system stores metadata in three types of files:
- **`.meta`** - Bucket metadata (list of buckets, sizes, object counts)
- **`xl.meta`** - Object metadata (size, ETag/SHA256, content-type, timestamps)
- **`.shares`** - Share link metadata (temporary URLs with expiration)

These files are encoded using [MessagePack](https://msgpack.org/) for efficient storage. The `meta-decoder` tool allows you to inspect and debug these files during development or troubleshooting.

## Building

```shell
cd src/test/tools/meta-decoder
go build -o meta-decoder main.go
```

## Usage

```shell
meta-decoder -meta <path>    # Decode .meta file (bucket metadata)
meta-decoder -xl <path>      # Decode xl.meta file (object metadata)
meta-decoder -share <path>   # Decode .shares file (share links)
meta-decoder -meta <path> -json   # Output as JSON only
```

### Flags

| Flag     | Description                                      | Required |
|----------|--------------------------------------------------|----------|
| `-meta`  | Path to `.meta` file (bucket metadata)          | One of these three |
| `-xl`    | Path to `xl.meta` file (object metadata)        | must be specified |
| `-share` | Path to `.shares` file (share links)            | |
| `-json`  | Output only JSON (no human-readable format)     | No |

__!!!NB!!!__ You can only specify one file type flag at a time.

## Examples

### Decode Bucket Metadata

```shell
# Human-readable output with JSON
meta-decoder -meta /data/.meta

# Output:
=== .meta File Contents ===
File:       /data/.meta
Version:    1
Updated At: 2024-01-15T10:30:00Z

=== Buckets (2) ===

[1] my-bucket
    Created:      2024-01-15T10:00:00Z
    Total Size:   1.23 GB (1234567890 bytes)
    Object Count: 42

[2] photos
    Created:      2024-01-15T10:15:00Z
    Total Size:   500.00 MB (524288000 bytes)
    Object Count: 120

=== JSON ===
{
  "version": 1,
  "updated_at": 1705315800,
  "buckets": [
    {
      "name": "my-bucket",
      "creation_date": 1705314000,
      "total_size": 1234567890,
      "object_count": 42
    }
  ]
}
```

### Decode Object Metadata

```shell
# Decode xl.meta for a specific object
meta-decoder -xl /data/my-bucket/photos/vacation.jpg/xl.meta

# Output:
=== xl.meta File Contents ===
File:          /data/my-bucket/photos/vacation.jpg/xl.meta
Version:       1
Size:          2.34 MB (2453264 bytes)
ETag:          a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456
Last Modified: 2024-01-15T14:30:00Z
Content-Type:  image/jpeg
Disk UUID:     550e8400-e29b-41d4-a716-446655440000

=== Parts (1) ===

[1] Part 1
    Size: 2.34 MB (2453264 bytes)
    ETag: a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456

=== JSON ===
{
  "version": 1,
  "size": 2453264,
  "etag": "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
  "last_modified": 1705329000,
  "content_type": "image/jpeg",
  "disk_uuid": "550e8400-e29b-41d4-a716-446655440000",
  "parts": [
    {
      "number": 1,
      "size": 2453264,
      "etag": "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"
    }
  ]
}
```

### Decode Share Links

```shell
# Decode share links database
meta-decoder -share /data/.shares

# Output:
=== .shares File Contents ===
File:    /data/.shares
Version: 1

=== Share Links (2) ===

[1] Token: abc123def456ghi789
    Bucket:     my-bucket
    Key:        photos/vacation.jpg
    Created At: 2024-01-15T14:00:00Z
    Expires At: 2024-01-16T14:00:00Z
    Status:     Active (expires in 23h 30m 0s)

[2] Token: xyz789abc123def456
    Bucket:     documents
    Key:        report.pdf
    Created At: 2024-01-10T10:00:00Z
    Expires At: Never
    Status:     Active (no expiration)

=== JSON ===
{
  "version": 1,
  "links": [
    {
      "token": "abc123def456ghi789",
      "bucket": "my-bucket",
      "key": "photos/vacation.jpg",
      "created_at": 1705327200,
      "expires_at": 1705413600
    }
  ]
}
```

### JSON-Only Output

Useful for piping to `jq` or other JSON processing tools:

```shell
# Get JSON only
meta-decoder -meta /data/.meta -json

# Pipe to jq for filtering
meta-decoder -meta /data/.meta -json | jq '.buckets[] | select(.name == "my-bucket")'

# Count total objects across all buckets
meta-decoder -meta /data/.meta -json | jq '[.buckets[].object_count] | add'

# List all buckets with sizes
meta-decoder -meta /data/.meta -json | jq -r '.buckets[] | "\(.name): \(.total_size) bytes"'
```

## File Locations

The metadata files are stored in the S3-storage data directory:

```
/data/
├── .meta                    # Global bucket metadata (decode with -meta)
├── .shares                  # Share links database (decode with -share)
├── my-bucket/
│   ├── .bucket.meta         # Bucket-specific metadata
│   └── photos/
│       └── vacation.jpg/
│           ├── xl.meta      # Object metadata (decode with -xl)
│           └── part.1       # Actual object data (binary)
```

## Use Cases

### Development & Debugging

```shell
# Check if bucket metadata is corrupt
meta-decoder -meta /data/.meta

# Verify object ETag/SHA256
meta-decoder -xl /data/my-bucket/file.txt/xl.meta

# Check share link expiration
meta-decoder -share /data/.shares
```

### Troubleshooting

```shell
# Find why bucket sizes don't match
meta-decoder -meta /data/.meta -json | jq '.buckets[] | {name, total_size, object_count}'

# Verify content-type detection
meta-decoder -xl /data/bucket/file.dat/xl.meta | grep "Content-Type"

# List expired share links
meta-decoder -share /data/.shares | grep "EXPIRED"
```

### Backup Verification

```shell
# Export metadata to JSON for backup
meta-decoder -meta /data/.meta -json > metadata-backup.json

# Verify backup integrity
diff <(meta-decoder -meta /data/.meta -json) metadata-backup.json
```

### Automation & Scripts

```shell
#!/bin/bash
# Check if any bucket exceeds 1GB

BUCKETS=$(meta-decoder -meta /data/.meta -json | jq -r '.buckets[] | select(.total_size > 1073741824) | .name')

if [ -n "$BUCKETS" ]; then
    echo "Buckets over 1GB:"
    echo "$BUCKETS"
fi
```

## Error Handling

The tool will exit with code 1 if:
- File cannot be read
- File format is invalid (not valid msgpack)
- Multiple file type flags are specified
- No file type flag is specified

```shell
# Example error
meta-decoder -meta /nonexistent/file
# Output: Error reading file: open /nonexistent/file: no such file or directory

# Invalid msgpack
meta-decoder -xl /data/bucket/file.txt/part.1
# Output: Error decoding msgpack: msgpack: invalid code
```

## Output Formats

### Human-Readable Format

The default output includes:
- Header with file path and version
- Formatted sections (buckets, objects, share links)
- Human-readable sizes (KB, MB, GB)
- Formatted timestamps (RFC3339)
- Status indicators (EXPIRED, Active)
- JSON dump at the end

### JSON-Only Format

With the `-json` flag, only valid JSON is output (no headers or formatting), making it suitable for:
- Piping to `jq`
- Parsing in scripts
- API integration
- Automated testing

## Dependencies

The tool depends on the S3-storage model package:
- `s3-storage/model` - Defines the metadata structures

Structures used:
- `MetaData` - Global metadata (`.meta` files)
- `ObjectMeta` - Object metadata (`xl.meta` files)
- `ShareLinkStore` - Share links (`.shares` files)

## Building from Source

```shell
# From project root
cd src/test/tools/meta-decoder

# Build
go build -o meta-decoder main.go

# Install globally (optional)
go install

# Run tests (if any)
go test ./...
```

## Integration with S3-Storage

The metadata files are automatically created and updated by the S3-storage server:

- **`.meta`** - Updated when buckets are created/deleted or object counts change
- **`xl.meta`** - Created when objects are uploaded via PutObject
- **`.shares`** - Updated when share links are created via the Web UI

This tool is read-only and does not modify metadata files.

## Common Tasks

### Find Large Objects

```shell
# Find all objects over 100MB
find /data -name "xl.meta" -exec sh -c '
    SIZE=$(meta-decoder -xl "$1" -json | jq -r ".size")
    if [ "$SIZE" -gt 104857600 ]; then
        echo "$1: $(($SIZE / 1048576)) MB"
    fi
' sh {} \;
```

### Audit Share Links

```shell
# List all active share links
meta-decoder -share /data/.shares | grep -A 5 "Status:     Active"

# Count expired links
meta-decoder -share /data/.shares | grep -c "Status:     EXPIRED"
```

### Export Bucket Statistics

```shell
# Create CSV of bucket statistics
echo "Bucket,Size (bytes),Objects" > buckets.csv
meta-decoder -meta /data/.meta -json | jq -r '.buckets[] | [.name, .total_size, .object_count] | @csv' >> buckets.csv
```

## Troubleshooting

### Error: "invalid msgpack"

**Cause**: File is not a valid msgpack file or is corrupted.

**Solution**:
- Verify the file path is correct
- Check if the file is a metadata file (not a data file like `part.1`)
- Try restoring from backup if corrupted

### Error: "no such file or directory"

**Cause**: File path is incorrect or file doesn't exist.

**Solution**:
- Check the file path
- Ensure the S3-storage server has created the metadata files
- For object metadata, remember the path includes the object name as a directory

### Unexpected Output Format

**Cause**: Using an old version of the tool with new metadata format.

**Solution**: Rebuild the tool to match the current S3-storage version:
```shell
cd src/test/tools/meta-decoder
go build -o meta-decoder main.go
```

## Version Compatibility

The tool must be built from the same codebase as the S3-storage server to ensure metadata structure compatibility. If the metadata format changes, rebuild the decoder.

Current supported metadata versions:
- `.meta` - Version 1
- `xl.meta` - Version 1
- `.shares` - Version 1

## License

Copyright © 2024 Almeria Industries

Part of the S3-storage project.

## See Also

- Main S3-storage README: `/Users/dan/go/src/go-s3-storage/README.md`
- Model definitions: `/Users/dan/go/src/go-s3-storage/src/model/`
- MessagePack specification: https://msgpack.org/
