package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

type queryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func NewClient(rawURL string) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:    u,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *Client) Query(ctx context.Context, q string) (queryResult, error) {
	u := *c.baseURL
	u.Path = "/api/v1/query"
	qs := u.Query()
	qs.Set("query", q)
	u.RawQuery = qs.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return queryResult{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return queryResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return queryResult{}, fmt.Errorf("prometheus status: %s", resp.Status)
	}

	var r queryResult
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return queryResult{}, err
	}
	if r.Status != "success" {
		return queryResult{}, fmt.Errorf("prometheus query failed: %s", r.Status)
	}
	return r, nil
}
