package telegram

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-telegram/bot/models"
)

// GetLargestPhoto returns the largest photo from an array of PhotoSize
func GetLargestPhoto(photos []models.PhotoSize) *models.PhotoSize {
	if len(photos) == 0 {
		return nil
	}

	largest := &photos[0]
	maxSize := photos[0].Width * photos[0].Height

	for i := 1; i < len(photos); i++ {
		size := photos[i].Width * photos[i].Height
		if size > maxSize {
			maxSize = size
			largest = &photos[i]
		}
	}

	return largest
}

// FileResponse represents the response from Telegram getFile API
type FileResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		FileID       string `json:"file_id"`
		FileUniqueID string `json:"file_unique_id"`
		FileSize     int    `json:"file_size"`
		FilePath     string `json:"file_path"`
	} `json:"result"`
}


// mediaClient is the shared HTTP client for media downloads
var mediaClient *http.Client = &http.Client{}

// SetMediaClient sets the HTTP client used for media downloads
func SetMediaClient(client *http.Client) {
	if client != nil {
		mediaClient = client
	}
}
// DownloadPhoto downloads a photo from Telegram servers using the Bot API
func DownloadPhoto(ctx context.Context, botToken, fileID string) ([]byte, error) {
	// Step 1: Get file path using getFile API
	getFileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, fileID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getFileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create getFile request: %w", err)
	}

	// Use shared media client
	resp, err := mediaClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getFile request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getFile failed with status %d: %s", resp.StatusCode, string(body))
	}

	var fileResp FileResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return nil, fmt.Errorf("decode getFile response: %w", err)
	}

	if !fileResp.OK {
		return nil, fmt.Errorf("getFile returned ok=false")
	}

	// Step 2: Download the actual file
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResp.Result.FilePath)

	fileReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create file download request: %w", err)
	}

	fileDownloadResp, err := mediaClient.Do(fileReq)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer fileDownloadResp.Body.Close()

	if fileDownloadResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("file download failed with status %d", fileDownloadResp.StatusCode)
	}

	// Read file data
	fileData, err := io.ReadAll(fileDownloadResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read file data: %w", err)
	}

	return fileData, nil
}

// EncodeBase64 encodes binary data to base64 string
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
