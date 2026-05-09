package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wwlocal-wework/config"
)

type WeWorkService struct {
	baseURL string
	cfg     *config.WeWorkConfig
	client  *http.Client
	token   string
	tokenExp time.Time
}

func NewWeWorkService(cfg *config.WeWorkConfig) *WeWorkService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
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
		return "", fmt.Errorf("get token error: %s", result.ErrMsg)
	}

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
		return nil, fmt.Errorf("get log list error: %s", result.ErrMsg)
	}

	return result.LogList, nil
}

type LogItem struct {
	FeatureID int    `json:"feature_id"`
	LogTime   int64  `json:"log_time"`
	IDC       string `json:"idc"`
	EncKey    string `json:"encrypt_key"`
	EncData   string `json:"encrypt_data"`
}

func (s *WeWorkService) doRequest(method, path string, body interface{}, token ...string) ([]byte, error) {
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
		req.URL.RawQuery = "access_token=" + url.QueryEscape(token[0])
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}