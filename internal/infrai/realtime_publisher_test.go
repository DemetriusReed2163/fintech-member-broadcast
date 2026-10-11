package realtimeapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPublishRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	var calls int
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		var payload PublishRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.IdempotencyKey != "payment-pay_101-settled" {
			t.Fatalf("idempotency key = %q", payload.IdempotencyKey)
		}
		status, body := http.StatusOK, `{"ok":true,"data":{},"error":null,"metadata":{}}`
		if calls == 1 {
			status, body = http.StatusTooManyRequests, `{"ok":false,"data":null,"error":{},"metadata":{}}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{"0"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	publisher, err := NewPublisher("test-key", client)
	if err != nil {
		t.Fatal(err)
	}
	publisher.sleep = func(context.Context, time.Duration) error { return nil }

	err = publisher.Publish(context.Background(), PublishRequest{Channel: "fintech-members", Event: "payment.settled", Data: map[string]string{"payment_id": "pay_101"}, AccountID: "acct_7"}, "payment-pay_101-settled")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}
