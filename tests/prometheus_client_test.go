package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	promc "lead-net-affinity/pkg/prometheus"
)

// This test is intentionally "struct-shape-agnostic":
// It doesn't assume whether NetworkMatrix has Links, Nodes, or any other field.
// It only checks that FetchNetworkMatrix:
//  1. successfully calls Prometheus,
//  2. parses the response without error,
//  3. produces a matrix with at least one non-empty map field.
//
// That way it stays valid even if we change NetworkMatrix layout
// (per-node vs per-link, etc.).
func TestPrometheus_Query_And_FetchMatrix(t *testing.T) {
	t.Helper()

	// Fake Prometheus HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")

		// We support three queries that the code under test may send:
		// latencyQuery, dropQuery, bwQuery.
		//
		// To be robust against both "per-node" and "per-link" code,
		// we include *both* `instance` and `src_node`/`dst_node` labels.
		switch q {
		case "latency_query":
			fmt.Fprint(w, `{
			  "status": "success",
			  "data": {
			    "resultType": "vector",
			    "result": [
			      {
			        "metric": {
			          "instance": "nodeA",
			          "src_node": "nodeA",
			          "dst_node": "nodeB"
			        },
			        "value": [ 1731700000.0, "0.005" ]
			      }
			    ]
			  }
			}`)
		case "drop_query":
			fmt.Fprint(w, `{
			  "status": "success",
			  "data": {
			    "resultType": "vector",
			    "result": [
			      {
			        "metric": {
			          "instance": "nodeA",
			          "src_node": "nodeA",
			          "dst_node": "nodeB"
			        },
			        "value": [ 1731700001.0, "10" ]
			      }
			    ]
			  }
			}`)
		case "bw_query":
			fmt.Fprint(w, `{
			  "status": "success",
			  "data": {
			    "resultType": "vector",
			    "result": [
			      {
			        "metric": {
			          "instance": "nodeA",
			          "src_node": "nodeA",
			          "dst_node": "nodeB"
			        },
			        "value": [ 1731700002.0, "100" ]
			      }
			    ]
			  }
			}`)
		default:
			t.Fatalf("unexpected Prometheus query: %q", q)
		}
	}))
	defer ts.Close()

	// Real client pointing to fake server
	client, err := promc.NewClient(ts.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()

	matrix, err := client.FetchNetworkMatrix(
		ctx,
		"latency_query",
		"drop_query",
		"bw_query",
	)
	if err != nil {
		t.Fatalf("FetchNetworkMatrix() error = %v", err)
	}
	if matrix == nil {
		t.Fatalf("FetchNetworkMatrix() returned nil matrix")
	}

	// Use reflection so we *don't* need to know whether the struct has
	// Nodes, Links, or something else right now.
	v := reflect.ValueOf(matrix)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		t.Fatalf("NetworkMatrix is not a struct, got kind %v", v.Kind())
	}

	// Find at least one exported map field that is non-empty.
	foundNonEmpty := false
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Map && f.Len() > 0 {
			foundNonEmpty = true
			break
		}
	}

	if !foundNonEmpty {
		t.Fatalf("expected at least one non-empty map field in NetworkMatrix, got none")
	}
}
