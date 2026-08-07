package alerts

import "testing"

func TestParseDockerComposeScopeID(t *testing.T) {
	tests := []struct {
		name            string
		scopeID         string
		wantHostID      string
		wantProjectName string
		wantOK          bool
	}{
		{
			name:            "well-formed scope ID",
			scopeID:         "docker:compose:host-uuid-1:my-stack",
			wantHostID:      "host-uuid-1",
			wantProjectName: "my-stack",
			wantOK:          true,
		},
		{
			name:    "missing project name separator",
			scopeID: "docker:compose:host-uuid-1",
			wantOK:  false,
		},
		{
			name:    "not a compose scope ID at all",
			scopeID: "host-uuid-1",
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostID, projectName, ok := parseDockerComposeScopeID(tt.scopeID)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if hostID != tt.wantHostID {
				t.Errorf("hostID = %q, want %q", hostID, tt.wantHostID)
			}
			if projectName != tt.wantProjectName {
				t.Errorf("projectName = %q, want %q", projectName, tt.wantProjectName)
			}
		})
	}
}
