// https://docs.github.com/en/rest/users/users#get-a-user
// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type notFound string

func (n notFound) Error() string { return string(n) }

type Client struct {
	Base string
	HTTP *http.Client
}

func New() *Client {
	return &Client{Base: "https://api.github.com", HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) get(kind, name, missing string, v any) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("%q isn't a GitHub name", name)
	}
	return c.call(http.MethodGet, "/"+kind+"/"+url.PathEscape(name), "", missing, v)
}

func (c *Client) call(method, path, jwt, missing string, v any) error {
	req, err := http.NewRequest(method, c.Base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return json.NewDecoder(resp.Body).Decode(v)
	case http.StatusNotFound:
		return notFound(missing)
	case http.StatusForbidden, http.StatusTooManyRequests:
		if resp.Header.Get("X-Ratelimit-Remaining") == "0" {
			reset, _ := strconv.ParseInt(resp.Header.Get("X-Ratelimit-Reset"), 10, 64)
			return fmt.Errorf("GitHub's rate limit is used up; try again after %s", time.Unix(reset, 0).UTC().Format(time.RFC3339))
		}
	}
	return fmt.Errorf("GitHub answered %s for %s", resp.Status, path)
}

func (c *Client) User(name string) (string, error) {
	var user struct{ Login string }
	if err := c.get("users", name, fmt.Sprintf("no GitHub account %q", name), &user); err != nil {
		return "", err
	}
	if user.Login == "" {
		return "", fmt.Errorf("GitHub gave no account for %q", name)
	}
	return user.Login, nil
}
