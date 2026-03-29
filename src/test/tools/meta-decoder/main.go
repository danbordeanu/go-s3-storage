package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"s3-storage/model"
)

func main() {
	// Define flags
	metaFile := flag.String("meta", "", "Path to .meta file (bucket metadata)")
	xlMetaFile := flag.String("xl", "", "Path to xl.meta file (object metadata)")
	shareFile := flag.String("share", "", "Path to .shares file (share links)")
	jsonOutput := flag.Bool("json", false, "Output only JSON (no human-readable format)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "meta-decoder - Decode S3 storage metadata files\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  meta-decoder -meta <path>    Decode .meta file (bucket metadata)\n")
		fmt.Fprintf(os.Stderr, "  meta-decoder -xl <path>      Decode xl.meta file (object metadata)\n")
		fmt.Fprintf(os.Stderr, "  meta-decoder -share <path>   Decode .shares file (share links)\n")
		fmt.Fprintf(os.Stderr, "  meta-decoder -meta <path> -json   Output as JSON only\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  meta-decoder -meta /data/.meta\n")
		fmt.Fprintf(os.Stderr, "  meta-decoder -xl /data/my-bucket/photos/image.png/xl.meta\n")
		fmt.Fprintf(os.Stderr, "  meta-decoder -share /data/.shares\n")
		fmt.Fprintf(os.Stderr, "  meta-decoder -meta /data/.meta -json\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Count specified flags
	flagCount := 0
	if *metaFile != "" {
		flagCount++
	}
	if *xlMetaFile != "" {
		flagCount++
	}
	if *shareFile != "" {
		flagCount++
	}

	// Validate input
	if flagCount == 0 {
		fmt.Fprintln(os.Stderr, "Error: must specify one of -meta, -xl, or -share flags")
		flag.Usage()
		os.Exit(1)
	}

	if flagCount > 1 {
		fmt.Fprintln(os.Stderr, "Error: cannot specify multiple file type flags")
		flag.Usage()
		os.Exit(1)
	}

	// Decode the appropriate file type
	if *metaFile != "" {
		decodeMetaFile(*metaFile, *jsonOutput)
	} else if *xlMetaFile != "" {
		decodeXLMetaFile(*xlMetaFile, *jsonOutput)
	} else {
		decodeShareFile(*shareFile, *jsonOutput)
	}
}

func decodeMetaFile(path string, jsonOnly bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var metadata model.MetaData
	_, err = metadata.UnmarshalMsg(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding msgpack: %v\n", err)
		os.Exit(1)
	}

	if jsonOnly {
		outputJSON(metadata)
		return
	}

	// Human-readable output
	fmt.Println("=== .meta File Contents ===")
	fmt.Printf("File:       %s\n", path)
	fmt.Printf("Version:    %d\n", metadata.Version)
	fmt.Printf("Updated At: %s\n", time.Unix(metadata.UpdatedAt, 0).Format(time.RFC3339))
	fmt.Println()

	fmt.Printf("=== Buckets (%d) ===\n", len(metadata.Buckets))
	for i, bucket := range metadata.Buckets {
		fmt.Printf("\n[%d] %s\n", i+1, bucket.Name)
		fmt.Printf("    Created:      %s\n", time.Unix(bucket.CreationDate, 0).Format(time.RFC3339))
		fmt.Printf("    Total Size:   %s (%d bytes)\n", formatBytes(bucket.TotalSize), bucket.TotalSize)
		fmt.Printf("    Object Count: %d\n", bucket.ObjectCount)
	}

	if len(metadata.Multiparts) > 0 {
		fmt.Printf("\n=== Multipart Uploads (%d) ===\n", len(metadata.Multiparts))
		for i, mp := range metadata.Multiparts {
			fmt.Printf("\n[%d] Upload ID: %s\n", i+1, mp.UploadID)
			fmt.Printf("    Bucket:    %s\n", mp.Bucket)
			fmt.Printf("    Key:       %s\n", mp.Key)
			fmt.Printf("    Initiated: %s\n", time.Unix(mp.Initiated, 0).Format(time.RFC3339))
		}
	}

	if len(metadata.Healing) > 0 {
		fmt.Printf("\n=== Healing Locks (%d) ===\n", len(metadata.Healing))
		for i, h := range metadata.Healing {
			fmt.Printf("\n[%d] ID: %s\n", i+1, h.ID)
			fmt.Printf("    Path:     %s\n", h.Path)
			fmt.Printf("    Acquired: %s\n", time.Unix(h.AcquiredAt, 0).Format(time.RFC3339))
			fmt.Printf("    Expires:  %s\n", time.Unix(h.ExpiresAt, 0).Format(time.RFC3339))
		}
	}

	fmt.Println("\n=== JSON ===")
	outputJSON(metadata)
}

func decodeXLMetaFile(path string, jsonOnly bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var objMeta model.ObjectMeta
	_, err = objMeta.UnmarshalMsg(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding msgpack: %v\n", err)
		os.Exit(1)
	}

	if jsonOnly {
		outputJSON(objMeta)
		return
	}

	// Human-readable output
	fmt.Println("=== xl.meta File Contents ===")
	fmt.Printf("File:          %s\n", path)
	fmt.Printf("Version:       %d\n", objMeta.Version)
	fmt.Printf("Size:          %s (%d bytes)\n", formatBytes(objMeta.Size), objMeta.Size)
	fmt.Printf("ETag:          %s\n", objMeta.ETag)
	fmt.Printf("Last Modified: %s\n", time.Unix(objMeta.LastModified, 0).Format(time.RFC3339))
	fmt.Printf("Content-Type:  %s\n", objMeta.ContentType)
	fmt.Printf("Disk UUID:     %s\n", objMeta.DiskUUID)

	fmt.Printf("\n=== Parts (%d) ===\n", len(objMeta.Parts))
	for i, part := range objMeta.Parts {
		fmt.Printf("\n[%d] Part %d\n", i+1, part.Number)
		fmt.Printf("    Size: %s (%d bytes)\n", formatBytes(part.Size), part.Size)
		fmt.Printf("    ETag: %s\n", part.ETag)
	}

	fmt.Println("\n=== JSON ===")
	outputJSON(objMeta)
}

func decodeShareFile(path string, jsonOnly bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var shareStore model.ShareLinkStore
	_, err = shareStore.UnmarshalMsg(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding msgpack: %v\n", err)
		os.Exit(1)
	}

	if jsonOnly {
		outputJSON(shareStore)
		return
	}

	// Human-readable output
	fmt.Println("=== .shares File Contents ===")
	fmt.Printf("File:    %s\n", path)
	fmt.Printf("Version: %d\n", shareStore.Version)
	fmt.Println()

	fmt.Printf("=== Share Links (%d) ===\n", len(shareStore.Links))
	now := time.Now().Unix()

	for i, link := range shareStore.Links {
		fmt.Printf("\n[%d] Token: %s\n", i+1, link.Token)
		fmt.Printf("    Bucket:     %s\n", link.Bucket)
		fmt.Printf("    Key:        %s\n", link.Key)
		fmt.Printf("    Created At: %s\n", time.Unix(link.CreatedAt, 0).Format(time.RFC3339))

		if link.ExpiresAt > 0 {
			expiresTime := time.Unix(link.ExpiresAt, 0)
			fmt.Printf("    Expires At: %s\n", expiresTime.Format(time.RFC3339))

			// Show status
			if now > link.ExpiresAt {
				fmt.Printf("    Status:     EXPIRED\n")
			} else {
				duration := time.Until(expiresTime)
				fmt.Printf("    Status:     Active (expires in %s)\n", formatDuration(duration))
			}
		} else {
			fmt.Printf("    Expires At: Never\n")
			fmt.Printf("    Status:     Active (no expiration)\n")
		}
	}

	fmt.Println("\n=== JSON ===")
	outputJSON(shareStore)
}

func outputJSON(v any) {
	jsonData, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonData))
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
