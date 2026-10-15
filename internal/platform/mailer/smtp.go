package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// sendTimeout bounds the whole dial+handshake+send sequence. Without it, a
// network that silently drops outbound SMTP (common for home routers, ISPs,
// or corporate firewalls blocking port 587/25) leaves the call hanging on
// the OS's TCP connect timeout — which can be a minute or more — instead of
// failing fast and logging a clear reason.
const sendTimeout = 10 * time.Second

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type SMTPMailer struct {
	cfg SMTPConfig
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

// Send connects fresh for every call (STARTTLS on cfg.Port, typically 587)
// rather than pooling a connection — registration volume here is low enough
// that connection reuse isn't worth the added complexity.
func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	addr := m.cfg.Host + ":" + m.cfg.Port

	conn, err := net.DialTimeout("tcp", addr, sendTimeout)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	deadline := time.Now().Add(sendTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}
	if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}
	if err := c.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := c.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.cfg.From, to, subject, body,
	)
	if _, err := wc.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return c.Quit()
}
