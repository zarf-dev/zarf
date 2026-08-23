package httpx

import (
	"context"
	"net"
)

func DNSCacheDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	cs := map[string][]net.IP{}

	return func(ctx context.Context, nw, addr string) (conn net.Conn, err error) {
		h, p, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, ok := cs[h]
		if !ok {
			ips, err = net.DefaultResolver.LookupIP(ctx, "ip4", h)
			if len(ips) == 0 {
				ips, err = net.DefaultResolver.LookupIP(ctx, "ip", h)
			}
			if err != nil {
				return nil, err
			}
			cs[h] = ips
		}
		// Try to connect to each IP address in order.
		for _, ip := range ips {
			conn, err = dialer.DialContext(ctx, nw, net.JoinHostPort(ip.String(), p))
			if err == nil {
				break
			}
		}
		return conn, err
	}
}
