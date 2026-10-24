// Package mail sends branded transactional email over SMTP.
package mail

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

type Content struct {
	Preheader  string
	Heading    string
	Paragraphs []string
	ButtonText string
	ButtonURL  string
	Code       string // one-time code, shown large
	Note       string // small muted print
}

type Message struct {
	To      string
	Subject string
	Text    string   // plain-text alternative
	Content *Content // rendered into the HTML layout
}

type Sender interface {
	Send(ctx context.Context, m Message) error
}

// Brand is the slice of app settings that styles an email.
type Brand struct {
	AppName      string
	Tagline      string
	LogoURL      string
	SupportEmail string
	SupportPhone string
	EmailFooter  string
	Address      string
}

type SMTP struct {
	addr, host, user, pass, from string
	brand                        func(context.Context) Brand
}

// NewSMTP returns a Sender. An empty host makes Send fail loudly rather than drop mail.
func NewSMTP(host string, port int, user, pass, from string, brand func(context.Context) Brand) *SMTP {
	return &SMTP{addr: net.JoinHostPort(host, fmt.Sprint(port)), host: host, user: user, pass: pass, from: from, brand: brand}
}

func (s *SMTP) Send(ctx context.Context, m Message) error {
	if s.host == "" {
		return fmt.Errorf("mail: SMTP_HOST not configured")
	}
	b := s.brand(ctx)
	text := m.Text
	if b.EmailFooter != "" {
		text += "\n\n" + b.EmailFooter
	} else if b.SupportEmail != "" {
		text += "\n\nNeed help? Contact " + b.SupportEmail + "."
	}
	c := m.Content
	if c == nil {
		c = &Content{Heading: m.Subject, Paragraphs: strings.Split(m.Text, "\n\n")}
	}
	boundary := "govite-" + fmt.Sprint(time.Now().UnixNano())
	var sb strings.Builder
	h := func(k, v string) { sb.WriteString(k + ": " + sanitizeHeader(v) + "\r\n") }
	h("From", fmt.Sprintf("%s <%s>", b.AppName, s.from))
	h("To", m.To)
	if b.SupportEmail != "" {
		h("Reply-To", b.SupportEmail)
	}
	h("Subject", "=?UTF-8?B?"+b64(m.Subject)+"?=")
	h("Date", time.Now().Format(time.RFC1123Z))
	h("MIME-Version", "1.0")
	h("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	sb.WriteString("\r\n")
	for _, part := range [][2]string{{"text/plain", text}, {"text/html", Render(*c, b)}} {
		sb.WriteString("--" + boundary + "\r\nContent-Type: " + part[0] + "; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n" + wrap76(b64(part[1])) + "\r\n")
	}
	sb.WriteString("--" + boundary + "--\r\n")

	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}
	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(s.addr, auth, s.from, []string{m.To}, []byte(sb.String())) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15 * time.Second):
		return fmt.Errorf("mail: smtp timeout")
	}
}

// Memory records messages instead of sending; used by tests.
type Memory struct {
	mu   sync.Mutex
	Sent []Message
}

func (m *Memory) Send(_ context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sent = append(m.Sent, msg)
	return nil
}

func (m *Memory) Last() (Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Sent) == 0 {
		return Message{}, false
	}
	return m.Sent[len(m.Sent)-1], true
}

func (m *Memory) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Sent)
}
