package github

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newKey(t *testing.T) (*rsa.PrivateKey, []byte, []byte) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(k)
	return k, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
}

func TestJWTIsWhatGitHubAsksFor(t *testing.T) {
	k, pkcs1, pkcs8 := newKey(t)
	now := time.Unix(1_800_000_000, 0)
	for _, key := range [][]byte{pkcs1, pkcs8} {
		token, err := JWT(42, key, now)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(token, ".")
		header, _ := base64.RawURLEncoding.DecodeString(parts[0])
		body, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims struct {
			Iat, Exp int64
			Iss      string
		}
		json.Unmarshal(body, &claims)
		if string(header) != `{"alg":"RS256","typ":"JWT"}` || claims.Iat != now.Unix()-60 || claims.Exp != now.Unix()+540 || claims.Iss != "42" {
			t.Fatalf("header %s, claims %+v", header, claims)
		}
		sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
		sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err := rsa.VerifyPKCS1v15(&k.PublicKey, crypto.SHA256, sum[:], sig); err != nil {
			t.Fatalf("signature: %v", err)
		}
	}
}

func TestJWTRefusesWhatIsNotAKeyWithoutShowingIt(t *testing.T) {
	if _, err := JWT(42, []byte("secret-looking text"), time.Now()); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error = %v", err)
	}
}

func TestAppSignsInAndReturnsItsNameAndOwner(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app" || r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"id":42,"slug":"someone-agents","owner":{"login":"Someone"}}`))
	}))
	defer s.Close()
	c := &Client{Base: s.URL, HTTP: s.Client()}
	app, err := c.App("good")
	if err != nil || app.ID != 42 || app.Slug != "someone-agents" || app.Owner != "Someone" {
		t.Fatalf("App = %+v, %v", app, err)
	}
	if _, err := c.App("bad"); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("bad key: %v", err)
	}
}

func TestInstallationIsFoundSignedAsTheApp(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Header.Get("Authorization") != "Bearer good":
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.EscapedPath() == "/users/someone/installation":
			w.Write([]byte(`{"id":99,"account":{"login":"someone"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()
	c := &Client{Base: s.URL, HTTP: s.Client()}
	if id, err := c.Installation("good", "someone"); err != nil || id != 99 {
		t.Fatalf("Installation = %d, %v", id, err)
	}
	if _, err := c.Installation("good", "other"); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("not installed: %v", err)
	}
	if _, err := c.Installation("bad", "someone"); err == nil || errors.Is(err, ErrNotInstalled) {
		t.Fatalf("bad JWT: %v", err)
	}
}
