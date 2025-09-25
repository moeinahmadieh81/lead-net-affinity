package monitoring

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MetricResult represents a single metric result from Prometheus
type MetricResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// PrometheusClient interface defines the contract for Prometheus clients
type PrometheusClient interface {
	Query(query string) ([]MetricResult, error)
	QueryRange(query string, start, end time.Time, step time.Duration) ([]MetricResult, error)
}

// RealPrometheusClient implements actual Prometheus API client
type RealPrometheusClient struct {
	baseURL    string
	httpClient *http.Client
}

// PrometheusResponse represents the Prometheus API response
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string         `json:"resultType"`
		Result     []MetricResult `json:"result"`
	} `json:"data"`
}

// NewRealPrometheusClient creates a new Prometheus client
func NewRealPrometheusClient(prometheusURL string) *RealPrometheusClient {
	return &RealPrometheusClient{
		baseURL: prometheusURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Query executes a Prometheus query
func (c *RealPrometheusClient) Query(query string) ([]MetricResult, error) {
	return c.queryPrometheus(query, time.Time{}, time.Time{}, 0)
}

// QueryRange executes a Prometheus range query
func (c *RealPrometheusClient) QueryRange(query string, start, end time.Time, step time.Duration) ([]MetricResult, error) {
	return c.queryPrometheus(query, start, end, step)
}

// queryPrometheus executes the actual Prometheus query
func (c *RealPrometheusClient) queryPrometheus(query string, start, end time.Time, step time.Duration) ([]MetricResult, error) {
	// Build query URL
	queryURL := fmt.Sprintf("%s/api/v1/query", c.baseURL)
	if !start.IsZero() && !end.IsZero() && step > 0 {
		queryURL = fmt.Sprintf("%s/api/v1/query_range", c.baseURL)
	}

	// Build query parameters
	params := url.Values{}
	params.Set("query", query)

	if !start.IsZero() {
		params.Set("start", strconv.FormatInt(start.Unix(), 10))
	}
	if !end.IsZero() {
		params.Set("end", strconv.FormatInt(end.Unix(), 10))
	}
	if step > 0 {
		params.Set("step", strconv.FormatInt(int64(step.Seconds()), 10))
	}

	// Make HTTP request
	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())
	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus query failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var promResp PrometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, fmt.Errorf("failed to decode prometheus response: %v", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", promResp.Status)
	}

	return promResp.Data.Result, nil
}

// MockPrometheusClient implements a mock Prometheus client for testing
type MockPrometheusClient struct {
	responses map[string][]MetricResult
}

// NewMockPrometheusClient creates a mock Prometheus client
func NewMockPrometheusClient() *MockPrometheusClient {
	return &MockPrometheusClient{
		responses: make(map[string][]MetricResult),
	}
}

// SetMockResponse sets a mock response for a query
func (c *MockPrometheusClient) SetMockResponse(query string, results []MetricResult) {
	c.responses[query] = results
}

// Query returns mock data for testing
func (c *MockPrometheusClient) Query(query string) ([]MetricResult, error) {
	if results, exists := c.responses[query]; exists {
		return results, nil
	}

	// Return default mock data based on query type
	return c.generateMockResponse(query), nil
}

// QueryRange returns mock data for testing
func (c *MockPrometheusClient) QueryRange(query string, start, end time.Time, step time.Duration) ([]MetricResult, error) {
	return c.Query(query)
}

// generateMockResponse generates realistic mock data based on query type
func (c *MockPrometheusClient) generateMockResponse(query string) []MetricResult {
	// Generate different mock data based on query content
	if strings.Contains(query, "bandwidth") || strings.Contains(query, "network_receive_bytes") {
		// Mock bandwidth data
		return []MetricResult{
			{
				Metric: map[string]string{"instance": "node-1"},
				Value:  []interface{}{time.Now().Unix(), "800.5"},
			},
			{
				Metric: map[string]string{"instance": "node-2"},
				Value:  []interface{}{time.Now().Unix(), "750.2"},
			},
			{
				Metric: map[string]string{"instance": "node-3"},
				Value:  []interface{}{time.Now().Unix(), "900.1"},
			},
		}
	} else if strings.Contains(query, "latency") || strings.Contains(query, "duration") {
		// Mock latency data
		return []MetricResult{
			{
				Metric: map[string]string{"service": "frontend"},
				Value:  []interface{}{time.Now().Unix(), "45.2"},
			},
			{
				Metric: map[string]string{"service": "search"},
				Value:  []interface{}{time.Now().Unix(), "38.7"},
			},
			{
				Metric: map[string]string{"service": "profile"},
				Value:  []interface{}{time.Now().Unix(), "52.1"},
			},
		}
	} else if strings.Contains(query, "throughput") || strings.Contains(query, "transmit_bytes") {
		// Mock throughput data
		return []MetricResult{
			{
				Metric: map[string]string{"instance": "node-1"},
				Value:  []interface{}{time.Now().Unix(), "650.3"},
			},
			{
				Metric: map[string]string{"instance": "node-2"},
				Value:  []interface{}{time.Now().Unix(), "720.8"},
			},
		}
	} else if strings.Contains(query, "packet_loss") || strings.Contains(query, "drop") {
		// Mock packet loss data
		return []MetricResult{
			{
				Metric: map[string]string{"instance": "node-1"},
				Value:  []interface{}{time.Now().Unix(), "0.05"},
			},
			{
				Metric: map[string]string{"instance": "node-2"},
				Value:  []interface{}{time.Now().Unix(), "0.08"},
			},
		}
	} else if strings.Contains(query, "requests_total") {
		// Mock request rate data
		return []MetricResult{
			{
				Metric: map[string]string{"service": "frontend"},
				Value:  []interface{}{time.Now().Unix(), "1500.5"},
			},
			{
				Metric: map[string]string{"service": "search"},
				Value:  []interface{}{time.Now().Unix(), "1200.3"},
			},
			{
				Metric: map[string]string{"service": "profile"},
				Value:  []interface{}{time.Now().Unix(), "800.7"},
			},
		}
	} else {
		// Default mock data
		return []MetricResult{
			{
				Metric: map[string]string{"instance": "default"},
				Value:  []interface{}{time.Now().Unix(), "100.0"},
			},
		}
	}
}
