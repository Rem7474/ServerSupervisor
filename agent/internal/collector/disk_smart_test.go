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

func TestParseSmartText(t *testing.T) {
	out := `Device Model:     Samsung SSD 860
Serial Number:    S3Z9NB0K123456
SMART overall-health self-assessment test result: PASSED
ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE
  5 Reallocated_Sector_Ct   0x0033   100   100   010    Pre-fail  Always       -       8
  9 Power_On_Hours          0x0032   095   095   000    Old_age   Always       -       21450
 12 Power_Cycle_Count       0x0032   099   099   000    Old_age   Always       -       310
197 Current_Pending_Sector  0x0032   100   100   000    Old_age   Always       -       2
198 Offline_Uncorrectable   0x0030   100   100   000    Old_age   Offline      -       4294967296
`
	h, err := parseSmartText("/dev/sda", out)
	if err != nil {
		t.Fatalf("parseSmartText: %v", err)
	}
	if h.Model != "Samsung SSD 860" || h.SerialNumber != "S3Z9NB0K123456" || h.SMARTStatus != "PASSED" {
		t.Errorf("identity/status = %q %q %q", h.Model, h.SerialNumber, h.SMARTStatus)
	}
	if h.ReallocatedSectors != 8 || h.PowerOnHours != 21450 || h.PowerCycles != 310 || h.PendingSectors != 2 {
		t.Errorf("counters = %+v", h)
	}
	if h.UncorrectableSectors != 0 {
		t.Errorf("out-of-range raw value must yield 0, got %d", h.UncorrectableSectors)
	}
}
