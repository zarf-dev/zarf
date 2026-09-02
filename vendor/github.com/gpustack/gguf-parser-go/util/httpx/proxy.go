package httpx

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gpustack/gguf-parser-go/util/osx"
)

var noProxies []*net.IPNet

func init() {
	noProxyEnv := osx.Getenv("NO_PROXY", osx.Getenv("no_proxy"))
	noProxyRules := strings.Split(noProxyEnv, ",")
	for i := range noProxyRules {
		_, cidr, _ := net.ParseCIDR(noProxyRules[i])
		if cidr != nil {
			noProxies = append(noProxies, cidr)
		}
	}
}

// ProxyFromEnvironment is similar to http.ProxyFromEnvironment,
// but it also respects the NO_PROXY environment variable.
func ProxyFromEnvironment(r *http.Request) (*url.URL, error) {
	if ip := net.ParseIP(r.URL.Hostname()); ip != nil {
		for i := range noProxies {
			if noProxies[i].Contains(ip) {
				return nil, nil
			}
		}
	}

	return http.ProxyFromEnvironment(r)
}
