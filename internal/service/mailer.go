package service

import (
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/LinuxSuRen/tick-flow/internal/config"
)

// Mailer sends plain-text mail over SMTP. When the host is empty every
// send is a logged no-op so local deployments work without SMTP (issue #6:
// invitation links then go to the server log).
type Mailer struct {
	cfg config.SMTPConfig
}

// NewMailer builds the mailer from configuration.
func NewMailer(cfg config.SMTPConfig) *Mailer {
	return &Mailer{cfg: cfg}
}

// Enabled reports whether SMTP is configured.
func (m *Mailer) Enabled() bool {
	return m.cfg.Host != ""
}

// Send delivers a plain-text message. Errors are returned but callers
// (notification subscriber) only log them: mail must never break the main
// flow (issue #16).
func (m *Mailer) Send(to, subject, body string) error {
	if !m.Enabled() {
		slog.Info("smtp not configured, skipping mail", "to", to, "subject", subject)
		return nil
	}
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	from := m.cfg.From
	if from == "" {
		from = m.cfg.Username
	}
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}
