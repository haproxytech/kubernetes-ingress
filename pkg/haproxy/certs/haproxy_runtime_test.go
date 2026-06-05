package certs

// Integration tests that spin up a real HAProxy process to prove that a blank
// line between PEM blocks in a cert chain causes the intermediate certificate
// to be silently dropped by the "set ssl cert" runtime command.
//
// cert-manager routinely produces TLS secrets where the cert chain contains a
// blank line between the leaf and the intermediate CA.  When the controller
// passes this payload to HAProxy's runtime socket without normalizing it first,
// HAProxy commits the transaction successfully but stores only the leaf — the
// intermediate CA is gone.  TLS clients that require chain verification then
// fail.
//
// Set HAPROXY_BIN to test against a specific binary:
//
//	HAPROXY_BIN=/path/to/haproxy go test ./pkg/haproxy/certs/ -run TestHAProxy_ -v

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var haproxyVersionRE = regexp.MustCompile(`HAProxy version (\d+)\.(\d+)`)

func haproxyBin() string {
	if b := os.Getenv("HAPROXY_BIN"); b != "" {
		return b
	}
	return "haproxy"
}

type rtHarness struct {
	adminSock    string
	cmd          *exec.Cmd
	logBuf       *bytes.Buffer
	dir          string
	majorVersion int
	minorVersion int
}

func startHAProxyForCerts(t *testing.T) *rtHarness {
	t.Helper()

	bin := haproxyBin()

	versionOut, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		t.Skipf("%s not available: %v", bin, err)
	}
	major, minor := 0, 0
	if m := haproxyVersionRE.FindSubmatch(versionOut); m != nil {
		major, _ = strconv.Atoi(string(m[1]))
		minor, _ = strconv.Atoi(string(m[2]))
	}
	t.Logf("haproxy %d.%d (%s)", major, minor, bin)

	// Runtime cert commands (new/set/commit ssl cert) require HAProxy >= 3.2.
	if major < 3 || (major == 3 && minor < 2) {
		t.Skipf("haproxy %d.%d does not support runtime cert commands (need >= 3.2)", major, minor)
	}

	dir := t.TempDir()
	adminSock := filepath.Join(dir, "admin.sock")
	initialCertPath := filepath.Join(dir, "initial.pem")

	if err := os.WriteFile(initialCertPath, generateInitialCert(t), 0o600); err != nil {
		t.Fatal(err)
	}

	port := findFreePort(t)
	cfg := fmt.Sprintf(`
global
    stats socket %s mode 660 level admin

defaults
    timeout connect 5s
    timeout client 10s
    timeout server 10s

frontend fe_ssl
    bind 127.0.0.1:%d ssl crt %s
    mode tcp
    default_backend be_dummy

backend be_dummy
    server dummy 127.0.0.1:1
`, adminSock, port, initialCertPath)

	cfgPath := filepath.Join(dir, "haproxy.cfg")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "-c", "-f", cfgPath).CombinedOutput(); err != nil {
		t.Skipf("haproxy config check failed: %v\n%s", err, out)
	}

	logBuf := &bytes.Buffer{}
	cmd := exec.Command(bin, "-W", "-db", "-f", cfgPath)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start haproxy: %v", err)
	}

	h := &rtHarness{
		adminSock: adminSock, cmd: cmd, logBuf: logBuf, dir: dir,
		majorVersion: major, minorVersion: minor,
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
		if t.Failed() {
			t.Logf("haproxy log:\n%s", logBuf)
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(adminSock); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(adminSock); err != nil {
		t.Fatalf("haproxy did not create admin socket within 5s\nlog:\n%s", logBuf)
	}
	return h
}

// socketCmd sends cmd to the HAProxy admin socket and returns the full response.
func (h *rtHarness) socketCmd(t *testing.T, cmd string) string {
	t.Helper()
	c, err := net.Dial("unix", h.adminSock)
	if err != nil {
		t.Fatalf("dial admin socket: %v", err)
	}
	defer c.Close()
	if !strings.HasSuffix(cmd, "\n") {
		cmd += "\n"
	}
	if _, err := c.Write([]byte(cmd)); err != nil {
		t.Fatalf("write to admin socket: %v", err)
	}
	if uc, ok := c.(*net.UnixConn); ok {
		_ = uc.CloseWrite()
	}
	b, err := io.ReadAll(c)
	if err != nil {
		t.Fatalf("read from admin socket: %v", err)
	}
	return strings.TrimSpace(string(b))
}

// commitCert runs the full new→set→commit transaction for the given PEM payload
// and returns the output of "show ssl cert", which includes "Chain Subject:" and
// "Chain Issuer:" only when the intermediate certificate was successfully stored.
func (h *rtHarness) commitCert(t *testing.T, payload []byte) string {
	t.Helper()
	certPath := filepath.Join(h.dir, fmt.Sprintf("txn-%d.pem", time.Now().UnixNano()))

	t.Logf("new ssl cert: %s", h.socketCmd(t, "new ssl cert "+certPath))
	setResp := h.socketCmd(t, fmt.Sprintf("set ssl cert %s <<\n%s\n", certPath, payload))
	t.Logf("set ssl cert:\n%s", setResp)
	t.Logf("commit ssl cert: %s", h.socketCmd(t, "commit ssl cert "+certPath))

	return h.socketCmd(t, "show ssl cert "+certPath)
}

func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func generateInitialCert(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-initial"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	var buf bytes.Buffer
	_ = pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return buf.Bytes()
}

// buildChainWithGap returns a PEM bundle of key + leaf + intermediate where a
// blank line (\n\n) separates the two certificate blocks — the format
// cert-manager produces.  pem.Encode always appends \n, so one extra \n
// creates the \n\n gap.
//
// NOTE: this is the only separator variant that causes HAProxy to silently drop
// the intermediate cert.  CRLF (\r\n\r\n) and space-only (\n \n) separators
// are accepted by HAProxy without losing the chain, so they cannot be proven
// here — their normalization is tested at the unit level in main_test.go.
func buildChainWithGap(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	makeCert := func(serial int64, cn string) []byte {
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: cn},
			NotBefore:    time.Now().Add(-time.Minute),
			NotAfter:     time.Now().Add(time.Hour),
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		if err != nil {
			t.Fatalf("create certificate: %v", err)
		}
		var b bytes.Buffer
		_ = pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: der})
		return b.Bytes()
	}
	var buf bytes.Buffer
	_ = pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	_, _ = buf.Write(makeCert(1, "test-leaf")) // ends with \n from pem.Encode
	_ = buf.WriteByte('\n')                    // blank line cert-manager injects
	_, _ = buf.Write(makeCert(2, "test-intermediate"))
	return buf.Bytes()
}

