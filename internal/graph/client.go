// Package graph is a minimal Microsoft Graph REST client for mail and calendar.
package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

// DefaultBaseURL is the Microsoft Graph v1.0 endpoint.
const DefaultBaseURL = "https://graph.microsoft.com/v1.0"

// ErrNotFound is returned for HTTP 404 responses.
var ErrNotFound = errors.New("not found")

// APIError is a Microsoft Graph error response.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("Graph API error (%d %s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("Graph API error: HTTP %d", e.Status)
}

// Client calls the Microsoft Graph REST API.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	Tokens  oauth2.TokenSource
}

// NewClient returns a Client using the default base URL and http client.
func NewClient(tokens oauth2.TokenSource) *Client {
	return &Client{
		HTTP:    http.DefaultClient,
		BaseURL: DefaultBaseURL,
		Tokens:  tokens,
	}
}

type listResponse[T any] struct {
	Value []T `json:"value"`
}

// do performs an authenticated JSON request against the Graph API.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}

	tok, err := c.Tokens.Token()
	if err != nil {
		var rerr *oauth2.RetrieveError
		if errors.As(err, &rerr) {
			return errors.New("Not authenticated. Run: outlook auth login")
		}
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode >= 400 {
		apiErr := &APIError{Status: resp.StatusCode}
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &envelope) == nil {
			apiErr.Code = envelope.Error.Code
			apiErr.Message = envelope.Error.Message
		}
		return apiErr
	}

	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}
