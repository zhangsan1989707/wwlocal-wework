package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
)

type AsyncExportService struct {
	baseURL       string
	corpid        string
	contactSecret string
	client        *http.Client
	token         string
	tokenExp      time.Time
	tokenMu       sync.Mutex
}

func NewAsyncExportService(cfg *config.WeWorkConfig) *AsyncExportService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify},
	}
	return &AsyncExportService{
		baseURL:       cfg.BaseURL,
		corpid:        cfg.CorpID,
		contactSecret: cfg.ContactSecret,
		client: &http.Client{
			Transport: tr,
			Timeout:   60 * time.Second,
		},
	}
}

func (s *AsyncExportService) GetToken() (string, error) {
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

type ExportJobStatus struct {
	ErrCode int         `json:"errcode"`
	ErrMsg  string      `json:"errmsg"`
	JobID   string      `json:"jobid"`
	Status  int         `json:"status"`
	Type    string      `json:"type,omitempty"`
	Detail  interface{} `json:"detail,omitempty"`
}

func (s *AsyncExportService) StartExport(departmentID int, fetchChild int) (string, error) {
	token, err := s.GetToken()
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("/cgi-bin/async/list?access_token=%s&department_id=%d&fetch_child=%d",
		token, departmentID, fetchChild)

	resp, err := s.doRequest("GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("start export failed: %w", err)
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		JobID   string `json:"jobid"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parse export response failed: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("start export error: %s", result.ErrMsg)
	}

	log.Printf("AsyncExport: started job %s for department %d", result.JobID, departmentID)
	return result.JobID, nil
}

func (s *AsyncExportService) GetJobStatus(jobID string) (*ExportJobStatus, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/cgi-bin/async/getresult?access_token=%s&jobid=%s",
		token, url.QueryEscape(jobID))

	resp, err := s.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get job status failed: %w", err)
	}

	var result ExportJobStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse job status response failed: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("get job status error: %s", result.ErrMsg)
	}

	return &result, nil
}

type ExportedUser struct {
	UserID         string   `json:"userid"`
	Name           string   `json:"name"`
	Department     []int    `json:"department"`
	Order          []any    `json:"order,omitempty"`
	Position       string   `json:"position"`
	Positions      []string `json:"positions"`
	Mobile         string   `json:"mobile"`
	Gender         string   `json:"gender"`
	Email          string   `json:"email"`
	IsLeaderInDept []int    `json:"is_leader_in_dept"`
	Avatar         string   `json:"avatar"`
	Telephone      string   `json:"telephone"`
	EnglishName    string   `json:"english_name"`
	Status         int      `json:"status"`
	Enable         int      `json:"enable"`
}

type ExportResult struct {
	ErrCode   int            `json:"errcode"`
	ErrMsg    string         `json:"errmsg"`
	Status    int            `json:"status"`
	Type      string         `json:"type"`
	Users     []ExportedUser `json:"users,omitempty"`
	FileItems []FileItem     `json:"file_items,omitempty"`
}

type FileItem struct {
	FileName string `json:"file_name"`
	FileID   string `json:"file_id"`
}

func (s *AsyncExportService) GetExportResult(jobID string) (*ExportResult, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/cgi-bin/async/getresult?access_token=%s&jobid=%s",
		token, url.QueryEscape(jobID))

	resp, err := s.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get export result failed: %w", err)
	}

	var result ExportResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse export result failed: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("get export result error: %s", result.ErrMsg)
	}

	return &result, nil
}

func (s *AsyncExportService) PollExportResult(jobID string, timeout time.Duration, pollInterval time.Duration) (*ExportResult, error) {
	deadline := time.Now().Add(timeout)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			result, err := s.GetExportResult(jobID)
			if err != nil {
				return nil, err
			}

			switch result.Status {
			case 0:
				log.Printf("AsyncExport: job %s status=0 (queued)", jobID)
			case 1:
				log.Printf("AsyncExport: job %s status=1 (processing)", jobID)
			case 2:
				log.Printf("AsyncExport: job %s status=2 (completed), got %d users, %d files",
					jobID, len(result.Users), len(result.FileItems))
				return result, nil
			case 3:
				log.Printf("AsyncExport: job %s status=3 (failed)", jobID)
				return nil, fmt.Errorf("export job failed: %v", result.ErrMsg)
			}

			if time.Now().After(deadline) {
				return nil, fmt.Errorf("export timeout after %v", timeout)
			}
		}
	}
}

func (s *AsyncExportService) DownloadExportFile(fileID string) ([]byte, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/cgi-bin/media/get_async_export_file?access_token=%s&file_id=%s",
		token, url.QueryEscape(fileID))

	resp, err := s.doRequestRaw("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("download file failed: %w", err)
	}

	return resp, nil
}

func (s *AsyncExportService) doRequest(method, path string, body interface{}) ([]byte, error) {
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

func (s *AsyncExportService) doRequestRaw(method, path string, body interface{}) ([]byte, error) {
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

func ExportedUserToContact(u ExportedUser) model.Contact {
	deptBytes, _ := json.Marshal(u.Department)
	posBytes, _ := json.Marshal(u.Positions)
	gender, _ := strconv.Atoi(u.Gender)

	rawJSON, _ := json.Marshal(u)

	return model.Contact{
		UserID:     u.UserID,
		Name:       u.Name,
		Mobile:     u.Mobile,
		Gender:     gender,
		Email:      u.Email,
		Position:   u.Position,
		Department: string(deptBytes),
		Positions:  string(posBytes),
		Avatar:     u.Avatar,
		Status:     u.Status,
		RawJSON:    string(rawJSON),
		SyncedAt:   time.Now(),
	}
}
