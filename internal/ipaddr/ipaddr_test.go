// Copyright 2026 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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
		{"[::1]", netip.MustParseAddr("::1")},                  // brackets stripped by url.Hostname
		{"fe80::1%25eth0", netip.MustParseAddr("fe80::1")},     // zone-qualified
		{"::ffff:127.0.0.1", netip.MustParseAddr("127.0.0.1")}, // IPv4-mapped
	} {
		// url.Hostname() strips IPv6 brackets; mimic that for the bracketed case.
		host := tc.host
		if len(host) > 2 && host[0] == '[' && host[len(host)-1] == ']' {
			host = host[1 : len(host)-1]
		}
		got, ok := Parse(host)
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
		{"127.0.0.1", true},
		{"2130706433", true},
		{"0x7f000001", true},
		{"127.1", true},
		{"127.000.000.001", true},
		{"169.254.169.254", true},
		{"2852039166", true},
		{"::1", true},
		{"fe80::1", true},
		{"fe80::1%25eth0", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"0.0.0.0", true},
		{"::ffff:169.254.169.254", true},
		{"example.com", false},
		{"8.8.8.8", false},
		{"3132109884", false}, // 186.84.175.92
	} {
		if got := IsPrivateOrLinkLocal(tc.host); got != tc.want {
			t.Errorf("IsPrivateOrLinkLocal(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}
