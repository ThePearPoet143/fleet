// https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
// https://docs.github.com/en/rest/apps/apps#get-the-authenticated-app
// https://docs.github.com/en/rest/apps/apps#get-a-user-installation-for-the-authenticated-app
// https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app
// https://pkg.go.dev/crypto/rsa#SignPKCS1v15
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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var ErrNotInstalled = errors.New("the app isn't installed on this account")

type App struct {
	ID          int64
	Slug, Owner string
}

func privateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("the key isn't a PEM file")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if rsaKey, ok := k.(*rsa.PrivateKey); err == nil && ok {
		return rsaKey, nil
	}
	return nil, errors.New("the key isn't an RSA private key")
}

func JWT(appID int64, pemBytes []byte, now time.Time) (string, error) {
	key, err := privateKey(pemBytes)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString
	claims, _ := json.Marshal(struct {
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
		Iss string `json:"iss"`
	}{now.Unix() - 60, now.Unix() + 540, strconv.FormatInt(appID, 10)})
	unsigned := enc([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + enc(claims)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + enc(sig), nil
}

func (c *Client) App(jwt string) (App, error) {
	var app struct {
		ID    int64
		Slug  string
		Owner struct{ Login string }
	}
	if err := c.call(http.MethodGet, "/app", jwt, "GitHub found no app for this key", &app); err != nil {
		return App{}, err
	}
	return App{app.ID, app.Slug, app.Owner.Login}, nil
}

func (c *Client) Installation(jwt, user string) (int64, error) {
	var inst struct{ ID int64 }
	err := c.call(http.MethodGet, "/users/"+url.PathEscape(user)+"/installation", jwt, "not installed", &inst)
	var missing notFound
	if errors.As(err, &missing) {
		return 0, ErrNotInstalled
	}
	if err == nil && inst.ID == 0 {
		err = errors.New("GitHub gave no installation")
	}
	return inst.ID, err
}

func (c *Client) Token(jwt string, installation int64) (string, error) {
	var answer struct{ Token string }
	path := "/app/installations/" + strconv.FormatInt(installation, 10) + "/access_tokens"
	if err := c.call(http.MethodPost, path, jwt, fmt.Sprintf("GitHub has no installation %d for this app", installation), &answer); err != nil {
		return "", err
	}
	if answer.Token == "" {
		return "", errors.New("GitHub answered with no token")
	}
	return answer.Token, nil
}

func CheckKey(pemBytes []byte) error {
	_, err := privateKey(pemBytes)
	return err
}
