package cluster

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

// DetectCandidateAddresses resolves all reachable candidate endpoints for this node.
// If configured addresses are provided, they are normalized with http:// and returned.
// Otherwise, it discovers all active, non-loopback unicast IPv4 interfaces and returns
// a sorted list of candidate URLs formatted as http://<ip>:<port>.
func DetectCandidateAddresses(bindAddr string, port int, configured []string) []string {
	if len(configured) > 0 {
		seen := make(map[string]bool, len(configured))
		var normalized []string
		for _, addr := range configured {
			trimmed := strings.TrimSpace(addr)
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
				trimmed = "http://" + trimmed
			}
			trimmed = strings.TrimRight(trimmed, "/")
			if !seen[trimmed] {
				seen[trimmed] = true
				normalized = append(normalized, trimmed)
			}
		}
		if len(normalized) > 0 {
			return normalized
		}
	}

	seen := make(map[string]bool)
	var candidates []string

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, a := range addrs {
				var ip net.IP
				switch v := a.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}

				if ip == nil {
					continue
				}

				ip4 := ip.To4()
				if ip4 == nil || ip4.IsLoopback() || ip4.IsMulticast() || ip4.IsUnspecified() {
					continue
				}

				cand := fmt.Sprintf("http://%s:%d", ip4.String(), port)
				if !seen[cand] {
					seen[cand] = true
					candidates = append(candidates, cand)
				}
			}
		}
	}

	if len(candidates) == 0 {
		if bindAddr != "" && bindAddr != "0.0.0.0" && bindAddr != "::" {
			candidates = append(candidates, fmt.Sprintf("http://%s:%d", bindAddr, port))
		} else {
			candidates = append(candidates, fmt.Sprintf("http://127.0.0.1:%d", port))
		}
	}

	sort.Strings(candidates)
	return candidates
}
