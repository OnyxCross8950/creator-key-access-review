package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Key struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type Workflow struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type ReviewInput struct {
	DigitalAssetDelivery Workflow `json:"digital_asset_delivery"`
	SubscriberUpdates    Workflow `json:"subscriber_updates"`
	ContentProcessing    Workflow `json:"content_processing"`
}

type Reach struct {
	Workflow string   `json:"workflow"`
	Allowed  bool     `json:"allowed"`
	Missing  []string `json:"missing"`
}

type KeyReview struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Reach []Reach `json:"reach"`
}

func audit(keys []Key, input ReviewInput) []KeyReview {
	workflows := []Workflow{input.DigitalAssetDelivery, input.SubscriberUpdates, input.ContentProcessing}
	result := make([]KeyReview, 0, len(keys))
	for _, key := range keys {
		have := make(map[string]bool, len(key.Scopes))
		for _, scope := range key.Scopes {
			have[scope] = true
		}
		review := KeyReview{ID: key.ID, Name: key.Name}
		for _, workflow := range workflows {
			missing := []string{}
			for _, scope := range workflow.Scopes {
				if !have[scope] {
					missing = append(missing, scope)
				}
			}
			review.Reach = append(review.Reach, Reach{Workflow: workflow.Name, Allowed: len(missing) == 0, Missing: missing})
		}
		result = append(result, review)
	}
	return result
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error apiError        `json:"error"`
}

type rejection struct {
	Status int
	Detail apiError
}

func (e rejection) Error() string { return fmt.Sprintf("%s: %s", e.Detail.Code, e.Detail.Message) }

type client struct {
	baseURL string
	key     string
	http    *http.Client
	backoff func(time.Duration)
}

func (c client) call(ctx context.Context, method, path string, body any, idempotency string) (json.RawMessage, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotency != "" {
			req.Header.Set("Idempotency-Key", idempotency)
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		contents, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		// Decode the envelope before classifying HTTP 4xx responses.
		var env envelope
		if err := json.Unmarshal(contents, &env); err != nil {
			return nil, fmt.Errorf("HTTP %d: invalid response: %w", res.StatusCode, err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			c.backoff(delay)
			continue
		}
		if !env.OK {
			return nil, rejection{Status: res.StatusCode, Detail: env.Error}
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("HTTP %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func (c client) keys(ctx context.Context) ([]Key, error) {
	data, err := c.call(ctx, http.MethodGet, "/v1/account/keys/list", nil, "")
	if err != nil {
		return nil, err
	}
	var result struct {
		Items []Key `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode keys: %w", err)
	}
	return result.Items, nil
}

func (c client) document(ctx context.Context, markdown string) (json.RawMessage, error) {
	digest := sha256.Sum256([]byte(markdown))
	return c.call(ctx, http.MethodPost, "/v1/pdf/generate", map[string]any{"markdown": markdown}, hex.EncodeToString(digest[:]))
}

func render(date string, reviews []KeyReview) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Creator access review - %s\n\n", date)
	for _, key := range reviews {
		fmt.Fprintf(&out, "## %s (%s)\n", key.Name, key.ID)
		for _, reach := range key.Reach {
			if reach.Allowed {
				fmt.Fprintf(&out, "- %s: reachable\n", reach.Workflow)
			} else {
				fmt.Fprintf(&out, "- %s: missing %s\n", reach.Workflow, strings.Join(reach.Missing, ", "))
			}
		}
	}
	return out.String()
}
