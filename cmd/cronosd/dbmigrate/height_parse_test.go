package dbmigrate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHeightFlag(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        HeightRange
		wantErr     bool
		errContains string
	}{
		{
			name:  "empty string",
			input: "",
			want:  HeightRange{},
		},
		{
			name:  "single height",
			input: "123456",
			want:  HeightRange{SpecificHeights: []int64{123456}},
		},
		{
			name:  "range",
			input: "10000-20000",
			want:  HeightRange{Start: 10000, End: 20000},
		},
		{
			name:  "range with spaces",
			input: "10000 - 20000",
			want:  HeightRange{Start: 10000, End: 20000},
		},
		{
			name:  "multiple heights",
			input: "123456,234567,999999",
			want:  HeightRange{SpecificHeights: []int64{123456, 234567, 999999}},
		},
		{
			name:  "multiple heights with spaces",
			input: "123456, 234567, 999999",
			want:  HeightRange{SpecificHeights: []int64{123456, 234567, 999999}},
		},
		{
			name:  "two heights",
			input: "100000,200000",
			want:  HeightRange{SpecificHeights: []int64{100000, 200000}},
		},
		{
			name:    "negative single height",
			input:   "-123",
			wantErr: true,
			// parsed as range with empty start, error is "invalid start height"
		},
		{
			name:    "negative range start",
			input:   "-100-200",
			wantErr: true,
			// multiple dashes cause "invalid range format"
		},
		{
			name:    "negative range end",
			input:   "100--200",
			wantErr: true,
			// multiple dashes cause "invalid range format"
		},
		{
			name:        "invalid range - start > end",
			input:       "20000-10000",
			wantErr:     true,
			errContains: "greater than",
		},
		{
			name:        "invalid format",
			input:       "abc",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "invalid range format - too many parts",
			input:       "10-20-30",
			wantErr:     true,
			errContains: "invalid range format",
		},
		{
			name:        "empty with commas",
			input:       ",,,",
			wantErr:     true,
			errContains: "no valid heights",
		},
		{
			name:  "mixed valid and empty heights",
			input: "123456,,234567",
			want:  HeightRange{SpecificHeights: []int64{123456, 234567}},
		},
		{
			name:        "invalid height in list",
			input:       "123456,abc,234567",
			wantErr:     true,
			errContains: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHeightFlag(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHeightRange_IsWithinRange_SpecificHeights(t *testing.T) {
	tests := []struct {
		name   string
		hr     HeightRange
		height int64
		want   bool
	}{
		{
			name:   "single height - match",
			hr:     HeightRange{SpecificHeights: []int64{123456}},
			height: 123456,
			want:   true,
		},
		{
			name:   "single height - no match",
			hr:     HeightRange{SpecificHeights: []int64{123456}},
			height: 123457,
			want:   false,
		},
		{
			name:   "multiple heights - first match",
			hr:     HeightRange{SpecificHeights: []int64{100, 200, 300}},
			height: 100,
			want:   true,
		},
		{
			name:   "multiple heights - middle match",
			hr:     HeightRange{SpecificHeights: []int64{100, 200, 300}},
			height: 200,
			want:   true,
		},
		{
			name:   "multiple heights - last match",
			hr:     HeightRange{SpecificHeights: []int64{100, 200, 300}},
			height: 300,
			want:   true,
		},
		{
			name:   "multiple heights - no match",
			hr:     HeightRange{SpecificHeights: []int64{100, 200, 300}},
			height: 150,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hr.IsWithinRange(tt.height)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHeightRange_String_SpecificHeights(t *testing.T) {
	tests := []struct {
		name string
		hr   HeightRange
		want string
	}{
		{
			name: "single height",
			hr:   HeightRange{SpecificHeights: []int64{123456}},
			want: "height 123456",
		},
		{
			name: "two heights",
			hr:   HeightRange{SpecificHeights: []int64{100, 200}},
			want: "heights 100, 200",
		},
		{
			name: "five heights",
			hr:   HeightRange{SpecificHeights: []int64{100, 200, 300, 400, 500}},
			want: "heights 100, 200, 300, 400, 500",
		},
		{
			name: "many heights (shows count)",
			hr:   HeightRange{SpecificHeights: []int64{100, 200, 300, 400, 500, 600}},
			want: "6 specific heights",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hr.String()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHeightRange_HasSpecificHeights(t *testing.T) {
	tests := []struct {
		name string
		hr   HeightRange
		want bool
	}{
		{
			name: "empty",
			hr:   HeightRange{},
			want: false,
		},
		{
			name: "range only",
			hr:   HeightRange{Start: 100, End: 200},
			want: false,
		},
		{
			name: "specific heights",
			hr:   HeightRange{SpecificHeights: []int64{100}},
			want: true,
		},
		{
			name: "both (specific takes precedence)",
			hr:   HeightRange{Start: 100, End: 200, SpecificHeights: []int64{150}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hr.HasSpecificHeights()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHeightRange_IsEmpty_WithSpecificHeights(t *testing.T) {
	tests := []struct {
		name string
		hr   HeightRange
		want bool
	}{
		{
			name: "completely empty",
			hr:   HeightRange{},
			want: true,
		},
		{
			name: "has specific heights",
			hr:   HeightRange{SpecificHeights: []int64{100}},
			want: false,
		},
		{
			name: "has range",
			hr:   HeightRange{Start: 100, End: 200},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hr.IsEmpty()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHeightRange_Validate_SpecificHeights(t *testing.T) {
	tests := []struct {
		name    string
		hr      HeightRange
		wantErr bool
	}{
		{
			name:    "valid specific heights",
			hr:      HeightRange{SpecificHeights: []int64{100, 200, 300}},
			wantErr: false,
		},
		{
			name:    "specific height with negative",
			hr:      HeightRange{SpecificHeights: []int64{100, -200, 300}},
			wantErr: true,
		},
		{
			name:    "specific height zero (valid)",
			hr:      HeightRange{SpecificHeights: []int64{0, 100}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hr.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
