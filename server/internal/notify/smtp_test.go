package notify

import "testing"

func TestEnvelopeAddress(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"plain address", "ops@example.com", "ops@example.com", false},
		{"display name is dropped", "Ops Team <ops@example.com>", "ops@example.com", false},
		{"CRLF smuggling is neutralised", "a@example.com\r\nRCPT TO:<evil@example.com>", "", true},
		{"CR/LF inside the local part collapses", "a\r\n@example.com", "a@example.com", false},
		{"empty", "", "", true},
		{"not an address", "not-an-address", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := envelopeAddress(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("envelopeAddress(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("envelopeAddress(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
