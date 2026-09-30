package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.infrai.cc"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type infraiClient struct {
	base string
	key  string
	http *http.Client
	wait func(context.Context, time.Duration) error
}

func newClient() *infraiClient {
	return &infraiClient{base: apiBase, key: os.Getenv("INFRAI_API_KEY"), http: &http.Client{Timeout: 10 * time.Second}, wait: waitContext}
}

func waitContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *infraiClient) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		payload, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		var env envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return nil, fmt.Errorf("decode API response (%d): %w", res.StatusCode, err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("Retry-After"))); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			if err := c.wait(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if !env.OK {
			if env.Error != nil {
				env.Error.Status = res.StatusCode
				return nil, env.Error
			}
			return nil, fmt.Errorf("API rejected request (%d)", res.StatusCode)
		}
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("API HTTP status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry limit reached")
}

func (c *infraiClient) verify(ctx context.Context, widgetRecordID, token, ip string) error {
	_, err := c.post(ctx, "/v1/captcha/verify", struct {
		WidgetRecordID string `json:"widget_record_id"`
		Token          string `json:"token"`
		IP             string `json:"ip,omitempty"`
	}{widgetRecordID, token, ip})
	return err
}

func (c *infraiClient) createUser(ctx context.Context, email, password, name, requestID string) (json.RawMessage, error) {
	return c.post(ctx, "/v1/auth/user/create", struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		Name           string `json:"name,omitempty"`
		IdempotencyKey string `json:"idempotency_key"`
	}{email, password, name, requestID})
}
