package transport

import (
	"testing"
)

func TestDefaultUserAgent(t *testing.T) {
	for _, tc := range []struct {
		defaultUA string
		ua        string
		want      string
	}{
		{
			want: "go-containerregistry",
		},
		{
			defaultUA: "foo",
			want:      "foo go-containerregistry",
		},
		{
			ua:   "bar",
			want: "bar go-containerregistry",
		},
		{
			defaultUA: "foo",
			ua:        "bar",
			want:      "bar go-containerregistry",
		},
	} {
		t.Run("", func(t *testing.T) {
			SetDefaultUserAgent(tc.defaultUA)
			t.Cleanup(func() {
				SetDefaultUserAgent("")
			})
			rt, ok := NewUserAgent(nil, tc.ua).(*userAgentTransport)
			if !ok {
				t.Fatalf("NewUserAgent returned a %T, want *userAgentTransport", rt)
			}
			if rt.ua != tc.want {
				t.Errorf("want %q, got %q", tc.want, rt.ua)
			}
		})
	}
}
