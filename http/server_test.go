package http

import (
	"testing"
)

func TestParseBytesRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *bytesRange
		wantErr bool
	}{
		{
			name:    "valid range",
			input:   "bytes=0-10",
			want:    &bytesRange{start: 0, end: 10},
			wantErr: false,
		},
		{
			name:    "open ended range",
			input:   "bytes=5-",
			want:    &bytesRange{start: 5, end: -1},
			wantErr: false,
		},
		{
			name:    "invalid prefix",
			input:   "invalid=0-10",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid range format",
			input:   "bytes=abc-def",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "start greater than end",
			input:   "bytes=10-5",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "negative start open ended",
			input:   "bytes=-1",
			want:    &bytesRange{start: -1, end: -1},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBytesRange(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBytesRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && tt.want != nil {
				if got.start != tt.want.start || got.end != tt.want.end {
					t.Errorf("parseBytesRange() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestBytesRangeContentRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		size    int64
		want    string
		wantErr bool
	}{
		{
			name:    "valid range",
			input:   "bytes=0-10",
			size:    10,
			want:    "bytes 0-10/10",
			wantErr: false,
		},
		{
			name:    "open ended range",
			input:   "bytes=5-",
			size:    10,
			want:    "bytes 5-9/10",
			wantErr: false,
		},
		{
			name:    "negative start",
			input:   "bytes=-10",
			size:    10,
			want:    "bytes 0-9/10",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpRange, err := parseBytesRange(tt.input)
			if err != nil {
				t.Errorf("parseBytesRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got := httpRange.ContentRange(tt.size)
			if got != tt.want {
				t.Errorf("ContentRange() = %v, want %v", got, tt.want)
			}
		})
	}
}
