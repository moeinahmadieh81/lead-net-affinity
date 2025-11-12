package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	promc "lead-net-affinity/pkg/prometheus"
)

func TestPrometheus_Query_And_FetchMatrix(t *testing.T) {
	// fake /api/v1/query responder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		// Check query param exists
		if _, ok := r.URL.Query()["query"]; !ok {
			http.Error(w, "missing query", 400)
			return
		}
		resp := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []map[string]interface{}{
					{
						"metric": map[string]string{
							"src_node": "nodeA",
							"dst_node": "nodeB",
						},
						"value": []interface{}{float64(0), "0.001"}, // pretend seconds; will be ms in code
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)

	c, err := promc.NewClient(u.String())
	if err != nil {
		t.Fatalf("NewClient err: %v", err)
	}

	// Low-level Query
	if _, err := c.Query(context.Background(), "test_query"); err != nil {
		t.Fatalf("Query err: %v", err)
	}

	// High-level FetchNetworkMatrix (will call Query 1-3 times)
	m, err := c.FetchNetworkMatrix(context.Background(), "lat_q", "drop_q", "bw_q")
	if err != nil {
		t.Fatalf("FetchNetworkMatrix err: %v", err)
	}
	if len(m.Links) == 0 {
		t.Fatalf("expected at least one link")
	}
	if m.Get("nodeA", "nodeB") == nil {
		t.Fatalf("expected nodeA->nodeB link")
	}
}
