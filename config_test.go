package rbhu

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "" +
		"# a comment\n" +
		"client_id=abc123\n" +
		"client_secret=\"sec ret\"\n" +
		"\n" +
		"redirect_url=https://example.test/cb\n" +
		"eidas=AA:BB:CC \n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := ConfigFromDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientID != "abc123" {
		t.Errorf("ClientID = %q", cfg.ClientID)
	}
	if cfg.ClientSecret != "sec ret" {
		t.Errorf("ClientSecret = %q (quotes should be stripped)", cfg.ClientSecret)
	}
	if cfg.RedirectURL != "https://example.test/cb" {
		t.Errorf("RedirectURL = %q", cfg.RedirectURL)
	}
	if cfg.EIDAS != "AA:BB:CC" {
		t.Errorf("EIDAS = %q (trailing space should be trimmed)", cfg.EIDAS)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("client_id", "id1")
	t.Setenv("client_secret", "sec1")
	t.Setenv("redirect_url", "https://r.test")
	t.Setenv("eidas", " FP ")

	cfg := ConfigFromEnv()
	if cfg.ClientID != "id1" || cfg.ClientSecret != "sec1" || cfg.RedirectURL != "https://r.test" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.EIDAS != "FP" {
		t.Errorf("EIDAS = %q, want trimmed", cfg.EIDAS)
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (Config{ClientID: "x", ClientSecret: "y"}).Validate(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
	if err := (Config{ClientID: "x"}).Validate(); err == nil {
		t.Error("expected error for missing ClientSecret")
	}
}

func TestLoadCertificate(t *testing.T) {
	// Build a self-signed cert + key, encode to PKCS#12, then load it back.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "rbhu-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	const pw = "changeit"
	p12, err := pkcs12.Modern.Encode(key, cert, nil, pw)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.p12")
	if err := os.WriteFile(path, p12, 0o600); err != nil {
		t.Fatal(err)
	}

	tlsCert, err := LoadCertificate(path, pw)
	if err != nil {
		t.Fatalf("LoadCertificate: %v", err)
	}
	if tlsCert.Leaf == nil || tlsCert.Leaf.Subject.CommonName != "rbhu-test" {
		t.Errorf("unexpected leaf: %+v", tlsCert.Leaf)
	}
	if tlsCert.PrivateKey == nil {
		t.Error("private key not loaded")
	}

	if _, err := LoadCertificate(path, "wrong"); err == nil {
		t.Error("expected error with wrong password")
	}
}
