package security

import "testing"

func TestIsEnvKeySensitive(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"RESTIC_PASSWORD", true},
		{"RESTIC_PASSWORD_FILE", true},
		{"OS_PASSWORD", true},
		{"SMTP_PASSWORD", true},
		{"SWIFT_API_KEY", true},
		{"ST_AUTH", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"RESTIC_REPOSITORY", false},
		{"SS_BACKUP_SITE_NAME", false},
		{"PATH", false},
	}
	for _, tt := range tests {
		if got := IsEnvKeySensitive(tt.key); got != tt.want {
			t.Errorf("IsEnvKeySensitive(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestFilterYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "colon style redacted",
			input: "password: supersecret",
			want:  "password: [REDACTED]",
		},
		{
			name:  "equals style redacted (previously a gap)",
			input: "RESTIC_PASSWORD=supersecret",
			want:  "RESTIC_PASSWORD= [REDACTED]",
		},
		{
			name:  "non-sensitive line passes through",
			input: "snapshot_id: abc123",
			want:  "snapshot_id: abc123",
		},
		{
			name:  "swift credential redacted",
			input: "SWIFT_API_KEY=xyz",
			want:  "SWIFT_API_KEY= [REDACTED]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterYAML(tt.input); got != tt.want {
				t.Errorf("FilterYAML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
