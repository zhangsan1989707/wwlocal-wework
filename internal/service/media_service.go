package service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"wwlocal-wework/config"
)

type MediaService struct {
	baseURL       string
	corpid        string
	contactSecret string
	client        *http.Client
	token         string
	tokenExp      time.Time
	tokenMu       sync.Mutex
}

func NewMediaService(cfg *config.WeWorkConfig) *MediaService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify},
	}
	return &MediaService{
		baseURL:       cfg.BaseURL,
		corpid:        cfg.CorpID,
		contactSecret: cfg.ContactSecret,
		client: &http.Client{
			Transport: tr,
			Timeout:   120 * time.Second,
		},
	}
}

func (s *MediaService) GetToken() (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if s.token != "" && time.Now().Before(s.tokenExp) {
		return s.token, nil
	}

	path := fmt.Sprintf("/cgi-bin/gettoken?corpid=%s&corpsecret=%s", s.corpid, s.contactSecret)
	resp, err := s.doRequest("GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("get token failed: %w", err)
	}

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parse token response failed: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("get token error: %s", result.ErrMsg)
	}

	s.token = result.AccessToken
	s.tokenExp = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return s.token, nil
}

type UploadResponse struct {
	ErrCode   int    `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
	MediaID   string `json:"media_id"`
	CreatedAt string `json:"created_at"`
}

func (s *MediaService) UploadFile(filePath string, fileType string) (*UploadResponse, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("media", filePath)
	if err != nil {
		return nil, fmt.Errorf("create form file failed: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copy file content failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer failed: %w", err)
	}

	path := fmt.Sprintf("/cgi-bin/media/upload?access_token=%s&type=%s", token, fileType)
	req, err := http.NewRequest("POST", s.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var result UploadResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse upload response failed: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("upload error: %s", result.ErrMsg)
	}

	log.Printf("MediaService: uploaded file %s, got media_id=%s", filePath, result.MediaID)
	return &result, nil
}

func (s *MediaService) UploadBuffer(data []byte, fileName string, fileType string) (*UploadResponse, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("media", fileName)
	if err != nil {
		return nil, fmt.Errorf("create form file failed: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write data failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer failed: %w", err)
	}

	path := fmt.Sprintf("/cgi-bin/media/upload?access_token=%s&type=%s", token, fileType)
	req, err := http.NewRequest("POST", s.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var result UploadResponse
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("parse upload response failed: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("upload error: %s", result.ErrMsg)
	}

	log.Printf("MediaService: uploaded buffer %s, got media_id=%s", fileName, result.MediaID)
	return &result, nil
}

type DownloadResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (s *MediaService) DownloadFile(mediaID string, savePath string) error {
	token, err := s.GetToken()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/cgi-bin/media/get?access_token=%s&media_id=%s", token, mediaID)

	req, err := http.NewRequest("GET", s.baseURL+path, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response failed: %w", err)
		}
		var result DownloadResponse
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("parse download response failed: %w", err)
		}
		return fmt.Errorf("download error: %s", result.ErrMsg)
	}

	outFile, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	log.Printf("MediaService: downloaded media_id=%s to %s", mediaID, savePath)
	return nil
}

func (s *MediaService) DownloadBuffer(mediaID string) ([]byte, string, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, "", err
	}

	path := fmt.Sprintf("/cgi-bin/media/get?access_token=%s&media_id=%s", token, mediaID)

	req, err := http.NewRequest("GET", s.baseURL+path, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("read response failed: %w", err)
		}
		var result DownloadResponse
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, "", fmt.Errorf("parse download response failed: %w", err)
		}
		return nil, "", fmt.Errorf("download error: %s", result.ErrMsg)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response failed: %w", err)
	}

	disposition := resp.Header.Get("Content-Disposition")
	fileName := "unknown"
	if strings.Contains(disposition, "filename=") {
		parts := strings.Split(disposition, "filename=")
		if len(parts) > 1 {
			fileName = strings.Trim(parts[1], "\" ")
		}
	}

	log.Printf("MediaService: downloaded media_id=%s, size=%d", mediaID, len(data))
	return data, fileName, nil
}

func (s *MediaService) doRequest(method, path string, body interface{}) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<(attempt-1)) * time.Second)
		}

		var reqBody io.Reader
		if body != nil {
			bodyBytes, _ := json.Marshal(body)
			reqBody = strings.NewReader(string(bodyBytes))
		}

		req, err := http.NewRequest(method, s.baseURL+path, reqBody)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request failed: %w", err)
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response failed: %w", err)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: HTTP %d", resp.StatusCode)
			continue
		}

		return data, nil
	}
	return nil, lastErr
}
