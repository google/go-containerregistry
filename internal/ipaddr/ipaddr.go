// Copyright 2026 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ipaddr

import (
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

// CheckRedirectSSRF rejects HTTP redirects that cross from a public host to a
// private or link-local IP literal. This prevents a malicious registry from
// issuing a 302 to a cloud instance metadata service (e.g. 169.254.169.254)
// or another internal network address.
//
// Same-host redirects and redirects to non-IP hostnames (including DNS names
// that may resolve to private addresses) are allowed. The first redirect in
// the chain uses the original request URL as the "origin host" via
// req.Response.Request, falling back to req.URL when no prior response exists.
func CheckRedirectSSRF(req *http.Request, via []*http.Request) error {
	if len(via) == 0 || req.Response == nil {
		return nil
	}
	origHost := via[0].URL.Hostname()
	destHost := req.URL.Hostname()
	if destHost == origHost {
		return nil // same-host redirect is always allowed
	}
	if IsPrivateOrLinkLocal(destHost) {
		return fmt.Errorf("SSRF protection: redirect from %q to private/link-local host %q denied", origHost, destHost)
	}
	return nil
}

// IsPrivateOrLinkLocal reports whether host denotes a loopback, private,
// link-local, or unspecified address. It accepts any IP-literal form the Go
// dialer accepts — canonical dotted-quad IPv4 and IPv6, zone-qualified and
// IPv4-mapped IPv6, and legacy inet_aton encodings (32-bit decimal
// "2130706433", hexadecimal "0x7f000001", partial dotted-quad "127.1",
// zero-padded octets) — so a guard based on it cannot be bypassed by
// spelling an internal address in a non-canonical way. DNS names are not IP
// literals and return false.
func IsPrivateOrLinkLocal(host string) bool {
	addr, ok := Parse(host)
	if !ok {
		return false
	}
	return addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsPrivate() || addr.IsUnspecified()
}

// Parse parses an IP literal in any form the Go dialer accepts.
func Parse(host string) (netip.Addr, bool) {
	// The dialer ignores IPv6 zone identifiers, so guards must too.
	host, _, _ = strings.Cut(host, "%")
	if addr, err := netip.ParseAddr(host); err == nil {
		// Treat IPv4-mapped IPv6 as the IPv4 address the dialer connects to.
		return addr.WithZone("").Unmap(), true
	}
	return parseLegacyIPv4(host)
}

// parseLegacyIPv4 implements the inet_aton forms the Go resolver accepts for
// IPv4: one to four dot-separated parts where the final part may fill the
// remaining bytes ("127.1" == 127.0.0.1) and each part may be decimal,
// hexadecimal (0x prefix), or octal (leading 0).
func parseLegacyIPv4(host string) (netip.Addr, bool) {
	parts := strings.Split(host, ".")
	if len(parts) < 1 || len(parts) > 4 {
		return netip.Addr{}, false
	}
	var b [4]byte
	for i, part := range parts[:len(parts)-1] {
		// bitsize 8 enforces the single-octet range before any conversion.
		v, err := strconv.ParseUint(part, 0, 8)
		if err != nil {
			return netip.Addr{}, false
		}
		b[i] = byte(v)
	}
	// The final part may fill as many bytes as remain, e.g. "127.1" -> 127.0.0.1;
	// with four parts it must still be a single octet. The bitsize enforces the
	// range before the conversion below.
	lastBits := [...]int{32, 24, 16, 8}[len(parts)-1]
	last, err := strconv.ParseUint(parts[len(parts)-1], 0, lastBits)
	if err != nil {
		return netip.Addr{}, false
	}
	v := uint32(last)
	for i := len(parts) - 1; i < 4; i++ {
		b[i] = byte(v >> (8 * (3 - i)))
	}
	return netip.AddrFrom4(b), true
}
