package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// FilesAPIConfig carries authentication and endpoint settings for the
// Anthropic Public Files API.
type FilesAPIConfig struct {
	// OAuthToken is the bearer token for authentication (from session JWT).
	OAuthToken string
	// BaseURL optionally overrides the default API host.
	BaseURL string
	// HTTPClient allows tests to inject a local transport.
	HTTPClient *http.Client
}

// File represents a file attachment parsed from CLI arguments.
type File struct {
	FileID        string
	RelativePath  string
}

// DownloadResult reports the outcome of a single file download.
type DownloadResult struct {
	FileID       string
	Path         string
	Success      bool
	Error        string
	BytesWritten int64
}

// UploadResult reports the outcome of a single file upload.
type UploadResult struct {
	Path    string
	FileID  string
	Size    int64
	Success bool
	Error   string
}

// FileMetadata describes a file returned by the list endpoint.
type FileMetadata struct {
	Filename string
	FileID   string
	Size     int64
}

const (
	filesAPIBetaHeader   = "files-api-2025-04-14,oauth-2025-04-20"
	anthropicAPIVersion  = "2023-06-01"
	maxRetries           = 3
	baseDelayMs          = 500
	maxFileSizeBytes     = 500 * 1024 * 1024 // 500MB
	defaultConcurrency   = 5
)

func defaultFilesAPIBaseURL() string {
	if u := os.Getenv("ANTHROPIC_BASE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	if u := os.Getenv("CLAUDE_CODE_API_BASE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://api.anthropic.com"
}

func (c *FilesAPIConfig) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return defaultFilesAPIBaseURL()
}

func (c *FilesAPIConfig) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *FilesAPIConfig) authHeader() string {
	return "Bearer " + c.OAuthToken
}

func (c *FilesAPIConfig) defaultHeaders() http.Header {
	h := make(http.Header)
	h.Set("Authorization", c.authHeader())
	h.Set("anthropic-version", anthropicAPIVersion)
	h.Set("anthropic-beta", filesAPIBetaHeader)
	return h
}

// retryResult signals whether a retry operation succeeded.
type retryResult struct {
	Done  bool
	Value []byte
	Error string
}

// retryWithBackoff executes an operation with exponential backoff.
func retryWithBackoff(operation string, attemptFn func(attempt int) retryResult) ([]byte, error) {
	var lastErr string
	for attempt := 1; attempt <= maxRetries; attempt++ {
		result := attemptFn(attempt)
		if result.Done {
			return result.Value, nil
		}
		lastErr = result.Error
		if attempt < maxRetries {
			delay := baseDelayMs * (1 << (attempt - 1))
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}
	return nil, fmt.Errorf("%s after %d attempts: %s", operation, maxRetries, lastErr)
}

// DownloadFile downloads a single file from the Anthropic Public Files API.
func (c *FilesAPIConfig) DownloadFile(fileID string) ([]byte, error) {
	url := c.baseURL() + "/v1/files/" + fileID + "/content"

	return retryWithBackoff("download file "+fileID, func(attempt int) retryResult {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return retryResult{Error: err.Error()}
		}
		req.Header = c.defaultHeaders()

		resp, err := c.httpClient().Do(req)
		if err != nil {
			return retryResult{Error: err.Error()}
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return retryResult{Error: err.Error()}
			}
			return retryResult{Done: true, Value: data}
		case http.StatusNotFound:
			return retryResult{Done: false, Error: "file not found: " + fileID}
		case http.StatusUnauthorized:
			return retryResult{Done: false, Error: "authentication failed"}
		case http.StatusForbidden:
			return retryResult{Done: false, Error: "access denied: " + fileID}
		default:
			return retryResult{Error: fmt.Sprintf("status %d", resp.StatusCode)}
		}
	})
}

// DownloadAndSaveFile downloads a file and writes it to disk.
func (c *FilesAPIConfig) DownloadAndSaveFile(file File, basePath string) DownloadResult {
	data, err := c.DownloadFile(file.FileID)
	if err != nil {
		return DownloadResult{
			FileID:  file.FileID,
			Path:    "",
			Success: false,
			Error:   err.Error(),
		}
	}

	fullPath := path.Join(basePath, file.RelativePath)
	dir := path.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return DownloadResult{
			FileID:  file.FileID,
			Path:    fullPath,
			Success: false,
			Error:   err.Error(),
		}
	}

	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return DownloadResult{
			FileID:  file.FileID,
			Path:    fullPath,
			Success: false,
			Error:   err.Error(),
		}
	}

	return DownloadResult{
		FileID:       file.FileID,
		Path:         fullPath,
		Success:      true,
		BytesWritten: int64(len(data)),
	}
}

// DownloadFiles downloads multiple files concurrently with a concurrency limit.
func (c *FilesAPIConfig) DownloadFiles(files []File, basePath string, concurrency int) []DownloadResult {
	if len(files) == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}

	results := make([]DownloadResult, len(files))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, f := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, file File) {
			defer wg.Done()
			results[idx] = c.DownloadAndSaveFile(file, basePath)
			<-sem
		}(i, f)
	}
	wg.Wait()
	return results
}

