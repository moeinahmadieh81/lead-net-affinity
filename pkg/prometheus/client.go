package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		log.Printf("[lead-net][prom] invalid Prometheus URL %q: %v", rawURL, err)
		return nil, err
	}
	log.Printf("[lead-net][prom] creating Prometheus client for baseURL=%s", u.String())
	return &Client{
		baseURL:    u,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *Client) Query(ctx context.Context, q string) (queryResult, error) {
	start := time.Now()

	u := *c.baseURL
	u.Path = "/api/v1/query"
	qs := u.Query()
	qs.Set("query", q)
	u.RawQuery = qs.Encode()

	log.Printf("[lead-net][prom] executing query %q against %s", q, u.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		log.Printf("[lead-net][prom] NewRequest failed for query %q: %v", q, err)
		return queryResult{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[lead-net][prom] HTTP request failed for query %q: %v", q, err)
		return queryResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[lead-net][prom] non-OK status for query %q: %s", q, resp.Status)
		return queryResult{}, fmt.Errorf("prometheus status: %s", resp.Status)
	}

	var r queryResult
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		log.Printf("[lead-net][prom] failed to decode response for query %q: %v", q, err)
		return queryResult{}, err
	}
	if r.Status != "success" {
		log.Printf("[lead-net][prom] query %q failed: status=%s", q, r.Status)
		return queryResult{}, fmt.Errorf("prometheus query failed: %s", r.Status)
	}

	log.Printf("[lead-net][prom] query %q succeeded in %s, resultType=%s, series=%d",
		q, time.Since(start).Round(time.Millisecond), r.Data.ResultType, len(r.Data.Result))

	return r, nil
}
