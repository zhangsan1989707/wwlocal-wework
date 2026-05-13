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

type ContactService struct {
	baseURL       string
	corpid        string
	contactSecret string
	client        *http.Client
	token         string
	tokenExp      time.Time
	tokenMu       sync.Mutex
}

func NewContactService(cfg *config.WeWorkConfig) *ContactService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &ContactService{
		baseURL:       cfg.BaseURL,
		corpid:        cfg.CorpID,
		contactSecret: cfg.ContactSecret,
		client: &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		},
	}
}

func (s *ContactService) GetToken() (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if s.token != "" && time.Now().Before(s.tokenExp) {
		return s.token, nil
	}

	path := fmt.Sprintf("/cgi-bin/gettoken?corpid=%s&corpsecret=%s", s.corpid, s.contactSecret)
	resp, err := s.doRequest("GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("get contact token failed: %w", err)
	}

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parse contact token response failed: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("get contact token error: %s", result.ErrMsg)
	}

	s.token = result.AccessToken
	s.tokenExp = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return s.token, nil
}

func (s *ContactService) GetDepartments() ([]model.DepartmentItem, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, err
	}

	resp, err := s.doRequest("GET", "/cgi-bin/department/list", nil, token)
	if err != nil {
		return nil, fmt.Errorf("get departments failed: %w", err)
	}

	var result struct {
		ErrCode    int                   `json:"errcode"`
		ErrMsg     string                `json:"errmsg"`
		Department []model.DepartmentItem `json:"department"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse departments response failed: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("get departments error: %s", result.ErrMsg)
	}
	return result.Department, nil
}

func (s *ContactService) GetSimpleUserList(departmentID int, fetchChild int) ([]model.SimpleUser, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, err
	}

	var allUsers []model.SimpleUser
	cursor := ""

	for {
		path := fmt.Sprintf("/cgi-bin/user/simplelist?department_id=%d&fetch_child=%d", departmentID, fetchChild)
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		resp, err := s.doRequest("GET", path, nil, token)
		if err != nil {
			return nil, fmt.Errorf("get simple user list failed: %w", err)
		}

		var result struct {
			ErrCode    int               `json:"errcode"`
			ErrMsg     string            `json:"errmsg"`
			UserList   []model.SimpleUser `json:"userlist"`
			NextCursor string            `json:"next_cursor"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("parse simple user list response failed: %w", err)
		}
		if result.ErrCode != 0 {
			return nil, fmt.Errorf("get simple user list error: %s", result.ErrMsg)
		}

		allUsers = append(allUsers, result.UserList...)
		log.Printf("GetSimpleUserList: dept=%d, fetched=%d, total=%d", departmentID, len(result.UserList), len(allUsers))

		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
		time.Sleep(100 * time.Millisecond)
	}

	return allUsers, nil
}

func (s *ContactService) GetUserDetail(userID string) (*model.ContactDetail, string, error) {
	token, err := s.GetToken()
	if err != nil {
		return nil, "", err
	}

	path := fmt.Sprintf("/cgi-bin/user/get?userid=%s", url.QueryEscape(userID))
	resp, err := s.doRequest("GET", path, nil, token)
	if err != nil {
		return nil, "", fmt.Errorf("get user detail failed: %w", err)
	}

	var result struct {
		ErrCode int `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		model.ContactDetail
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, "", fmt.Errorf("parse user detail response failed: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, "", fmt.Errorf("get user detail error for %s: %s", userID, result.ErrMsg)
	}

	return &result.ContactDetail, string(resp), nil
}

// FetchAllDetails 并发拉取用户详情，返回成功的详情列表和失败的 userID 列表
func (s *ContactService) FetchAllDetails(userIDs []string, concurrency int, cancelCh <-chan struct{}) ([]model.Contact, []string) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	if concurrency <= 0 {
		concurrency = 5
	}

	type result struct {
		contact model.Contact
		err     string
	}

	jobsCh := make(chan string, len(userIDs))
	resultsCh := make(chan result, len(userIDs))

	for _, uid := range userIDs {
		jobsCh <- uid
	}
	close(jobsCh)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for uid := range jobsCh {
				select {
				case <-cancelCh:
					return
				default:
				}

				detail, rawJSON, err := s.GetUserDetail(uid)
				if err != nil {
					log.Printf("fetch detail failed for %s: %v", uid, err)
					resultsCh <- result{err: uid}
					continue
				}
				deptBytes, _ := json.Marshal(detail.Department)
				posBytes, _ := json.Marshal(detail.Positions)
				gender, _ := strconv.Atoi(detail.Gender)
				resultsCh <- result{contact: model.Contact{
					UserID:     detail.UserID,
					Name:       detail.Name,
					Mobile:     detail.Mobile,
					Gender:     gender,
					Email:      detail.Email,
					Position:   detail.Position,
					Department: string(deptBytes),
					Positions:  string(posBytes),
					Avatar:     detail.Avatar,
					Status:     detail.Status,
					RawJSON:    rawJSON,
					SyncedAt:   time.Now(),
				}}
				time.Sleep(200 * time.Millisecond)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var contacts []model.Contact
	var failed []string
	done := 0
	total := len(userIDs)
	for r := range resultsCh {
		if r.err != "" {
			failed = append(failed, r.err)
		} else {
			contacts = append(contacts, r.contact)
		}
		done++
		if done%100 == 0 || done == total {
			log.Printf("FetchAllDetails progress: %d/%d (ok=%d, fail=%d)", done, total, len(contacts), len(failed))
		}
	}
	return contacts, failed
}

func (s *ContactService) doRequest(method, path string, body interface{}, token ...string) ([]byte, error) {
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
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
