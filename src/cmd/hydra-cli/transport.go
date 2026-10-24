// =============================================================================
// HYDRA-UMC-TOOL-CLI - One HTTP client and the rules for credentials: cmd/hydra-cli/transport.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// Every request of this CLI goes through the client built here, so they all
// identify themselves the same way (User-Agent "hydra-cli/<version>" and
// X-Hydra-Client: cli), trust the same certificates and obey the same rule about
// credentials:
//
//   - A password, a login or a bearer token is never sent over plain http to a
//     host that is not this machine. HYDRA_CLI_ALLOW_INSECURE=1 (or "allowInsecure"
//     in a --config file) turns the refusal into a warning, for a network the
//     operator trusts; https needs nothing.
//   - A server with a certificate of its own CA is trusted with HYDRA_CLI_CA_FILE
//     (or "caFile" in a --config file): a PEM file added to the system roots.
//
// Both are read when the target is resolved (resolveTarget), so a bad CA file is a
// configuration error before anything is sent.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	caFileEnv        = "HYDRA_CLI_CA_FILE"
	allowInsecureEnv = "HYDRA_CLI_ALLOW_INSECURE"
)

// transportSettings are the two options above, as resolved for the command being run.
type transportSettings struct {
	CAFile        string
	AllowInsecure bool
}

// activeTransport is what the command being run was given. resolveTarget sets it at the start of every command (a
// `shell` runs one command after another in one process, each resolving its own).
var activeTransport transportSettings

var (
	caPoolMu    sync.Mutex
	caPoolCache = map[string]*x509.CertPool{}
)

// loadCAPool reads a PEM file of certificates and returns the system roots with those added.
func loadCAPool(path string) (*x509.CertPool, error) {
	caPoolMu.Lock()
	defer caPoolMu.Unlock()
	if pool, ok := caPoolCache[path]; ok {
		return pool, nil
	}
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s holds no PEM certificate", path)
	}
	caPoolCache[path] = pool
	return pool, nil
}

// settingsFromEnvironment reads the two options from the environment.
func settingsFromEnvironment() transportSettings {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(allowInsecureEnv)))
	return transportSettings{
		CAFile:        strings.TrimSpace(os.Getenv(caFileEnv)),
		AllowInsecure: value == "1" || value == "true" || value == "yes",
	}
}

// checkTransportSettings fails with a configuration error when the CA file cannot be used.
func checkTransportSettings(settings transportSettings) error {
	if settings.CAFile == "" {
		return nil
	}
	if _, err := loadCAPool(settings.CAFile); err != nil {
		return newCliError(ExitConfigError, fmt.Errorf("the CA file (%s or \"caFile\"): %w", caFileEnv, err))
	}
	return nil
}

// identifyingTransport marks every request as coming from this CLI, unless a caller already set the header.
type identifyingTransport struct {
	base http.RoundTripper
}

func (t identifyingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if clone.Header.Get("User-Agent") == "" {
		clone.Header.Set("User-Agent", "hydra-cli/"+Version)
	}
	if clone.Header.Get("X-Hydra-Client") == "" {
		clone.Header.Set("X-Hydra-Client", "cli")
	}
	return t.base.RoundTrip(clone)
}

// newClient is the one HTTP client of the CLI: the timeout of the command, the CA file when one is configured, and the
// identification headers. A response is read through readBody, which caps its size.
func newClient(timeout time.Duration) http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if activeTransport.CAFile != "" {
		if pool, err := loadCAPool(activeTransport.CAFile); err == nil {
			transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
		}
	}
	return http.Client{Timeout: timeout, Transport: identifyingTransport{base: transport}}
}

// isLocalHost reports whether a host name or address is this machine.
func isLocalHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// requireSecureCredentials is called before a password, a login or a token is sent. Over https, or to this machine, it
// passes. Over plain http to another host it refuses, unless the operator allowed it (then it only warns).
func requireSecureCredentials(server string) error {
	parsed, err := url.Parse(server)
	if err != nil || parsed.Scheme != "http" || isLocalHost(parsed.Hostname()) {
		return nil
	}
	if activeTransport.AllowInsecure {
		fmt.Fprintf(os.Stderr, "hydra-cli: warning: sending credentials over plain http to %s (%s is set); use https when the network is not trusted\n", parsed.Host, allowInsecureEnv)
		return nil
	}
	return newCliError(ExitConfigError, fmt.Errorf(
		"refusing to send a password or a token over plain http to %s: use an https:// server (a private CA goes in %s), or set %s=1 if this network is trusted",
		parsed.Host, caFileEnv, allowInsecureEnv))
}
