// Package client is a Go port of the reference Python implementation's
// switch_client.py (see ~/code/experiments/hpe/switch_client.py).
package client

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
)

type Client struct {
	BaseURL    string
	httpClient *http.Client
	password   string
	dirty      atomic.Bool

	// AdminPort is the interface number that resource_port.go's write
	// methods refuse to touch, regardless of what's requested -- protects
	// whichever port is the switch's uplink/management path. Set once by
	// provider.Configure(), read by resource_port.go's guard.
	AdminPort int64
}

func NewClient(host string, https bool, password string) (*Client, error) {
	scheme := "http"
	if https {
		scheme = "https"
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}
	return &Client{
		BaseURL:    fmt.Sprintf("%s://%s", scheme, host),
		httpClient: &http.Client{Jar: jar},
		password:   password,
	}, nil
}

func (c *Client) resolveURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return c.BaseURL + path
	}
	return c.BaseURL + "/" + path
}

// Login POSTs to /hp_login.html and establishes the session cookie.
//
// A bare POST occasionally comes back without setting SID (the switch seems
// to want a session cookie already present, even an empty one, before it
// will issue a real one) -- a GET first makes login reliable.
//
// Unlike the Python version, there's no on-disk session cache here: this
// provider process lives for the whole plan/apply run, so Configure() logs
// in once and every resource/data source reuses the same *Client.
func (c *Client) Login() error {
	if _, err := c.httpClient.Get(c.resolveURL("/")); err != nil {
		return fmt.Errorf("priming session: %w", err)
	}

	form := url.Values{
		"pwd":      {c.password},
		"login":    {"Login"},
		"err_flag": {""},
		"err_msg":  {""},
	}
	resp, err := c.httpClient.PostForm(c.resolveURL("/hp_login.html"), form)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading login response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login request returned status %d", resp.StatusCode)
	}
	if strings.Contains(strings.ToLower(string(body)), "<title>login</title>") {
		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		return fmt.Errorf("login failed, response still looks like the login page: %q", snippet)
	}
	return nil
}

// Logout releases the switch's one active session slot (GET
// /index.html?logout=1, found by probing -- this firmware has no documented
// logout endpoint). Best-effort: call it when the provider process is
// shutting down so the *next* `tofu` invocation doesn't have to wait out the
// switch's own session timeout (see Web Parameters -> Session Timeout, ~5
// min) before it can log in.
func (c *Client) Logout() error {
	resp, err := c.httpClient.Get(c.resolveURL("/index.html?logout=1"))
	if err != nil {
		return fmt.Errorf("logout request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

// Fetch GETs BaseURL+path (path may be relative or absolute) and returns the body.
func (c *Client) Fetch(path string) (string, error) {
	resp, err := c.httpClient.Get(c.resolveURL(path))
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response for %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s returned status %d", path, resp.StatusCode)
	}
	return string(body), nil
}

var errFlagRejectedRe = regexp.MustCompile(`(?i)name="err_flag"[^>]*value="1"`)
var errMsgRe = regexp.MustCompile(`(?i)name="err_msg"[^>]*value="([^"]*)"`)

// postForm POSTs exactly the fields given (caller supplies everything,
// including submit_flag) and returns the raw response body. Low-level
// building block shared by SubmitForm (submit_flag=8, the common case) and
// port_config.go's reloadPortConfig (submit_flag=1, to select a port
// without applying any change).
func (c *Client) postForm(path string, fields map[string]string) (string, error) {
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}

	resp, err := c.httpClient.PostForm(c.resolveURL(path), form)
	if err != nil {
		return "", fmt.Errorf("posting to %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response for %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("posting to %s returned status %d", path, resp.StatusCode)
	}
	return string(body), nil
}

// SubmitForm POSTs fields to path, merged with the firmware's standard
// bookkeeping fields (submit_flag, submit_target, err_flag, err_msg,
// clazz_information), and returns the response body.
//
// Returns an error if the response's err_flag comes back "1" -- the
// firmware's own validation-failure signal. NOTE: most value validation in
// this firmware happens client-side in JS, not on the server -- an invalid
// enum value, for example, is just silently ignored (state left unchanged)
// rather than reported via err_flag. So this check catches *some* rejected
// writes, but a caller that needs certainty should read the value back
// afterwards and compare, rather than trusting the absence of an error.
func (c *Client) SubmitForm(path string, fields map[string]string) (string, error) {
	target := strings.TrimPrefix(path, "/")
	merged := map[string]string{
		// 8 == xui_operation_submit (per the firmware's own _xe_jsvars.js);
		// 1 is xui_operation_reload and silently applies nothing.
		"submit_flag":       "8",
		"submit_target":     target,
		"err_flag":          "0",
		"err_msg":           "",
		"clazz_information": target,
	}
	for k, v := range fields {
		merged[k] = v
	}

	bodyStr, err := c.postForm(path, merged)
	if err != nil {
		return "", err
	}

	if errFlagRejectedRe.MatchString(bodyStr) {
		msg := "unknown error"
		if m := errMsgRe.FindStringSubmatch(bodyStr); m != nil {
			msg = m[1]
		}
		return "", fmt.Errorf("switch rejected write to %q: %s", path, msg)
	}
	c.dirty.Store(true)
	return bodyStr, nil
}

// Dirty reports whether any SubmitForm call has succeeded yet during this
// Client's lifetime -- used by the provider to decide whether a
// save_running_config=true shutdown save is actually warranted (no point
// wearing the switch's flash on a plan-only run that wrote nothing).
func (c *Client) Dirty() bool {
	return c.dirty.Load()
}

const SaveAllChangesPath = "/SaveAllChanges.html"

// SaveRunningConfig persists the switch's running configuration to its
// non-volatile config file (the web UI's "Save Configuration" button) --
// without this, config changes made via SubmitForm are lost on the next
// reboot/power cycle. The page has no editable fields, just a submit
// button, so this posts the baseline fields only.
func (c *Client) SaveRunningConfig() error {
	_, err := c.SubmitForm(SaveAllChangesPath, map[string]string{})
	return err
}
