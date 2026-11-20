package github

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func server(t *testing.T) *Client {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case "/users/someone":
			w.Write([]byte(`{"login":"Someone","type":"User"}`))
		case "/apps/someone-agents":
			w.Write([]byte(`{"id":42,"slug":"someone-agents","name":"someone-agents","owner":{"login":"Someone"}}`))
		case "/apps/borrowed":
			w.Write([]byte(`{"id":7,"slug":"borrowed","owner":{"login":"other"}}`))
		case "/users/busy":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Close)
	return &Client{Base: s.URL, HTTP: s.Client()}
}

func TestUserReturnsGitHubsSpellingOfTheLogin(t *testing.T) {
	login, err := server(t).User("someone")
	if err != nil || login != "Someone" {
		t.Fatalf("User = %q, %v", login, err)
	}
}

func TestUserSaysWhenNoSuchAccountExists(t *testing.T) {
	_, err := server(t).User("nobody")
	if err == nil || err.Error() != `no GitHub account "nobody"` {
		t.Fatalf("User(nobody) error = %v", err)
	}
	if _, err := server(t).User("busy"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("User(busy) error = %v; want the status", err)
	}
}

func TestAppReturnsTheIDOfAnAppTheUserOwns(t *testing.T) {
	app, err := server(t).App("someone-agents", "Someone")
	if err != nil || app.ID != 42 || app.Name != "someone-agents" {
		t.Fatalf("App = %+v, %v", app, err)
	}
}

func TestAppRefusesAnAppSomeoneElseOwnsOrNoApp(t *testing.T) {
	if _, err := server(t).App("borrowed", "Someone"); err == nil || !strings.Contains(err.Error(), `owned by "other"`) {
		t.Errorf("borrowed error = %v", err)
	}
	if _, err := server(t).App("missing", "Someone"); err == nil || !strings.Contains(err.Error(), `no GitHub App "missing"`) {
		t.Errorf("missing error = %v", err)
	}
}

func TestNamesAreEscapedInThePath(t *testing.T) {
	if _, err := server(t).User("someone/../apps/borrowed"); err == nil {
		t.Fatal("a slash in a name reached another endpoint")
	}
}
