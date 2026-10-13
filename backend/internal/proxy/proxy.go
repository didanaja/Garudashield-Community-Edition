// Package proxy implements a hardened, single-upstream reverse proxy built on
// net/http/httputil. Clients cannot choose the upstream; it is fixed by
// configuration and validated before use.
package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"garudashield/internal/middleware"
)

// hopHeaders are removed from both directions. Connection-scoped headers must
// never be forwarded by a proxy.
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// untrustedForwardingHeaders are deleted from inbound requests before the proxy
// attaches its own trusted values. A client must never be able to spoof these.
var untrustedForwardingHeaders = []string{
	"Forwarded",
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"X-Forwarded-Port",
	"X-Real-Ip",
}

// Options configures a Proxy.
type Options struct {
	UpstreamURL        *url.URL
	Timeout            time.Duration
	ResponseHeaderWait time.Duration
	MaxIdleConns       int
}

// Proxy is a validated single-upstream reverse proxy.
type Proxy struct {
	upstream *url.URL
	rp       *httputil.ReverseProxy
	host     string
}

// New validates the upstream and constructs a Proxy. An invalid upstream is a
// hard error: the engine refuses to start rather than proxy to an unsafe target.
func New(opts Options) (*Proxy, error) {
	if err := ValidateUpstream(opts.UpstreamURL, true); err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: nil, // never honour environment proxies; keeps destination deterministic
		DialContext: (&net.Dialer{
			Timeout:   opts.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxInt(opts.MaxIdleConns, 50),
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   opts.Timeout,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: opts.ResponseHeaderWait,
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(opts.UpstreamURL)
			pr.Out.Host = opts.UpstreamURL.Host

			for _, h := range untrustedForwardingHeaders {
				pr.Out.Header.Del(h)
			}
			for _, h := range hopHeaders {
				pr.Out.Header.Del(h)
				pr.In.Header.Del(h)
			}

			if ip := middleware.ClientIPFromContext(pr.In.Context()); ip != "" {
				pr.Out.Header.Set("X-Forwarded-For", ip)
			}
			pr.Out.Header.Set("X-Forwarded-Proto", schemeOf(pr.In))
			if pr.In.Host != "" {
				pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)
			}
			pr.Out.Header.Set("X-Garuda-Shield", "demo-edition")
		},
		Transport:     transport,
		FlushInterval: 100 * time.Millisecond,
		// Never leak internal error detail to the client.
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			writeGatewayError(w)
		},
		ModifyResponse: func(resp *http.Response) error {
			// Remove headers that can leak upstream infrastructure identity.
			resp.Header.Del("Server")
			for _, h := range hopHeaders {
				resp.Header.Del(h)
			}
			return nil
		},
	}

	return &Proxy{
		upstream: opts.UpstreamURL,
		rp:       rp,
		host:     opts.UpstreamURL.Host,
	}, nil
}

// ServeHTTP forwards the request to the configured upstream.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.rp.ServeHTTP(w, r)
}

// Upstream returns the configured upstream URL (used for status reporting,
// never exposed verbatim to unauthenticated clients).
func (p *Proxy) Upstream() *url.URL { return p.upstream }

// Healthy performs a lightweight TCP reachability probe of the upstream.
func (p *Proxy) Healthy(timeout time.Duration) bool {
	u := p.upstream
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		if u.Scheme == "https" {
			host = net.JoinHostPort(host, "443")
		} else {
			host = net.JoinHostPort(host, "80")
		}
	}
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ValidateUpstream enforces the SSRF guardrails for a configured destination.
//
// allowPrivate gates loopback/RFC1918 destinations. The demo permits them so a
// local test upstream can be used, but cloud metadata endpoints and other
// dangerous targets are ALWAYS rejected.
func ValidateUpstream(u *url.URL, allowPrivate bool) error {
	if u == nil || u.Host == "" {
		return fmt.Errorf("upstream must be an absolute URL with a host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("upstream scheme must be http or https")
	}
	if u.User != nil {
		return fmt.Errorf("upstream URL must not contain userinfo credentials")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("upstream URL must not contain a query or fragment")
	}

	host := u.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		// If the name does not resolve we still reject obviously-literal unsafe
		// hosts; a name that fails to resolve simply will not work at runtime.
		if ip := net.ParseIP(host); ip != nil {
			ips = []net.IP{ip}
		} else {
			return nil
		}
	}
	for _, ip := range ips {
		if err := checkIP(ip, allowPrivate); err != nil {
			return err
		}
	}
	return nil
}

func checkIP(ip net.IP, allowPrivate bool) error {
	if ip.IsUnspecified() {
		return fmt.Errorf("upstream must not target an unspecified address")
	}
	if ip.IsMulticast() {
		return fmt.Errorf("upstream must not target a multicast address")
	}
	// Cloud metadata endpoints are always prohibited.
	if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
		return fmt.Errorf("upstream must not target a cloud metadata endpoint")
	}
	// Link-local addresses are never a legitimate upstream.
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("upstream must not target a link-local address")
	}
	if ip.IsLoopback() || ip.IsPrivate() {
		if !allowPrivate {
			return fmt.Errorf("upstream must not target a private or loopback address")
		}
	}
	return nil
}

func schemeOf(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func writeGatewayError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte(`{"error":"upstream_unavailable","message":"The configured upstream could not be reached."}`))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SanitizeHost returns a redacted representation of an upstream host suitable
// for unauthenticated status output.
func SanitizeHost(u *url.URL) string {
	if u == nil {
		return "unconfigured"
	}
	host := u.Hostname()
	// Mask all but the first label to avoid leaking internal hostnames.
	parts := strings.Split(host, ".")
	if len(parts) <= 1 {
		return host
	}
	return parts[0] + ".***"
}
