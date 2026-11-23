package mailer

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jobhunter/internal/config"
	"jobhunter/internal/model"
)

// Message is one outbound email.
type Message struct {
	To         string
	Subject    string
	Text       string
	HTML       string
	AttachName string
	Attach     []byte
}

// Sender delivers mail over SMTP, or writes .eml files when DryRun is set.
type Sender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	DryRun   bool
	Outbox   string
}

// Send delivers one message.
func (s Sender) Send(msg Message) error {
	raw := buildMIME(s.FromName, s.From, msg)
	if s.DryRun {
		return s.writeOutbox(msg.Subject, raw)
	}
	return s.sendSMTP(msg.To, raw)
}

func (s Sender) sendSMTP(to string, raw []byte) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
	if s.Port == 465 {
		return sendImplicitTLS(s.Host, addr, auth, s.From, to, raw)
	}
	return smtp.SendMail(addr, auth, s.From, []string{to}, raw)
}

func sendImplicitTLS(host, addr string, auth smtp.Auth, from, to string, raw []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (s Sender) writeOutbox(subject string, raw []byte) error {
	if err := os.MkdirAll(s.Outbox, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("%d-%s.eml", time.Now().UnixNano(), slug(subject))
	path := filepath.Join(s.Outbox, name)
	return os.WriteFile(path, raw, 0o644)
}

// Application is the hiring-contact email, with the resume attached by the caller.
func Application(user config.User, job model.Job, skills []string) Message {
	skillText := humanList(limit(skills, 6))
	subject := sanitize(fmt.Sprintf("Application for %s at %s - Deepak Kumar Mandal", clip(job.Title, 80), clip(job.Company, 40)))
	text := fmt.Sprintf(`Dear Hiring Team,

I am applying for the %s role at %s. I am a Software Development Engineer based in Gurugram, building backend systems for 5G core networks.

I work in Go (Fiber) and C++, with Docker, Kubernetes, PostgreSQL, and socket programming (TCP, UDP, SCTP). At Amantya I built an NF Simulator that emulates AMF, SMF, AUSF, UDR, and UDM, and I develop multi-threaded REST services and containerized provisioning tools. I studied Computer Science at Netaji Subhas University of Technology.

This role matches my background in %s.

My resume is attached. I would be glad to talk.

Deepak Kumar Mandal
%s
%s
LinkedIn: %s
GitHub: %s
LeetCode: %s

Role: %s
`, job.Title, job.Company, skillText, user.Phone, user.Email, user.LinkedIn, user.GitHub, user.LeetCode, job.URL)

	htmlBody := `<div style="font-family:Arial,sans-serif;line-height:1.5;color:#1a1a1a">` + textToHTML(text) + `</div>`
	return Message{To: "", Subject: subject, Text: text, HTML: htmlBody}
}

// Item is one role inside the digest sent to the candidate.
type Item struct {
	Title       string
	Company     string
	Location    string
	URL         string
	Source      string
	Salary      string
	Email       string
	Posted      string
	Note        string
	Score       int
	Skills      string
	Description string
}

// Digest is the summary mailed to the candidate.
type Digest struct {
	Name  string
	Sent  []Item
	Apply []Item
}

// DigestMessage builds the candidate's summary email.
func DigestMessage(to string, digest Digest) Message {
	subject := digestSubject(len(digest.Sent), len(digest.Apply))
	var text strings.Builder
	fmt.Fprintf(&text, "Hi %s,\n\nThese new roles match your Go, C++, Python, SQL, Docker, and Kubernetes background.\n", digest.Name)
	if len(digest.Sent) > 0 {
		text.WriteString("\nApplications emailed to the hiring contact\n")
		for _, item := range digest.Sent {
			writeItem(&text, item)
		}
	}
	if len(digest.Apply) > 0 {
		text.WriteString("\nNo hiring email on the posting. Apply with these links\n")
		for _, item := range digest.Apply {
			writeItem(&text, item)
		}
	}
	text.WriteString("\nEach role credits its source. Remotive, Jobicy, and Remote OK links point back at those sites.\n")

	var htmlBuf bytes.Buffer
	if err := digestTmpl.Execute(&htmlBuf, digest); err != nil {
		htmlBuf.WriteString("<pre>")
		htmlBuf.WriteString(template.HTMLEscapeString(text.String()))
		htmlBuf.WriteString("</pre>")
	}
	return Message{To: to, Subject: subject, Text: text.String(), HTML: htmlBuf.String()}
}

func writeItem(b *strings.Builder, item Item) {
	fmt.Fprintf(b, "\n- %s at %s\n  %s\n  Skills: %s (score %d)\n  Source: %s", item.Title, item.Company, item.URL, item.Skills, item.Score, item.Source)
	if item.Location != "" {
		fmt.Fprintf(b, " | %s", item.Location)
	}
	if item.Posted != "" {
		fmt.Fprintf(b, " | posted %s", item.Posted)
	}
	if item.Salary != "" {
		fmt.Fprintf(b, " | %s", item.Salary)
	}
	if item.Email != "" {
		fmt.Fprintf(b, "\n  Hiring email: %s", item.Email)
	}
	if item.Note != "" {
		fmt.Fprintf(b, "\n  %s", item.Note)
	}
	if item.Description != "" {
		fmt.Fprintf(b, "\n  %s", item.Description)
	}
	b.WriteString("\n")
}

func digestSubject(sent, apply int) string {
	switch {
	case sent > 0 && apply > 0:
		return fmt.Sprintf("Job hunter: %d applications sent, %d roles to apply", sent, apply)
	case sent > 0:
		return fmt.Sprintf("Job hunter: %d applications sent to hiring contacts", sent)
	default:
		return fmt.Sprintf("Job hunter: %d new roles to apply", apply)
	}
}

func buildMIME(fromName, from string, msg Message) []byte {
	mixed := fmt.Sprintf("mix_%d", time.Now().UnixNano())
	alt := fmt.Sprintf("alt_%d", time.Now().UnixNano())
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", formatAddr(fromName, from))
	fmt.Fprintf(&b, "To: %s\r\n", sanitize(msg.To))
	fmt.Fprintf(&b, "Reply-To: %s\r\n", sanitize(from))
	fmt.Fprintf(&b, "Subject: %s\r\n", sanitize(msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%d.%s@jobhunter>\r\n", time.Now().UnixNano(), slug(msg.To))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	if len(msg.Attach) > 0 {
		fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mixed)
		fmt.Fprintf(&b, "--%s\r\n", mixed)
	}
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", alt)
	fmt.Fprintf(&b, "--%s\r\n%s", alt, encodedPart("text/plain; charset=UTF-8", msg.Text))
	fmt.Fprintf(&b, "--%s\r\n%s", alt, encodedPart("text/html; charset=UTF-8", msg.HTML))
	fmt.Fprintf(&b, "--%s--\r\n", alt)
	if len(msg.Attach) > 0 {
		name := msg.AttachName
		if name == "" {
			name = "resume.pdf"
		}
		fmt.Fprintf(&b, "--%s\r\n", mixed)
		fmt.Fprintf(&b, "Content-Type: application/pdf; name=%q\r\n", name)
		fmt.Fprintf(&b, "Content-Disposition: attachment; filename=%q\r\n", name)
		fmt.Fprintf(&b, "Content-Transfer-Encoding: base64\r\n\r\n%s\r\n", wrapBase64(msg.Attach))
		fmt.Fprintf(&b, "--%s--\r\n", mixed)
	}
	return []byte(b.String())
}

func encodedPart(contentType, body string) string {
	return fmt.Sprintf("Content-Type: %s\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n", contentType, wrapBase64([]byte(body)))
}

func wrapBase64(raw []byte) string {
	encoded := base64.StdEncoding.EncodeToString(raw)
	var b strings.Builder
	for len(encoded) > 76 {
		b.WriteString(encoded[:76])
		b.WriteString("\r\n")
		encoded = encoded[76:]
	}
	b.WriteString(encoded)
	return b.String()
}

func formatAddr(name, email string) string {
	email = sanitize(email)
	name = sanitize(name)
	if name == "" {
		return email
	}
	return fmt.Sprintf("%q <%s>", name, email)
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}

func textToHTML(s string) string {
	return strings.ReplaceAll(template.HTMLEscapeString(s), "\n", "<br>\n")
}

func humanList(items []string) string {
	switch len(items) {
	case 0:
		return "backend engineering"
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func limit(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-3]) + "..."
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			if b.Len() == 0 || b.String()[b.Len()-1] != '-' {
				b.WriteByte('-')
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "mail"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

var digestTmpl = template.Must(template.New("digest").Parse(`<!DOCTYPE html>
<html><body style="font-family:Arial,sans-serif;color:#1a1a1a;line-height:1.45;max-width:680px">
<p>Hi {{.Name}}, these new roles match your Go, C++, Python, SQL, Docker, and Kubernetes background.</p>
{{if .Sent}}
<h2 style="font-size:16px;margin:24px 0 8px">Applications emailed to the hiring contact</h2>
{{range .Sent}}{{template "card" .}}{{end}}
{{end}}
{{if .Apply}}
<h2 style="font-size:16px;margin:24px 0 8px">No hiring email on the posting. Apply with these links</h2>
{{range .Apply}}{{template "card" .}}{{end}}
{{end}}
<p style="color:#555;font-size:12px">Each role credits its source. Remotive, Jobicy, and Remote OK links go back to those sites. The next check runs in about 2 hours.</p>
</body></html>
{{define "card"}}
<div style="border:1px solid #e4e4e4;border-radius:8px;padding:12px 14px;margin:0 0 12px">
  <div style="font-weight:bold">{{.Title}}</div>
  <div>{{.Company}}{{if .Location}} · {{.Location}}{{end}}</div>
  <div style="color:#444;font-size:14px">{{.Skills}} · score {{.Score}} · {{.Source}}{{if .Posted}} · {{.Posted}}{{end}}{{if .Salary}} · {{.Salary}}{{end}}</div>
  {{if .Email}}<div style="font-size:14px">Hiring email: {{.Email}}</div>{{end}}
  {{if .Note}}<div style="font-size:14px">{{.Note}}</div>{{end}}
  <div style="margin-top:6px"><a href="{{.URL}}">{{.URL}}</a></div>
  {{if .Description}}<p style="font-size:14px;color:#333;margin:8px 0 0">{{.Description}}</p>{{end}}
</div>
{{end}}`))
