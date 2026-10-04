package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestAudit(t *testing.T) {
	input := ReviewInput{
		DigitalAssetDelivery: Workflow{"asset delivery", []string{"asset:read", "asset:send"}},
		SubscriberUpdates:    Workflow{"subscriber updates", []string{"subscriber:write"}},
		ContentProcessing:    Workflow{"content processing", []string{"content:run"}},
	}
	tests := []struct {
		name string
		key  Key
		want []Reach
	}{
		{"partial", Key{ID: "k1", Scopes: []string{"asset:read", "asset:send", "content:run"}}, []Reach{{"asset delivery", true, []string{}}, {"subscriber updates", false, []string{"subscriber:write"}}, {"content processing", true, []string{}}}},
		{"none", Key{ID: "k2"}, []Reach{{"asset delivery", false, []string{"asset:read", "asset:send"}}, {"subscriber updates", false, []string{"subscriber:write"}}, {"content processing", false, []string{"content:run"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := audit([]Key{tt.key}, input)[0].Reach
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("reach = %#v; want %#v", got, tt.want)
			}
		})
	}
}

func TestSameCredentialAndRetry(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-from-env" {
			t.Error("credential differs")
		}
		switch r.URL.Path {
		case "/v1/account/keys/list":
			if r.Method != http.MethodGet {
				t.Error("keys method")
			}
			w.Write([]byte(`{"ok":true,"data":{"count":1,"items":[{"id":"k1","name":"editor","scopes":["asset:read"]}],"next_cursor":null}}`))
		case "/v1/pdf/generate":
			requests++
			if r.Method != http.MethodPost || r.Header.Get("Idempotency-Key") == "" {
				t.Error("document request boundary")
			}
			var body map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 || body["markdown"] == nil {
				t.Error("document body")
			}
			if requests == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"ok":false,"error":{"code":"RATE_LIMITED","message":"retry"}}`))
				return
			}
			w.Write([]byte(`{"ok":true,"data":{"id":"document-1"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := client{baseURL: server.URL, key: "test-from-env", http: server.Client(), backoff: func(time.Duration) {}}
	keys, err := c.keys(context.Background())
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys: %v", err)
	}
	_, err = c.document(context.Background(), render("2026-09-20", audit(keys, ReviewInput{
		DigitalAssetDelivery: Workflow{"asset delivery", []string{"asset:read"}},
		SubscriberUpdates:    Workflow{"subscriber updates", []string{"subscriber:write"}},
		ContentProcessing:    Workflow{"content processing", []string{"content:run"}},
	})))
	if err != nil || requests != 2 {
		t.Fatalf("document: %v, requests: %d", err, requests)
	}
}
