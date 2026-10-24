package fcm

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInvalidFCMTokenOnlyForDeadDevices(t *testing.T) {
	if !invalidFCMToken([]byte(`{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`)) {
		t.Fatal("UNREGISTERED must delete the token")
	}
	if invalidFCMToken([]byte(`{"error":{"status":"INVALID_ARGUMENT","message":"bad data payload"}}`)) {
		t.Fatal("a malformed payload must not delete a valid token")
	}
}

func TestSendShapeAndDeadTokens(t *testing.T) {
	var mu sync.Mutex
	got := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/projects/p1/messages:send" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Message map[string]any `json:"message"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		tok := body.Message["token"].(string)
		mu.Lock()
		got[tok] = body.Message
		mu.Unlock()
		switch tok {
		case "dead":
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`))
		case "flaky":
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	f := &FCM{client: srv.Client(), project: "p1", base: srv.URL}
	dead := f.Send(context.Background(), []string{"ok", "dead", "flaky"}, PushMessage{Title: "T", Body: "B", ImageURL: "http://img", Data: map[string]string{"id": "1"}})

	if len(dead) != 1 || dead[0] != "dead" {
		t.Fatalf("only UNREGISTERED tokens are dead, got %v", dead)
	}
	n := got["ok"]["notification"].(map[string]any)
	if n["title"] != "T" || n["body"] != "B" || n["image"] != "http://img" || got["ok"]["data"].(map[string]any)["id"] != "1" {
		t.Fatalf("bad payload: %v", got["ok"])
	}
}

func TestNewFCMFromServiceAccount(t *testing.T) {
	if f, err := NewFCM(context.Background(), ""); f != nil || err != nil {
		t.Fatal("empty path must disable push")
	}
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	sa, _ := json.Marshal(map[string]string{"type": "service_account", "project_id": "p1", "private_key": string(pemKey), "client_email": "x@p1.iam.gserviceaccount.com", "token_uri": "https://oauth2.googleapis.com/token"})
	path := filepath.Join(t.TempDir(), "sa.json")
	_ = os.WriteFile(path, sa, 0o600)
	f, err := NewFCM(context.Background(), path)
	if err != nil || f.project != "p1" || f.base != fcmBase {
		t.Fatalf("NewFCM: %v %+v", err, f)
	}
}
