package matrixbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HomeserverURL  string
	UserID         string
	AccessToken    string
	GovernorRoomID string
}

type Client struct {
	baseURL string
	userID  string
	token   string
	http    *http.Client
	logger  *slog.Logger
}

type Event struct {
	RoomID  string
	Sender  string
	Type    string
	EventID string
	MsgType string
	Body    string
}

func NewClient(cfg Config, logger *slog.Logger) (*Client, error) {
	if strings.TrimSpace(cfg.HomeserverURL) == "" {
		return nil, fmt.Errorf("matrix homeserver URL is required")
	}
	if strings.TrimSpace(cfg.UserID) == "" {
		return nil, fmt.Errorf("matrix user ID is required")
	}
	if strings.TrimSpace(cfg.AccessToken) == "" {
		return nil, fmt.Errorf("matrix access token is required")
	}

	return &Client{
		baseURL: strings.TrimRight(cfg.HomeserverURL, "/"),
		userID:  cfg.UserID,
		token:   cfg.AccessToken,
		http: &http.Client{
			Timeout: 45 * time.Second,
		},
		logger: logger,
	}, nil
}

func (c *Client) UserID() string {
	return c.userID
}

func (c *Client) JoinRoom(ctx context.Context, roomID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/join/%s", url.PathEscape(roomID))
	return c.doJSON(ctx, http.MethodPost, path, map[string]any{}, nil)
}

func (c *Client) LeaveRoom(ctx context.Context, roomID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/leave", url.PathEscape(roomID))
	return c.doJSON(ctx, http.MethodPost, path, map[string]any{}, nil)
}

func (c *Client) ForgetRoom(ctx context.Context, roomID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/forget", url.PathEscape(roomID))
	return c.doJSON(ctx, http.MethodPost, path, map[string]any{}, nil)
}

func (c *Client) SendText(ctx context.Context, roomID, body string) error {
	txnID := strconv.FormatInt(time.Now().UnixNano(), 10)
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", url.PathEscape(roomID), txnID)

	payload := map[string]any{
		"msgtype":        "m.text",
		"body":           body,
		"format":         "org.matrix.custom.html",
		"formatted_body": formatPreBody(body),
	}

	return c.doJSON(ctx, http.MethodPut, path, payload, nil)
}

func formatPreBody(body string) string {
	escaped := html.EscapeString(body)
	return "<pre>" + escaped + "</pre>"
}

func (c *Client) SetTyping(ctx context.Context, roomID string, typing bool, timeout time.Duration) error {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/typing/%s", url.PathEscape(roomID), url.PathEscape(c.userID))

	payload := map[string]any{
		"typing": typing,
	}
	if typing && timeout > 0 {
		payload["timeout"] = timeout.Milliseconds()
	}

	return c.doJSON(ctx, http.MethodPut, path, payload, nil)
}

func (c *Client) CreateRoom(ctx context.Context, name, topic string, invitees []string) (string, error) {
	payload := map[string]any{
		"preset":    "private_chat",
		"name":      name,
		"topic":     topic,
		"invite":    invitees,
		"is_direct": len(invitees) == 1,
	}

	var resp struct {
		RoomID string `json:"room_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/createRoom", payload, &resp); err != nil {
		return "", err
	}
	if resp.RoomID == "" {
		return "", fmt.Errorf("createRoom succeeded but returned empty room_id")
	}
	return resp.RoomID, nil
}

func (c *Client) SyncOnce(ctx context.Context, since string, timeout time.Duration) (string, []Event, error) {
	query := url.Values{}
	if since != "" {
		query.Set("since", since)
	}
	query.Set("timeout", strconv.FormatInt(timeout.Milliseconds(), 10))

	path := "/_matrix/client/v3/sync"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var resp syncResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return since, nil, err
	}

	events := make([]Event, 0)
	for roomID, joined := range resp.Rooms.Join {
		for _, raw := range joined.Timeline.Events {
			if raw.Type != "m.room.message" {
				continue
			}
			events = append(events, Event{
				RoomID:  roomID,
				Sender:  raw.Sender,
				Type:    raw.Type,
				EventID: raw.EventID,
				MsgType: raw.Content.MsgType,
				Body:    raw.Content.Body,
			})
		}
	}

	return resp.NextBatch, events, nil
}

type syncResponse struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join map[string]struct {
			Timeline struct {
				Events []struct {
					Type    string `json:"type"`
					Sender  string `json:"sender"`
					EventID string `json:"event_id"`
					Content struct {
						MsgType string `json:"msgtype"`
						Body    string `json:"body"`
					} `json:"content"`
				} `json:"events"`
			} `json:"timeline"`
		} `json:"join"`
	} `json:"rooms"`
}

func (c *Client) doJSON(ctx context.Context, method, path string, requestBody any, responseBody any) error {
	fullURL := c.baseURL + path

	var bodyReader io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal request JSON: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("matrix API request failed: %w", err)
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read matrix API response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("matrix API %s %s failed with status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(responseData)))
	}

	if responseBody != nil && len(responseData) > 0 {
		if err := json.Unmarshal(responseData, responseBody); err != nil {
			return fmt.Errorf("decode matrix API response: %w", err)
		}
	}

	return nil
}
