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
	"net/netip"
	"strconv"
	"strings"
)

var (
	// CGNAT / Shared Address Space (RFC 6598: 100.64.0.0/10).
	cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

	// IPv4 "This host on this network" (RFC 1122: 0.0.0.0/8).
	thisNetworkPrefix = netip.MustParsePrefix("0.0.0.0/8")

	// IPv4 Reserved for future use & limited broadcast (RFC 1112 / RFC 919: 240.0.0.0/4).
	reservedPrefix = netip.MustParsePrefix("240.0.0.0/4")

	// Deprecated IPv6 site-local unicast (RFC 3879: fec0::/10).
	siteLocalPrefix = netip.MustParsePrefix("fec0::/10")

	// NAT64 local-use IPv4/IPv6 translation prefix (RFC 8215: 64:ff9b:1::/48).
	nat64LocalPrefix = netip.MustParsePrefix("64:ff9b:1::/48")

	// NAT64 well-known prefix (RFC 6052: 64:ff9b::/96).
	nat64Prefix = netip.MustParsePrefix("64:ff9b::/96")

	// 6to4 encapsulation prefix (RFC 3056: 2002::/16).
	sixToFourPrefix = netip.MustParsePrefix("2002::/16")

	// Teredo tunneling prefix (RFC 4380: 2001::/32).
	teredoPrefix = netip.MustParsePrefix("2001::/32")
)

// IsPrivateOrLinkLocal reports whether host denotes a loopback, private,
// link-local, unspecified, or non-globally-routable internal address. It accepts
// any IP-literal form the Go dialer accepts — canonical dotted-quad IPv4 and IPv6,
// zone-qualified and IPv4-mapped IPv6, and legacy inet_aton encodings (32-bit decimal
// "2130706433", hexadecimal "0x7f000001", partial dotted-quad "127.1", zero-padded
// octets) — so a guard based on it cannot be bypassed by spelling an internal address
// in a non-canonical way.
//
// In addition to standard RFC 1918 and RFC 4193 private ranges, this checks:
//   - Carrier-Grade NAT (CGNAT) / Shared Address Space (RFC 6598 100.64.0.0/10)
//   - Deprecated IPv6 site-local addresses (RFC 3879 fec0::/10)
//   - IPv4 "This host on this network" (RFC 1122 0.0.0.0/8) and reserved (240.0.0.0/4)
//   - IPv6 transition mechanisms embedding private or link-local IPv4 destinations:
//     NAT64 (RFC 6052 64:ff9b::/96, RFC 8215 64:ff9b:1::/48), 6to4 (RFC 3056 2002::/16),
//     Teredo (RFC 4380 2001::/32), ISATAP (RFC 5214), and IPv4-compatible IPv6 (RFC 4291 ::/96).
//
// DNS names are not IP literals and return false.
func IsPrivateOrLinkLocal(host string) bool {
	addr, ok := Parse(host)
	if !ok {
		return false
	}
	return isBlocked(addr)
}

func isBlocked(addr netip.Addr) bool {
	if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsPrivate() || addr.IsUnspecified() {
		return true
	}

	if addr.Is4() {
		return cgnatPrefix.Contains(addr) ||
			thisNetworkPrefix.Contains(addr) ||
			reservedPrefix.Contains(addr)
	}

	if addr.Is6() {
		if siteLocalPrefix.Contains(addr) || nat64LocalPrefix.Contains(addr) {
			return true
		}

		b := addr.As16()

		// NAT64 well-known prefix (RFC 6052): bytes 12..15 encode IPv4.
		if nat64Prefix.Contains(addr) {
			v4 := netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
			return isBlocked(v4)
		}

		// 6to4 (RFC 3056): bytes 2..5 encode IPv4 (2002:V4ADDR::/48).
		if sixToFourPrefix.Contains(addr) {
			v4 := netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]})
			return isBlocked(v4)
		}

		// IPv4-compatible IPv6 (RFC 4291 ::/96): bytes 0..11 are zero.
		if isIPv4Compatible(b) {
			v4 := netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
			return isBlocked(v4)
		}

		// Teredo (RFC 4380 2001::/32): bytes 4..7 encode server IPv4, bytes 12..15 encode inverted client IPv4.
		if teredoPrefix.Contains(addr) {
			server := netip.AddrFrom4([4]byte{b[4], b[5], b[6], b[7]})
			client := netip.AddrFrom4([4]byte{b[12] ^ 0xff, b[13] ^ 0xff, b[14] ^ 0xff, b[15] ^ 0xff})
			return isBlocked(server) || isBlocked(client)
		}

		// ISATAP (RFC 5214): Interface ID matches 0000:5efe:w.x.y.z or 0200:5efe:w.x.y.z.
		if (b[8] == 0x00 || b[8] == 0x02) && b[9] == 0x00 && b[10] == 0x5e && b[11] == 0xfe {
			v4 := netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
			return isBlocked(v4)
		}
	}

	return false
}

func isIPv4Compatible(b [16]byte) bool {
	for i := 0; i < 12; i++ {
		if b[i] != 0 {
			return false
		}
	}
	// Exclude :: and ::1 which are already caught by IsUnspecified and IsLoopback.
	return !(b[12] == 0 && b[13] == 0 && b[14] == 0 && (b[15] == 0 || b[15] == 1))
}

// Parse parses an IP literal in any form the Go dialer accepts.
func Parse(host string) (netip.Addr, bool) {
	// Strip optional IPv6 surrounding brackets if present.
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
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
