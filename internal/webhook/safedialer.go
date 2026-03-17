package webhook

import (
	"context"
	"fmt"
	"net"
)

// privateRanges contains CIDR ranges that should be blocked for webhook
// delivery to prevent SSRF attacks targeting internal services.
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local / cloud metadata
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("invalid CIDR in privateRanges: " + cidr)
		}
		privateRanges = append(privateRanges, network)
	}
}

// IsPrivateIP reports whether ip falls within a private or reserved range.
func IsPrivateIP(ip net.IP) bool {
	for _, network := range privateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// safeDialer wraps net.Dialer and blocks connections to private/reserved IPs.
type safeDialer struct {
	dialer   net.Dialer
	resolver *net.Resolver
}

func newSafeDialer() *safeDialer {
	return &safeDialer{
		resolver: net.DefaultResolver,
	}
}

// DialContext resolves the hostname and blocks connections to private IPs.
func (d *safeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("splitting host:port: %w", err)
	}

	ips, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", host, err)
	}

	for _, ip := range ips {
		if IsPrivateIP(ip.IP) {
			return nil, fmt.Errorf("webhook URL resolves to private/reserved IP %s", ip.IP)
		}
	}

	return d.dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}
