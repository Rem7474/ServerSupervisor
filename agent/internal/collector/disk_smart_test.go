package collector

import "testing"

func TestParseSMARTRawValue(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"0", 0},
		{"12345", 12345},
		{"2147483647", 2147483647},
		{"2147483648", 0},
		{"-1", 0},
		{"abc", 0},
		{"", 0},
		{"34 (Min/Max 20/45)", 0},
	}
	for _, tt := range tests {
		if got := parseSMARTRawValue(tt.in); got != tt.want {
			t.Errorf("parseSMARTRawValue(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestSmartCount(t *testing.T) {
	tests := []struct {
		in   float64
		want int
	}{
		{0, 0}, {42, 42}, {2147483647, 2147483647},
		{2147483648, 0}, {-5, 0}, {1e300, 0},
	}
	for _, tt := range tests {
		if got := smartCount(tt.in); got != tt.want {
			t.Errorf("smartCount(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
