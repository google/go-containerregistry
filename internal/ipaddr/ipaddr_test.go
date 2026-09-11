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
	"testing"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		host string
		want netip.Addr
	}{
		{"127.0.0.1", netip.MustParseAddr("127.0.0.1")},
		{"2130706433", netip.MustParseAddr("127.0.0.1")},       // 32-bit decimal
		{"0x7f000001", netip.MustParseAddr("127.0.0.1")},       // hex
		{"127.1", netip.MustParseAddr("127.0.0.1")},            // partial dotted-quad
		{"127.000.000.001", netip.MustParseAddr("127.0.0.1")},  // zero-padded
		{"2852039166", netip.MustParseAddr("169.254.169.254")}, // metadata as decimal
		{"0177.0.0.1", netip.MustParseAddr("127.0.0.1")},       // octal
		{"[::1]", netip.MustParseAddr("::1")},                  // bracketed IPv6
		{"[64:ff9b::a9fe:a9fe]", netip.MustParseAddr("64:ff9b::a9fe:a9fe")}, // bracketed NAT64
		{"fe80::1%25eth0", netip.MustParseAddr("fe80::1")},     // zone-qualified
		{"::ffff:127.0.0.1", netip.MustParseAddr("127.0.0.1")}, // IPv4-mapped
	} {
		got, ok := Parse(tc.host)
		if !ok {
			t.Errorf("Parse(%q) failed, want %v", tc.host, tc.want)
			continue
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
	for _, host := range []string{"example.com", "localhost", "127.0.0.256", "1.2.3.4.5", ""} {
		if _, ok := Parse(host); ok {
			t.Errorf("Parse(%q) succeeded, want failure", host)
		}
	}
}

func TestIsPrivateOrLinkLocal(t *testing.T) {
	for _, tc := range []struct {
		host string
		want bool
	}{
		// Standard loopback and private IPv4/IPv6
		{"127.0.0.1", true},
		{"2130706433", true},
		{"0x7f000001", true},
		{"127.1", true},
		{"127.000.000.001", true},
		{"169.254.169.254", true},
		{"2852039166", true},
		{"::1", true},
		{"[::1]", true},
		{"fe80::1", true},
		{"fe80::1%25eth0", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"0.0.0.0", true},
		{"::ffff:169.254.169.254", true},

		// Carrier-Grade NAT (CGNAT) / Shared Address Space (RFC 6598: 100.64.0.0/10)
		{"100.64.0.1", true},
		{"100.127.255.254", true},
		{"100.128.0.1", false},

		// "This host on this network" (RFC 1122: 0.0.0.0/8)
		{"0.1.2.3", true},
		{"0.255.255.255", true},

		// Limited broadcast & reserved (RFC 919 / RFC 1112: 240.0.0.0/4)
		{"255.255.255.255", true},
		{"240.0.0.1", true},

		// Deprecated IPv6 site-local unicast (RFC 3879: fec0::/10)
		{"fec0::1", true},
		{"[fec0::1]", true},

		// NAT64 well-known prefix (RFC 6052: 64:ff9b::/96)
		{"64:ff9b::169.254.169.254", true},
		{"64:ff9b::a9fe:a9fe", true},
		{"[64:ff9b::a9fe:a9fe]", true},
		{"64:ff9b::10.0.0.1", true},
		{"64:ff9b::0a00:0001", true},
		{"64:ff9b::127.0.0.1", true},
		{"64:ff9b::100.64.1.1", true},
		{"64:ff9b::8.8.8.8", false}, // Public IPv4 translated via NAT64 is not private
		{"64:ff9b::1.1.1.1", false},

		// NAT64 local-use prefix (RFC 8215: 64:ff9b:1::/48)
		{"64:ff9b:1::1", true},
		{"[64:ff9b:1::abcd]", true},

		// 6to4 encapsulation (RFC 3056: 2002::/16)
		{"2002:0a00:0001::", true}, // embeds 10.0.0.1
		{"2002:a9fe:a9fe::", true}, // embeds 169.254.169.254
		{"2002:7f00:0001::", true}, // embeds 127.0.0.1
		{"2002:6440:0101::", true}, // embeds 100.64.1.1
		{"2002:0808:0808::", false}, // embeds 8.8.8.8 (public IPv4)

		// IPv4-compatible IPv6 (RFC 4291: ::/96)
		{"::10.0.0.1", true},
		{"::169.254.169.254", true},
		{"::a9fe:a9fe", true},
		{"::8.8.8.8", false},

		// ISATAP (RFC 5214)
		{"fe80::0200:5efe:10.0.0.1", true},
		{"2001:db8::5efe:169.254.169.254", true},
		{"2001:db8::5efe:8.8.8.8", false},

		// Teredo tunneling (RFC 4380: 2001::/32)
		// Client IPv4 169.254.169.254 (0xa9fe:a9fe) XOR 0xffffffff = 5601:5601
		{"2001:0000:4136:e378:8000:63bf:5601:5601", true},
		// Client IPv4 8.8.8.8 (0x0808:0808) XOR 0xffffffff = f7f7:f7f7
		{"2001:0000:4136:e378:8000:63bf:f7f7:f7f7", false},

		// Public addresses
		{"example.com", false},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"3132109884", false}, // 186.84.175.92
		{"2607:f8b0:4005:805::200e", false},
		{"2001:4860:4860::8888", false},
	} {
		if got := IsPrivateOrLinkLocal(tc.host); got != tc.want {
			t.Errorf("IsPrivateOrLinkLocal(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func BenchmarkIsPrivateOrLinkLocal(b *testing.B) {
	hosts := []string{
		"127.0.0.1",
		"169.254.169.254",
		"100.64.0.1",
		"64:ff9b::a9fe:a9fe",
		"2002:a9fe:a9fe::",
		"::1",
		"8.8.8.8",
		"example.com",
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = IsPrivateOrLinkLocal(hosts[i%len(hosts)])
	}
}
