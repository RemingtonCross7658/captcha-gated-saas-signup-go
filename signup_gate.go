package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

type signupInput struct {
	Tenant         string `json:"tenant"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	Name           string `json:"name"`
	WidgetRecordID string `json:"widget_record_id"`
	CaptchaToken   string `json:"captcha_token"`
	RequestID      string `json:"request_id"`
}

type account struct {
	Email string          `json:"email"`
	State string          `json:"state"`
	User  json.RawMessage `json:"user"`
}

type tenant struct {
	Name     string             `json:"name"`
	Accounts map[string]account `json:"accounts"`
}

type signupAPI interface {
	verify(context.Context, string, string, string) error
	createUser(context.Context, string, string, string, string) (json.RawMessage, error)
}

type gate struct {
	mu      sync.Mutex
	tenants map[string]*tenant
	api     signupAPI
}

func (g *gate) signup(ctx context.Context, in signupInput, ip string) (account, error) {
	if in.Tenant == "" || in.Email == "" || in.Password == "" || in.WidgetRecordID == "" || in.CaptchaToken == "" || in.RequestID == "" {
		return account{}, errors.New("tenant, email, password, widget_record_id, captcha_token and request_id are required")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	t := g.tenants[in.Tenant]
	if t == nil {
		return account{}, errors.New("unknown tenant")
	}
	if _, ok := t.Accounts[in.Email]; ok {
		return account{}, errors.New("account already exists")
	}
	if err := g.api.verify(ctx, in.WidgetRecordID, in.CaptchaToken, ip); err != nil {
		return account{}, err
	}
	user, err := g.api.createUser(ctx, in.Email, in.Password, in.Name, in.RequestID)
	if err != nil {
		return account{}, err
	}
	created := account{Email: in.Email, State: "active", User: user}
	t.Accounts[in.Email] = created
	return created, nil
}
