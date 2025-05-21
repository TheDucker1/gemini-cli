package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// New helper function
func readFileOrString(pathOrString string) (string, error) {
	if strings.HasPrefix(pathOrString, "@") {
		filePath := strings.TrimPrefix(pathOrString, "@")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file '%s': %w", filePath, err)
		}
		return string(data), nil
	}
	// If not starting with @, assume it's a direct string (JSON content)
	return pathOrString, nil
}

func processFileArgument(arg string) (mimeType string, base64Data string, err error) {
	if strings.HasPrefix(arg, "@") {
		filePath := strings.TrimPrefix(arg, "@")
		return readFileAsBase64(filePath)
	} else if strings.HasPrefix(arg, "file://") {
		parsedURL, err := url.Parse(arg)
		if err != nil {
			return "", "", fmt.Errorf("invalid file URI '%s': %w", arg, err)
		}
		filePath := parsedURL.Path
		if os.PathSeparator == '\\' && strings.HasPrefix(filePath, "/") {
			filePath = strings.TrimPrefix(filePath, "/")
		}
		return readFileAsBase64(filePath)
	} else if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		return readURLAsBase64(arg)
	} else if strings.HasPrefix(arg, "data:") {
		return parseDataURI(arg)
	}
	return "", "", fmt.Errorf("unsupported file argument format: %s. Use @/path, file://, http(s)://, or data:", arg)
}

func readFileAsBase64(filePath string) (mimeType string, base64Data string, err error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file '%s': %w", filePath, err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".txt":
		mimeType = "text/plain"
	case ".json":
		mimeType = "application/json"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".heic":
		mimeType = "image/heic"
	case ".heif":
		mimeType = "image/heif"
	case ".pdf":
		mimeType = "application/pdf"
	case ".mp3":
		mimeType = "audio/mpeg"
	case ".wav":
		mimeType = "audio/wav"
	case ".mp4":
		mimeType = "video/mp4"
	default:
		mimeType = http.DetectContentType(data)
		if mimeType == "application/octet-stream" { // If still generic, give a better generic default
			mimeType = "application/octet-stream"
		}
	}

	base64Data = base64.StdEncoding.EncodeToString(data)
	return mimeType, base64Data, nil
}

func readURLAsBase64(fileURL string) (mimeType string, base64Data string, err error) {
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch URL '%s': %w", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to fetch URL '%s': status %s", fileURL, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body from URL '%s': %w", fileURL, err)
	}

	mimeType = resp.Header.Get("Content-Type")
	// Try to refine if generic or missing
	if mimeType == "" || mimeType == "application/octet-stream" || !strings.Contains(mimeType, "/") {
		parsedURL, _ := url.Parse(fileURL)
		ext := strings.ToLower(filepath.Ext(parsedURL.Path))
		pathMime := ""
		switch ext {
		case ".jpg", ".jpeg":
			pathMime = "image/jpeg"
		case ".png":
			pathMime = "image/png"
		case ".gif":
			pathMime = "image/gif"
		case ".webp":
			pathMime = "image/webp"
		case ".pdf":
			pathMime = "application/pdf"
		case ".txt":
			pathMime = "text/plain"
			// Add more common types
		}
		if pathMime != "" {
			mimeType = pathMime
		} else {
			detectedMime := http.DetectContentType(data)
			if detectedMime != "application/octet-stream" {
				mimeType = detectedMime
			} else if mimeType == "" { // if original mimeType was empty and detection is octet-stream
				mimeType = "application/octet-stream" // final fallback
			}
			// if original mimeType was octet-stream and detection is also octet-stream, it stays octet-stream
		}
	}

	base64Data = base64.StdEncoding.EncodeToString(data)
	return mimeType, base64Data, nil
}

func parseDataURI(dataURI string) (mimeType string, base64Data string, err error) {
	parts := strings.SplitN(dataURI, ",", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid data URI format: missing comma separator")
	}
	header := parts[0]
	base64Data = parts[1]

	if !strings.HasPrefix(header, "data:") {
		return "", "", fmt.Errorf("invalid data URI format: must start with 'data:'")
	}
	headerContent := strings.TrimPrefix(header, "data:")

	encodingParts := strings.SplitN(headerContent, ";", 2)
	mimeType = encodingParts[0]
	if mimeType == "" {
		// As per RFC 2397, default is text/plain;charset=US-ASCII
		// For simplicity, we'll just say text/plain if it's empty.
		// The Gemini API usually requires a more specific image/audio/video type for media.
		mimeType = "text/plain"
	}

	isBase64Encoded := false
	if len(encodingParts) > 1 && strings.ToLower(encodingParts[1]) == "base64" {
		isBase64Encoded = true
	}

	if !isBase64Encoded {
		return "", "", fmt.Errorf("data URI not specified as base64 encoded (e.g., data:image/png;base64,...). Only base64 is supported for file data.")
	}

	_, err = base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", "", fmt.Errorf("invalid base64 data in data URI: %w", err)
	}

	return mimeType, base64Data, nil
}
