package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	return d.Decode(dst)
}

func handler(g *gate, adminToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		var in signupInput
		if err := decode(r, &in); err != nil {
			reply(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		result, err := g.signup(r.Context(), in, ip)
		if err != nil {
			var remote *apiError
			switch {
			case err.Error() == "account already exists":
				reply(w, 409, map[string]string{"error": err.Error()})
			case errors.As(err, &remote) && remote.Status >= 400 && remote.Status < 500:
				reply(w, http.StatusUnprocessableEntity, map[string]string{"error": "signup rejected", "code": remote.Code})
			case errors.As(err, &remote):
				reply(w, 502, map[string]string{"error": "upstream request failed"})
			case strings.Contains(err.Error(), "required"), err.Error() == "unknown tenant":
				reply(w, 400, map[string]string{"error": err.Error()})
			default:
				reply(w, 502, map[string]string{"error": "upstream request failed"})
			}
			return
		}
		reply(w, 201, result)
	})
	admin := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "Bearer "+adminToken {
			reply(w, 401, map[string]string{"error": "unauthorized"})
			return false
		}
		return true
	}
	mux.HandleFunc("POST /admin/tenants", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) {
			return
		}
		var in struct {
			Name string `json:"name"`
		}
		if decode(r, &in) != nil || in.Name == "" {
			reply(w, 400, map[string]string{"error": "name required"})
			return
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.tenants[in.Name] != nil {
			reply(w, 409, map[string]string{"error": "tenant exists"})
			return
		}
		g.tenants[in.Name] = &tenant{Name: in.Name, Accounts: make(map[string]account)}
		reply(w, 201, g.tenants[in.Name])
	})
	mux.HandleFunc("GET /admin/tenants/{tenant}", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) {
			return
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		t := g.tenants[r.PathValue("tenant")]
		if t == nil {
			reply(w, 404, map[string]string{"error": "tenant not found"})
			return
		}
		reply(w, 200, t)
	})
	mux.HandleFunc("POST /admin/tenants/{tenant}/accounts/{email}/suspend", func(w http.ResponseWriter, r *http.Request) {
		if !admin(w, r) {
			return
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		t := g.tenants[r.PathValue("tenant")]
		if t == nil {
			reply(w, 404, map[string]string{"error": "tenant not found"})
			return
		}
		a, found := t.Accounts[r.PathValue("email")]
		if !found {
			reply(w, 404, map[string]string{"error": "account not found"})
			return
		}
		a.State = "suspended"
		t.Accounts[a.Email] = a
		reply(w, 200, a)
	})
	return mux
}

func main() {
	if os.Getenv("INFRAI_API_KEY") == "" || os.Getenv("ADMIN_TOKEN") == "" {
		log.Fatal("set INFRAI_API_KEY and ADMIN_TOKEN")
	}
	g := &gate{tenants: make(map[string]*tenant), api: newClient()}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Fatal(http.ListenAndServe(addr, handler(g, os.Getenv("ADMIN_TOKEN"))))
}
