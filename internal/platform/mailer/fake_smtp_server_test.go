package mailer_test

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeSMTPServerScript controls how the fake server responds at each stage,
// so each test case can drive Send() down a specific branch.
type fakeSMTPServerScript struct {
	rejectHello     bool
	rejectStartTLS  bool
	rejectAuth      bool
	rejectMailFrom  bool
	rejectRcptTo    bool
	rejectData      bool
	dropAfterData   bool // accept DATA then close the connection, to force a write/close error on the client
	dropImmediately bool // close before sending the 220 greeting, to force smtp.NewClient to error
}

// startFakeSMTPServer speaks just enough SMTP (plus STARTTLS) to drive
// mailer.SMTPMailer.Send through every branch without a real mail server. It
// returns the listener address and the server's certificate pool so the test
// can configure SMTPConfig.RootCAs to trust it.
func startFakeSMTPServer(t *testing.T, script fakeSMTPServerScript) (addr string, pool *x509.CertPool) {
	t.Helper()

	cert, pool := generateSelfSignedCert(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		serveFakeSMTP(conn, cert, script)
	}()

	return ln.Addr().String(), pool
}

func serveFakeSMTP(conn net.Conn, cert tls.Certificate, script fakeSMTPServerScript) {
	defer func() { _ = conn.Close() }()

	if script.dropImmediately {
		return
	}

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	write := func(line string) {
		_, _ = rw.WriteString(line + "\r\n")
		_ = rw.Flush()
	}
	readLine := func() (string, error) {
		line, err := rw.ReadString('\n')
		return strings.TrimRight(line, "\r\n"), err
	}

	write("220 fake.smtp ESMTP")

	line, err := readLine()
	if err != nil || !strings.HasPrefix(strings.ToUpper(line), "EHLO") {
		return
	}
	if script.rejectHello {
		write("554 no thanks")
		return
	}
	write("250-fake.smtp")
	write("250 STARTTLS")

	line, err = readLine()
	if err != nil || strings.ToUpper(line) != "STARTTLS" {
		return
	}
	if script.rejectStartTLS {
		write("454 TLS not available")
		return
	}
	write("220 Go ahead")

	tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	defer func() { _ = tlsConn.Close() }()
	conn = tlsConn
	rw = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	write = func(line string) {
		_, _ = rw.WriteString(line + "\r\n")
		_ = rw.Flush()
	}

	line, err = readLine()
	if err != nil || !strings.HasPrefix(strings.ToUpper(line), "EHLO") {
		return
	}
	write("250-fake.smtp")
	write("250 AUTH PLAIN")

	line, err = readLine()
	if err != nil || !strings.HasPrefix(strings.ToUpper(line), "AUTH") {
		return
	}
	if script.rejectAuth {
		write("535 authentication failed")
		return
	}
	write("235 Authentication successful")

	line, err = readLine()
	if err != nil || !strings.HasPrefix(strings.ToUpper(line), "MAIL FROM") {
		return
	}
	if script.rejectMailFrom {
		write("451 mail from rejected")
		return
	}
	write("250 OK")

	line, err = readLine()
	if err != nil || !strings.HasPrefix(strings.ToUpper(line), "RCPT TO") {
		return
	}
	if script.rejectRcptTo {
		write("550 no such user")
		return
	}
	write("250 OK")

	line, err = readLine()
	if err != nil || strings.ToUpper(line) != "DATA" {
		return
	}
	if script.rejectData {
		write("554 no data for you")
		return
	}
	write("354 Start mail input")

	if script.dropAfterData {
		return
	}

	for {
		line, err = readLine()
		if err != nil || line == "." {
			break
		}
	}
	write("250 OK queued")

	line, err = readLine()
	if err != nil || strings.ToUpper(line) != "QUIT" {
		return
	}
	write("221 Bye")
}

func generateSelfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	cert := tls.Certificate{Certificate: [][]byte{derBytes}, PrivateKey: key}

	leaf, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	return cert, pool
}

func hostPort(addr string) (host, port string) {
	parts := strings.Split(addr, ":")
	return parts[0], parts[len(parts)-1]
}
