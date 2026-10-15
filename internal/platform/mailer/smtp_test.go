package mailer_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"salt/hackethon/internal/platform/mailer"
)

func TestSMTPMailer_Send_Success(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{})
	host, port := hostPort(addr)

	m := mailer.NewSMTPMailer(mailer.SMTPConfig{
		Host: host, Port: port, Username: "user", Password: "pass", From: "from@example.com",
		RootCAs: pool,
	})

	err := m.Send(context.Background(), "to@example.com", "Subject", "Body")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestSMTPMailer_Send_DialError(t *testing.T) {
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: "127.0.0.1", Port: "1"}) // port 1 refuses connections

	err := m.Send(context.Background(), "to@example.com", "Subject", "Body")
	if err == nil || !strings.Contains(err.Error(), "smtp dial:") {
		t.Fatalf("err = %v, want it wrapped with %q", err, "smtp dial:")
	}
}

func TestSMTPMailer_Send_NewClientError(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{dropImmediately: true})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	err := m.Send(context.Background(), "to@example.com", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "smtp client:") {
		t.Fatalf("err = %v, want it wrapped with %q", err, "smtp client:")
	}
}

func TestSMTPMailer_Send_HelloError(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{rejectHello: true})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	err := m.Send(context.Background(), "to@example.com", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "smtp hello:") {
		t.Fatalf("err = %v, want it wrapped with %q", err, "smtp hello:")
	}
}

func TestSMTPMailer_Send_StartTLSError(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{rejectStartTLS: true})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	err := m.Send(context.Background(), "to@example.com", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "smtp starttls:") {
		t.Fatalf("err = %v, want it wrapped with %q", err, "smtp starttls:")
	}
}

func TestSMTPMailer_Send_AuthError(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{rejectAuth: true})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	err := m.Send(context.Background(), "to@example.com", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "smtp auth:") {
		t.Fatalf("err = %v, want it wrapped with %q", err, "smtp auth:")
	}
}

func TestSMTPMailer_Send_MailFromError(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{rejectMailFrom: true})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	err := m.Send(context.Background(), "to@example.com", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "smtp mail from:") {
		t.Fatalf("err = %v, want it wrapped with %q", err, "smtp mail from:")
	}
}

func TestSMTPMailer_Send_RcptToError(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{rejectRcptTo: true})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	err := m.Send(context.Background(), "to@example.com", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "smtp rcpt to:") {
		t.Fatalf("err = %v, want it wrapped with %q", err, "smtp rcpt to:")
	}
}

func TestSMTPMailer_Send_DataError(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{rejectData: true})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	err := m.Send(context.Background(), "to@example.com", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "smtp data:") {
		t.Fatalf("err = %v, want it wrapped with %q", err, "smtp data:")
	}
}

func TestSMTPMailer_Send_WriteError(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{dropAfterData: true})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	err := m.Send(context.Background(), "to@example.com", "s", "b")
	if err == nil {
		t.Fatal("expected a write/close error after the server dropped the connection, got nil")
	}
	if !strings.Contains(err.Error(), "smtp write:") && !strings.Contains(err.Error(), "smtp close:") {
		t.Fatalf("err = %v, want it wrapped with %q or %q", err, "smtp write:", "smtp close:")
	}
}

// TestSMTPMailer_Send_RespectsContextDeadline covers the branch where the
// caller's context deadline is tighter than the mailer's own sendTimeout.
func TestSMTPMailer_Send_RespectsContextDeadline(t *testing.T) {
	addr, pool := startFakeSMTPServer(t, fakeSMTPServerScript{})
	host, port := hostPort(addr)
	m := mailer.NewSMTPMailer(mailer.SMTPConfig{Host: host, Port: port, RootCAs: pool})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.Send(ctx, "to@example.com", "s", "b"); err != nil {
		t.Fatalf("Send: %v", err)
	}
}
