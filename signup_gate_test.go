package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAPI struct {
	reject  bool
	creates int
}

func (f *fakeAPI) verify(context.Context, string, string, string) error {
	if f.reject {
		return &apiError{Code: "CAPTCHA_SCORE_TOO_LOW", Status: 422}
	}
	return nil
}
func (f *fakeAPI) createUser(context.Context, string, string, string, string) (json.RawMessage, error) {
	f.creates++
	return json.RawMessage(`{"id":"user-1"}`), nil
}

func TestSignupDecision(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reject  bool
		state   string
		creates int
	}{
		{"accepted", false, "active", 1},
		{"rejected", true, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{reject: tc.reject}
			g := &gate{api: api, tenants: map[string]*tenant{"acme": {Name: "acme", Accounts: map[string]account{}}}}
			a, err := g.signup(context.Background(), signupInput{Tenant: "acme", Email: "dev@example.com", Password: "secret", WidgetRecordID: "widget-1", CaptchaToken: "captcha", RequestID: "signup-1"}, "127.0.0.1")
			if tc.reject {
				var remote *apiError
				if !errors.As(err, &remote) || remote.Status != 422 {
					t.Fatalf("expected captcha rejection, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if a.State != tc.state || api.creates != tc.creates || len(g.tenants["acme"].Accounts) != tc.creates {
				t.Fatalf("account=%+v creates=%d", a, api.creates)
			}
		})
	}
}

func TestBusinessRejectionFromHTTP422(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/captcha/verify" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["widget_record_id"] != "widget-1" || body["token"] != "token" || body["ip"] != "127.0.0.1" {
			t.Errorf("unexpected captcha request: %#v", body)
		}
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"CAPTCHA_SCORE_TOO_LOW","message":"score below threshold"}}`))
	}))
	defer server.Close()
	c := &infraiClient{base: server.URL, key: "test", http: server.Client(), wait: waitContext}
	err := c.verify(context.Background(), "widget-1", "token", "127.0.0.1")
	var remote *apiError
	if !errors.As(err, &remote) || remote.Code != "CAPTCHA_SCORE_TOO_LOW" || remote.Status != 422 {
		t.Fatalf("lost business rejection: %v", err)
	}
	if strings.Contains(err.Error(), "decode") {
		t.Fatal(err)
	}
}

func TestDuplicateSignupDoesNotExposeAccount(t *testing.T) {
	api := &fakeAPI{}
	g := &gate{api: api, tenants: map[string]*tenant{"acme": {Name: "acme", Accounts: map[string]account{
		"dev@example.com": {Email: "dev@example.com", State: "active", User: json.RawMessage(`{"id":"user-1"}`)},
	}}}}
	a, err := g.signup(context.Background(), signupInput{Tenant: "acme", Email: "dev@example.com", Password: "secret", WidgetRecordID: "widget-1", CaptchaToken: "token", RequestID: "another-request"}, "127.0.0.1")
	if err == nil || err.Error() != "account already exists" || a.User != nil || api.creates != 0 {
		t.Fatalf("duplicate result=%+v err=%v creates=%d", a, err, api.creates)
	}
}