// TestHAProxy_BlankLineDropsIntermediateCert proves that a blank line between
// PEM blocks causes HAProxy to silently commit the transaction with only the
// leaf certificate — the intermediate CA is lost.
//
// HAProxy's "set ssl cert" runtime command accepts the payload without error
// but the blank line terminates the PEM section it processes, so the
// intermediate cert that follows is never stored.  "show ssl cert" on the
// committed entry then lacks the "Chain Subject:" and "Chain Issuer:" fields
// that would be present if the full chain had been received.
func TestHAProxy_BlankLineDropsIntermediateCert(t *testing.T) {
	h := startHAProxyForCerts(t)

	chain := buildChainWithGap(t)
	if !bytes.Contains(chain, []byte("\n\n")) {
		t.Fatal("test setup error: chain must contain a blank line")
	}

	show := h.commitCert(t, chain)
	t.Logf("show ssl cert:\n%s", show)

	if strings.Contains(show, "Chain Subject:") {
		t.Fatal("intermediate cert should have been dropped (blank line in payload), " +
			"but 'Chain Subject:' is present — HAProxy may have fixed this behaviour")
	}
}

// TestHAProxy_NormalizePEMPreservesIntermediateCert proves that normalizePEM
// removes the blank line, producing a payload where HAProxy stores the full
// chain.  "show ssl cert" on the committed entry contains "Chain Subject:",
// confirming the intermediate CA was received.
//
// CRLF (\r\n\r\n) and space-only (\n \n) separators are NOT tested here
// because HAProxy accepts those inputs without losing the chain regardless of
// normalization — they cannot be distinguished at the runtime-command level.
// Their normalization is proven by the unit tests in main_test.go.
func TestHAProxy_NormalizePEMPreservesIntermediateCert(t *testing.T) {
	h := startHAProxyForCerts(t)

	chain := buildChainWithGap(t)
	normalized := normalizePEM(chain)

	show := h.commitCert(t, normalized)
	t.Logf("show ssl cert:\n%s", show)

	if !strings.Contains(show, "Chain Subject:") {
		t.Fatal("expected 'Chain Subject:' in show ssl cert output — " +
			"normalizePEM output did not preserve the intermediate certificate")
	}
}