// UploadFile uploads a single file to the Files API (BYOC mode).
func (c *FilesAPIConfig) UploadFile(filePath, relativePath string) UploadResult {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return UploadResult{
			Path:    relativePath,
			Success: false,
			Error:   err.Error(),
		}
	}

	if int64(len(content)) > maxFileSizeBytes {
		return UploadResult{
			Path:    relativePath,
			Success: false,
			Error:   fmt.Sprintf("file exceeds maximum size of %d bytes", maxFileSizeBytes),
		}
	}

	filename := path.Base(relativePath)
	body, contentType, err := buildMultipartBody(filename, content)
	if err != nil {
		return UploadResult{
			Path:    relativePath,
			Success: false,
			Error:   err.Error(),
		}
	}

	url := c.baseURL() + "/v1/files"
	data, err := retryWithBackoff("upload file "+relativePath, func(attempt int) retryResult {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return retryResult{Error: err.Error()}
		}
		req.Header = c.defaultHeaders()
		req.Header.Set("Content-Type", contentType)

		resp, err := c.httpClient().Do(req)
		if err != nil {
			return retryResult{Error: err.Error()}
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated:
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return retryResult{Error: err.Error()}
			}
			return retryResult{Done: true, Value: data}
		case http.StatusUnauthorized:
			return retryResult{Done: false, Error: "authentication failed"}
		case http.StatusForbidden:
			return retryResult{Done: false, Error: "access denied"}
		case http.StatusRequestEntityTooLarge:
			return retryResult{Done: false, Error: "file too large"}
		default:
			return retryResult{Error: fmt.Sprintf("status %d", resp.StatusCode)}
		}
	})
	if err != nil {
		return UploadResult{
			Path:    relativePath,
			Success: false,
			Error:   err.Error(),
		}
	}

	var resp struct {
		ID   string `json:"id"`
		Size int64  `json:"size_bytes"`
	}
	if jsonErr := json.Unmarshal(data, &resp); jsonErr != nil {
		return UploadResult{
			Path:    relativePath,
			Success: false,
			Error:   jsonErr.Error(),
		}
	}
	if resp.ID == "" {
		return UploadResult{
			Path:    relativePath,
			Success: false,
			Error:   "upload succeeded but no file ID returned",
		}
	}

	return UploadResult{
		Path:    relativePath,
		FileID:  resp.ID,
		Size:    resp.Size,
		Success: true,
	}
}

// buildMultipartBody constructs a multipart/form-data body for file upload.
func buildMultipartBody(filename string, content []byte) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// File part.
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write(content); err != nil {
		return nil, "", err
	}

	// Purpose part.
	if err := w.WriteField("purpose", "user_data"); err != nil {
		return nil, "", err
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), w.FormDataContentType(), nil
}

// UploadFiles uploads multiple files concurrently with a concurrency limit.
func (c *FilesAPIConfig) UploadFiles(files []struct{ Path, RelativePath string }, concurrency int) []UploadResult {
	if len(files) == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}

	results := make([]UploadResult, len(files))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, f := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, file struct{ Path, RelativePath string }) {
			defer wg.Done()
			results[idx] = c.UploadFile(file.Path, file.RelativePath)
			<-sem
		}(i, f)
	}
	wg.Wait()
	return results
}

// ListFilesCreatedAfter lists files created after the given ISO 8601 timestamp.
func (c *FilesAPIConfig) ListFilesCreatedAfter(afterCreatedAt string) ([]FileMetadata, error) {
	baseURL := c.baseURL()
	headers := c.defaultHeaders()

	var allFiles []FileMetadata
	var afterID string

	for {
		params := make(map[string]string)
		params["after_created_at"] = afterCreatedAt
		if afterID != "" {
			params["after_id"] = afterID
		}

		page, err := retryWithBackoff("list files", func(attempt int) retryResult {
			req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/files", nil)
			if err != nil {
				return retryResult{Error: err.Error()}
			}
			req.Header = headers

			q := req.URL.Query()
			for k, v := range params {
				q.Set(k, v)
			}
			req.URL.RawQuery = q.Encode()

			resp, err := c.httpClient().Do(req)
			if err != nil {
				return retryResult{Error: err.Error()}
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return retryResult{Error: err.Error()}
				}
				return retryResult{Done: true, Value: data}
			case http.StatusUnauthorized:
				return retryResult{Done: false, Error: "authentication failed"}
			case http.StatusForbidden:
				return retryResult{Done: false, Error: "access denied"}
			default:
				return retryResult{Error: fmt.Sprintf("status %d", resp.StatusCode)}
			}
		})
		if err != nil {
			return nil, err
		}

		var payload struct {
			Data     []struct {
				ID        string `json:"id"`
				Filename  string `json:"filename"`
				SizeBytes int64  `json:"size_bytes"`
			} `json:"data"`
			HasMore bool `json:"has_more"`
		}
		if err := json.Unmarshal(page, &payload); err != nil {
			return nil, fmt.Errorf("parse list response: %w", err)
		}

		for _, f := range payload.Data {
			allFiles = append(allFiles, FileMetadata{
				Filename: f.Filename,
				FileID:   f.ID,
				Size:     f.SizeBytes,
			})
		}

		if !payload.HasMore || len(payload.Data) == 0 {
			break
		}
		afterID = payload.Data[len(payload.Data)-1].ID
	}

	return allFiles, nil
}

// ParseFileSpecs parses file attachment specs from CLI arguments.
// Format: <file_id>:<relative_path>
func ParseFileSpecs(specs []string) []File {
	var files []File
	for _, spec := range specs {
		for _, s := range strings.Fields(spec) {
			colonIdx := strings.Index(s, ":")
			if colonIdx == -1 {
				continue
			}
			fileID := s[:colonIdx]
			relativePath := s[colonIdx+1:]
			if fileID == "" || relativePath == "" {
				continue
			}
			files = append(files, File{FileID: fileID, RelativePath: relativePath})
		}
	}
	return files
}
