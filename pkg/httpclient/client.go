package httpclient

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"wwlocal-wework/config"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(cfg *config.WeWorkConfig) *Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return &Client{
		baseURL: cfg.BaseURL,
		client: &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		},
	}
}

func (c *Client) Get(path string) ([]byte, error) {
	url := c.baseURL + path
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) Post(path string, body []byte) ([]byte, error) {
	url := c.baseURL + path
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("http post failed: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}