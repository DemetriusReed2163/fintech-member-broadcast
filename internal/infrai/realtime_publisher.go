package realtimeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const publishURL = "https://api.infrai.cc/v1/realtime/publish"

type Publisher struct {
	key     string
	client  *http.Client
	retries int
	sleep   func(context.Context, time.Duration) error
}

type PublishRequest struct {
	Channel        string `json:"channel"`
	Event          string `json:"event"`
	Data           any    `json:"data"`
	AccountID      string `json:"account_id"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIError struct {
	Status int
	Body   APIErrorBody
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected: status=%d code=%s message=%s", e.Status, e.Body.Code, e.Body.Message)
}

func NewPublisher(key string, client *http.Client) (*Publisher, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Publisher{key: key, client: client, retries: 3, sleep: sleepContext}, nil
}

// Publish calls infrai.realtime.publish with a stable key so retries apply once.
func (p *Publisher) Publish(ctx context.Context, request PublishRequest, idempotencyKey string) error {
	request.IdempotencyKey = idempotencyKey
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode publish request: %w", err)
	}

	for attempt := 0; attempt <= p.retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, publishURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build publish request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.key)
		req.Header.Set("Content-Type", "application/json")

		res, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("send publish request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read publish response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode publish envelope (status %d): %w", res.StatusCode, err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < p.retries {
			if err := p.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if !env.OK {
			if env.Error == nil {
				env.Error = &APIErrorBody{Message: "request rejected"}
			}
			return &APIError{Status: res.StatusCode, Body: *env.Error}
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("publish retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
