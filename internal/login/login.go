package login

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrRejected = errors.New("foundry rejected the login")

type Result struct {
	SetCookies []string
	Redirect   string
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type joinRequest struct {
	Action   string `json:"action"`
	UserId   string `json:"userid"`
	Password string `json:"password"`
}

type joinResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Redirect string `json:"redirect"`
}

func (client *Client) Join(
	ctx context.Context,
	userId string,
	password string,
	cookieHeader string,
) (*Result, error) {
	requestBody, err := json.Marshal(joinRequest{
		Action: "join",
		UserId: userId,
		Password: password,
	})
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.BaseURL+"/join", bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	if cookieHeader != "" {
		request.Header.Set("Cookie", cookieHeader)
	}

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: %s", ErrRejected, strings.TrimSpace(string(responseBody)))
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from foundry", response.StatusCode)
	}

	var joinResult joinResponse
	if err := json.Unmarshal(responseBody, &joinResult); err != nil {
		return nil, fmt.Errorf("decode join response: %w", err)
	}

	if joinResult.Status != "success" {
		return nil, fmt.Errorf("%w: %s", ErrRejected, joinResult.Message)
	}

	return &Result{
		SetCookies: response.Header.Values("Set-Cookie"),
		Redirect:   joinResult.Redirect,
	}, nil
}
