package mail

import (
	"encoding/base64"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

// ponytail: colours mirror fe light theme; email clients can't read CSS vars.
// Move to app settings if per-tenant colours are needed.
const (
	cBg      = "#f3f6f7"
	cCard    = "#ffffff"
	cText    = "#0f2328"
	cMuted   = "#5d7178"
	cBorder  = "#dfe7e9"
	cPrimary = "#0b7a8c"
	cTint    = "#eaf5f7"
	font     = "-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif"
	mono     = "'SFMono-Regular',Menlo,Consolas,'Courier New',monospace"
)

var httpURL = regexp.MustCompile(`(?i)^https?://`)

func esc(s string) string { return html.EscapeString(s) }

// safeURL: only http(s) links reach an href/src.
func safeURL(u string) string {
	if httpURL.MatchString(u) {
		return esc(u)
	}
	return ""
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func wrap76(s string) string {
	var b strings.Builder
	for len(s) > 76 {
		b.WriteString(s[:76] + "\r\n")
		s = s[76:]
	}
	return b.String() + s
}

func sanitizeHeader(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// Render builds the branded HTML email.
func Render(c Content, b Brand) string {
	var sb strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&sb, f, a...) }

	logo := safeURL(b.LogoURL)
	btn := safeURL(c.ButtonURL)

	w(`<!doctype html><html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/><meta name="color-scheme" content="light"/><title>%s</title></head>`, esc(c.Heading))
	w(`<body style="margin:0;padding:0;background:%s;font-family:%s;">`, cBg, font)
	if c.Preheader != "" {
		w(`<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">%s</div>`, esc(c.Preheader))
	}
	w(`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:%s;"><tr><td align="center" style="padding:32px 16px;">`, cBg)
	w(`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background:%s;border:1px solid %s;border-radius:12px;overflow:hidden;">`, cCard, cBorder)
	w(`<tr><td style="height:4px;background:%s;font-size:0;line-height:0;">&nbsp;</td></tr>`, cPrimary)
	w(`<tr><td style="padding:24px 32px 0;"><table role="presentation" cellpadding="0" cellspacing="0"><tr>`)
	if logo != "" {
		w(`<td style="padding-right:10px;"><img src="%s" alt="" height="32" style="display:block;height:32px;width:auto;border:0;"/></td>`, logo)
	}
	w(`<td style="font-size:18px;font-weight:700;color:%s;">%s</td></tr></table></td></tr>`, cText, esc(b.AppName))
	w(`<tr><td style="padding:24px 32px 32px;"><h1 style="margin:0 0 16px;font-size:22px;line-height:30px;font-weight:700;color:%s;">%s</h1>`, cText, esc(c.Heading))
	for _, p := range c.Paragraphs {
		w(`<p style="margin:0 0 16px;font-size:15px;line-height:24px;color:%s;">%s</p>`, cText, esc(p))
	}
	if btn != "" {
		w(`<table role="presentation" cellpadding="0" cellspacing="0" style="margin:8px 0 20px;"><tr><td style="border-radius:8px;background:%s;"><a href="%s" style="display:inline-block;padding:12px 24px;font-size:15px;font-weight:600;color:#ffffff;text-decoration:none;border-radius:8px;">%s</a></td></tr></table>`, cPrimary, btn, esc(c.ButtonText))
		w(`<p style="margin:0 0 16px;font-size:12px;line-height:18px;color:%s;">Button not working? Copy this link into your browser:<br/><a href="%s" style="color:%s;word-break:break-all;">%s</a></p>`, cMuted, btn, cPrimary, btn)
	}
	if c.Code != "" {
		w(`<div style="margin:8px 0 20px;padding:16px;text-align:center;background:%s;border:1px solid %s;border-radius:8px;font-family:%s;font-size:32px;font-weight:700;letter-spacing:8px;color:%s;">%s</div>`, cTint, cBorder, mono, cText, esc(c.Code))
	}
	if c.Note != "" {
		w(`<p style="margin:0;font-size:13px;line-height:20px;color:%s;">%s</p>`, cMuted, esc(c.Note))
	}
	w(`</td></tr><tr><td style="padding:20px 32px;background:%s;border-top:1px solid %s;font-size:12px;line-height:18px;color:%s;">`, cBg, cBorder, cMuted)
	if b.Tagline != "" {
		w(`<p style="margin:0 0 8px;font-weight:600;color:%s;">%s</p>`, cText, esc(b.Tagline))
	}
	if b.EmailFooter != "" {
		w(`<p style="margin:0 0 8px;">%s</p>`, esc(b.EmailFooter))
	}
	var contact []string
	if b.SupportEmail != "" {
		contact = append(contact, fmt.Sprintf(`<a href="mailto:%s" style="color:%s;text-decoration:none;">%s</a>`, esc(b.SupportEmail), cPrimary, esc(b.SupportEmail)))
	}
	if b.SupportPhone != "" {
		contact = append(contact, esc(b.SupportPhone))
	}
	if len(contact) > 0 {
		w(`<p style="margin:0 0 8px;">%s</p>`, strings.Join(contact, " &nbsp;·&nbsp; "))
	}
	if b.Address != "" {
		w(`<p style="margin:0 0 8px;">%s</p>`, esc(b.Address))
	}
	w(`<p style="margin:0;">&copy; %d %s</p></td></tr></table></td></tr></table></body></html>`, time.Now().Year(), esc(b.AppName))
	return sb.String()
}
