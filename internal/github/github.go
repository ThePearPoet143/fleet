// https://docs.github.com/en/rest/users/users#get-a-user
// https://docs.github.com/en/rest/apps/apps#get-an-app
package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	Base string
	HTTP *http.Client
}

type App struct {
	ID   int64
	Name string
}

func New() *Client {
	return &Client{Base: "https://api.github.com", HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) get(path, missing string, v any) error {
	req, err := http.NewRequest(http.MethodGet, c.Base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(resp.Body).Decode(v)
	case http.StatusNotFound:
		return fmt.Errorf("%s", missing)
	}
	return fmt.Errorf("GitHub answered %s for %s", resp.Status, path)
}

func (c *Client) User(name string) (string, error) {
	var user struct{ Login string }
	if err := c.get("/users/"+url.PathEscape(name), fmt.Sprintf("no GitHub account %q", name), &user); err != nil {
		return "", err
	}
	return user.Login, nil
}

func (c *Client) App(slug, owner string) (App, error) {
	var app struct {
		ID    int64
		Name  string
		Owner struct{ Login string }
	}
	missing := fmt.Sprintf("no GitHub App %q: make one first, then rerun setup", slug)
	if err := c.get("/apps/"+url.PathEscape(slug), missing, &app); err != nil {
		return App{}, err
	}
	if app.Owner.Login != owner {
		return App{}, fmt.Errorf("the GitHub App %q is owned by %q, not %q", slug, app.Owner.Login, owner)
	}
	return App{app.ID, app.Name}, nil
}
