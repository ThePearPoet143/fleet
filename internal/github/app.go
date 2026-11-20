// https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
// https://docs.github.com/en/rest/apps/apps#get-the-authenticated-app
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
	"strconv"
	"time"
)

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
	if err := c.call("/app", jwt, "GitHub found no app for this key", &app); err != nil {
		return App{}, err
	}
	return App{app.ID, app.Slug, app.Owner.Login}, nil
}
