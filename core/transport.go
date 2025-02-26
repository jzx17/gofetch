package core

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/http2"
)

var defaultCipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
}

func getDefaultTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:       tls.VersionTLS13,
		CipherSuites:     defaultCipherSuites,
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		Renegotiation:    tls.RenegotiateNever,
	}
}

// RoundTripFunc type is an adapter to allow the use of ordinary functions as http.RoundTripper.
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip executes a single HTTP transaction.
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TransportConfig holds configuration for creating an HTTP transport
type TransportConfig struct {
	// TLS configuration
	TLSConfig           *tls.Config
	EnableHTTP2         bool
	InsecureSkipVerify  bool
	TLSHandshakeTimeout time.Duration

	// Timeouts
	ResponseHeaderTimeout time.Duration
	ExpectContinueTimeout time.Duration

	// Connection management
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
	DisableKeepAlives   bool

	// Proxy
	ProxyURL string

	// Other settings
	DisableCompression bool
	ForceAttemptHTTP2  bool
}

// DefaultTransportConfig returns a TransportConfig with sensible defaults
func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		TLSConfig:             getDefaultTLSConfig(),
		EnableHTTP2:           true,
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0, // unlimited
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NewTransport creates a new http.Transport with the given configuration
func NewTransport(config TransportConfig) (*http.Transport, error) {
	tlsConfig := config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = getDefaultTLSConfig()
	}

	// If InsecureSkipVerify is requested, set it in the TLS config
	if config.InsecureSkipVerify {
		tlsConfig = tlsConfig.Clone()
		tlsConfig.InsecureSkipVerify = true
	}

	tr := &http.Transport{
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		DisableCompression:    config.DisableCompression,
		ForceAttemptHTTP2:     config.ForceAttemptHTTP2,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
	}

	// Configure proxy if provided
	if config.ProxyURL != "" {
		proxyURL, err := http.DefaultTransport.(*http.Transport).Proxy(
			&http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}})
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		tr.Proxy = http.ProxyURL(proxyURL)
	}

	// Enable HTTP/2 for this transport
	if config.EnableHTTP2 && !config.ForceAttemptHTTP2 {
		if err := http2.ConfigureTransport(tr); err != nil {
			return nil, fmt.Errorf("failed to configure HTTP/2 transport: %w", err)
		}
	}

	return tr, nil
}

// TLSTransport is a wrapper around http.Transport that is configured for TLS and HTTP/2.
type TLSTransport struct {
	Transport *http.Transport
}

// NewTLSTransport creates a new TLSTransport with the given TLS configuration, handshake timeout,
// maximum idle connections, and idle connection timeout. HTTP/2 is enabled for the transport.
func NewTLSTransport(enableHTTP2 bool, tlsConfig *tls.Config, tlsHandshakeTimeout time.Duration, maxIdleCons int, idleConnTimeout time.Duration) (*TLSTransport, error) {
	if tlsConfig == nil {
		tlsConfig = getDefaultTLSConfig()
	} else {
		if tlsConfig.MinVersion == 0 {
			tlsConfig.MinVersion = tls.VersionTLS13
		}
		if len(tlsConfig.CipherSuites) == 0 {
			tlsConfig.CipherSuites = defaultCipherSuites
		}
		if len(tlsConfig.CurvePreferences) == 0 {
			tlsConfig.CurvePreferences = []tls.CurveID{tls.X25519, tls.CurveP256}
		}
		if tlsConfig.Renegotiation == 0 {
			tlsConfig.Renegotiation = tls.RenegotiateNever
		}
	}

	tr := &http.Transport{
		TLSClientConfig:     tlsConfig,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
		MaxIdleConns:        maxIdleCons,
		IdleConnTimeout:     idleConnTimeout,
		ForceAttemptHTTP2:   enableHTTP2,
	}

	// Enable HTTP/2 for this transport if not using ForceAttemptHTTP2
	if enableHTTP2 && !tr.ForceAttemptHTTP2 {
		if err := http2.ConfigureTransport(tr); err != nil {
			return nil, fmt.Errorf("failed to configure HTTP/2 transport: %w", err)
		}
	}

	return &TLSTransport{Transport: tr}, nil
}

// RoundTrip delegates the round-trip to the underlying Transport.
func (tt *TLSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return tt.Transport.RoundTrip(req)
}
