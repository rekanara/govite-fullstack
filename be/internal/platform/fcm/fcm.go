package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type PushMessage struct {
	Title    string
	Body     string
	ImageURL string
	Data     map[string]string
}

// Pusher sends push messages. It returns tokens the provider reported dead,
// so the caller can delete them.
type Pusher interface {
	Send(ctx context.Context, tokens []string, m PushMessage) (invalid []string)
}

// FCM talks to the Firebase HTTP v1 API with a service account.
type FCM struct {
	client  *http.Client
	project string
}

// NewFCM returns nil when path is empty (push disabled).
func NewFCM(ctx context.Context, path string) (*FCM, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	creds, err := google.CredentialsFromJSON(ctx, raw, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return nil, err
	}
	var sa struct {
		ProjectID string `json:"project_id"`
	}
	_ = json.Unmarshal(raw, &sa)
	if sa.ProjectID == "" {
		return nil, fmt.Errorf("fcm: project_id missing in service account")
	}
	return &FCM{client: oauth2.NewClient(ctx, creds.TokenSource), project: sa.ProjectID}, nil
}

const fcmConcurrency = 16

// Send posts one request per token (v1 has no multicast), 16 at a time.
func (f *FCM) Send(ctx context.Context, tokens []string, m PushMessage) []string {
	url := "https://fcm.googleapis.com/v1/projects/" + f.project + "/messages:send"
	var (
		mu      sync.Mutex
		invalid []string
		wg      sync.WaitGroup
		sem     = make(chan struct{}, fcmConcurrency)
	)
	for _, tok := range tokens {
		wg.Add(1)
		sem <- struct{}{}
		go func(tok string) {
			defer func() { <-sem; wg.Done() }()
			notification := map[string]string{"title": m.Title, "body": m.Body}
			if m.ImageURL != "" {
				notification["image"] = m.ImageURL
			}
			payload, _ := json.Marshal(map[string]any{"message": map[string]any{"token": tok, "notification": notification, "data": m.Data}})
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			res, err := f.client.Do(req)
			if err != nil {
				slog.ErrorContext(ctx, "fcm send failed", "err", err)
				return
			}
			defer res.Body.Close()
			if res.StatusCode >= 300 {
				body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
				if invalidFCMToken(body) {
					mu.Lock()
					invalid = append(invalid, tok)
					mu.Unlock()
				} else {
					slog.ErrorContext(ctx, "fcm send rejected", "status", res.StatusCode, "body", string(body))
				}
			}
		}(tok)
	}
	wg.Wait()
	return invalid
}

func invalidFCMToken(body []byte) bool {
	var res struct {
		Error struct {
			Status  string `json:"status"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &res) != nil {
		return false
	}
	if res.Error.Status == "UNREGISTERED" {
		return true
	}
	for _, d := range res.Error.Details {
		if d.ErrorCode == "UNREGISTERED" {
			return true
		}
	}
	return false
}

// Memory records pushes; used by tests.
type Memory struct {
	mu   sync.Mutex
	Sent [][]string
}

func (m *Memory) Send(_ context.Context, tokens []string, _ PushMessage) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sent = append(m.Sent, tokens)
	return nil
}
