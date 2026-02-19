package limitio

import (
	"errors"
	"strings"
	"testing"
)

func TestReadAll(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		max     int64
		wantErr bool
		wantLen int
	}{
		{
			name:    "within limit",
			input:   "hello",
			max:     10,
			wantLen: 5,
		},
		{
			name:    "exactly at limit",
			input:   "hello",
			max:     5,
			wantLen: 5,
		},
		{
			name:    "exceeds limit",
			input:   "hello world",
			max:     5,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			max:     5,
			wantLen: 0,
		},
		{
			name:    "negative max",
			input:   "hello",
			max:     -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadAll(strings.NewReader(tt.input), tt.max)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.max >= 0 && !errors.Is(err, ErrLimitExceeded) {
					t.Fatalf("expected ErrLimitExceeded, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("got %d bytes, want %d", len(got), tt.wantLen)
			}
		})
	}
}
