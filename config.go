package rbhu

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// Config holds the credentials and certificate needed to talk to RBHU.
type Config struct {
	// ClientID is the marketplace application client id.
	ClientID string
	// ClientSecret is the marketplace application secret.
	ClientSecret string
	// RedirectURL is the OAuth redirect URI registered for the application.
	RedirectURL string
	// EIDAS is the SHA-256 fingerprint of the client/eIDAS certificate,
	// sent as the TPP-Signature-Certificate identifier where required.
	EIDAS string
	// Certificate is the client (mutual-TLS) certificate used for the
	// MTLS_EP data-API endpoints. Optional for plain-TLS calls.
	Certificate *tls.Certificate
}

// Validate reports whether the minimum OAuth fields are present.
func (c Config) Validate() error {
	var missing []string
	if c.ClientID == "" {
		missing = append(missing, "ClientID")
	}
	if c.ClientSecret == "" {
		missing = append(missing, "ClientSecret")
	}
	if len(missing) > 0 {
		return fmt.Errorf("rbhu: incomplete config, missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ConfigFromEnv builds a Config from environment variables:
// client_id, client_secret, redirect_url, eidas.
func ConfigFromEnv() Config {
	return Config{
		ClientID:     os.Getenv("client_id"),
		ClientSecret: os.Getenv("client_secret"),
		RedirectURL:  os.Getenv("redirect_url"),
		EIDAS:        strings.TrimSpace(os.Getenv("eidas")),
	}
}

// LoadDotEnv parses a simple KEY=VALUE .env file. Lines that are blank or
// start with '#' are ignored. Surrounding quotes and whitespace are trimmed.
// It does not mutate the process environment.
func LoadDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		out[k] = v
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ConfigFromDotEnv loads a Config from a .env file (see LoadDotEnv).
func ConfigFromDotEnv(path string) (Config, error) {
	m, err := LoadDotEnv(path)
	if err != nil {
		return Config{}, err
	}
	return Config{
		ClientID:     m["client_id"],
		ClientSecret: m["client_secret"],
		RedirectURL:  m["redirect_url"],
		EIDAS:        strings.TrimSpace(m["eidas"]),
	}, nil
}

// LoadCertificate decodes a PKCS#12 (.p12/.pfx) bundle into a TLS client
// certificate for use with the mutual-TLS data endpoints.
func LoadCertificate(path, password string) (*tls.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rbhu: read p12: %w", err)
	}
	key, cert, caCerts, err := pkcs12.DecodeChain(raw, password)
	if err != nil {
		return nil, fmt.Errorf("rbhu: decode p12: %w", err)
	}
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}
	for _, ca := range caCerts {
		tlsCert.Certificate = append(tlsCert.Certificate, ca.Raw)
	}
	return tlsCert, nil
}
