package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"wwlocal-wework/config"
)

type WeWorkService struct {
	baseURL  string
	cfg      *config.WeWorkConfig
	client   *http.Client
	token    string
	tokenExp time.Time
	tokenMu  sync.Mutex
}

func NewWeWorkService(cfg *config.WeWorkConfig) *WeWorkService {
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost: 20,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
	}

	return &WeWorkService{
		baseURL: cfg.BaseURL,
		cfg:     cfg,
		client: &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		},
	}
}

func (s *WeWorkService) GetToken() (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if s.token != "" && time.Now().Before(s.tokenExp) {
		return s.token, nil
	}

	path := fmt.Sprintf("/cgi-bin/gettoken?corpid=%s&corpsecret=%s", s.cfg.CorpID, s.cfg.Secret)
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
		log.Printf("get token error: errcode=%d, errmsg=%s", result.ErrCode, result.ErrMsg)
		return "", fmt.Errorf("get token error: %s (errcode: %d)", result.ErrMsg, result.ErrCode)
	}

	log.Printf("token refreshed successfully, expires_in=%d", result.ExpiresIn)
	s.token = result.AccessToken
	s.tokenExp = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)

	return s.token, nil
}

func (s *WeWorkService) GetLogList(featureID int, startTime, endTime int64, startIndex, limit int) ([]LogItem, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, err
	}

	path := "/cgi-bin/corp/get_log_list"
	reqBody := map[string]interface{}{
		"feature_id":  featureID,
		"start_time":  startTime,
		"end_time":    endTime,
		"start_index": startIndex,
		"limit":       limit,
	}

	resp, err := s.doRequest("POST", path, reqBody, token)
	if err != nil {
		return nil, fmt.Errorf("get log list failed: %w", err)
	}

	var result struct {
		ErrCode    int       `json:"errcode"`
		ErrMsg     string    `json:"errmsg"`
		FeatureID  int       `json:"feature_id"`
		LogList    []LogItem `json:"log_list"`
		StartIndex int       `json:"start_index"`
		Limit      int       `json:"limit"`
		EndTime    int64     `json:"end_time"`
		HasMore    bool      `json:"has_more"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse log list response failed: %w", err)
	}

	if result.ErrCode != 0 {
		log.Printf("get log list error: feature=%d, errcode=%d, errmsg=%s", featureID, result.ErrCode, result.ErrMsg)
		return nil, fmt.Errorf("get log list error: %s (errcode: %d)", result.ErrMsg, result.ErrCode)
	}

	return result.LogList, nil
}

type LogItem struct {
	FeatureID int    `json:"feature_id"`
	LogTime   int64  `json:"log_time"`
	IDC       string `json:"idc"`
	EncKey    string `json:"enc_key"`
	EncData   string `json:"enc_data"`
}

func (s *WeWorkService) doRequest(method, path string, body interface{}, token ...string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<(attempt-1)) * time.Second) // 1s, 2s
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
		if len(token) > 0 {
			q := req.URL.Query()
			q.Set("access_token", token[0])
			req.URL.RawQuery = q.Encode()
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request failed: %w", err)
			log.Printf("request attempt %d failed: %v", attempt+1, err)
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
			log.Printf("request attempt %d got HTTP %d", attempt+1, resp.StatusCode)
			continue
		}

		return data, nil
	}
	return nil, lastErr
}
