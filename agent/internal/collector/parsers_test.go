package collector

import "testing"

func TestParseAccessLine_NPM(t *testing.T) {
	line := `[09/May/2024:12:34:56 +0000] - - 200 - GET https example.com "/index.html?a=1" [Client 1.2.3.4] [Length 1234] [Gzip -] "Mozilla/5.0" "-"`
	got, ok := parseAccessLine(line)
	if !ok {
		t.Fatal("expected NPM line to parse")
	}
	if got.source != "npm" {
		t.Errorf("source = %q, want npm", got.source)
	}
	if got.method != "GET" {
		t.Errorf("method = %q, want GET", got.method)
	}
	if got.status != 200 {
		t.Errorf("status = %d, want 200", got.status)
	}
	if got.ip != "1.2.3.4" {
		t.Errorf("ip = %q, want 1.2.3.4", got.ip)
	}
	if got.domain != "example.com" {
		t.Errorf("domain = %q, want example.com", got.domain)
	}
	if got.path != "/index.html" { // query string stripped by cleanPath
		t.Errorf("path = %q, want /index.html (query stripped)", got.path)
	}
	if got.bytes != 1234 {
		t.Errorf("bytes = %d, want 1234", got.bytes)
	}
}

func TestParseAccessLine_Common(t *testing.T) {
	line := `1.2.3.4 - - [09/May/2024:12:34:56 +0000] "GET /admin HTTP/1.1" 404 512 "-" "sqlmap/1.5"`
	got, ok := parseAccessLine(line)
	if !ok {
		t.Fatal("expected common line to parse")
	}
	if got.source != "nginx" {
		t.Errorf("source = %q, want nginx", got.source)
	}
	if got.method != "GET" || got.path != "/admin" || got.status != 404 {
		t.Errorf("got method=%q path=%q status=%d", got.method, got.path, got.status)
	}
	if got.ip != "1.2.3.4" {
		t.Errorf("ip = %q, want 1.2.3.4", got.ip)
	}
	if got.bytes != 512 {
		t.Errorf("bytes = %d, want 512", got.bytes)
	}
	if got.ua != "sqlmap/1.5" {
		t.Errorf("ua = %q, want sqlmap/1.5", got.ua)
	}
	if got.domain != "(unknown)" {
		t.Errorf("domain = %q, want (unknown)", got.domain)
	}
}

func TestParseAccessLine_DashBytes(t *testing.T) {
	line := `1.2.3.4 - - [09/May/2024:12:34:56 +0000] "HEAD / HTTP/1.1" 304 - "-" "curl/8"`
	got, ok := parseAccessLine(line)
	if !ok {
		t.Fatal("expected line to parse")
	}
	if got.bytes != 0 {
		t.Errorf("bytes = %d, want 0 for '-'", got.bytes)
	}
}

func TestParseAccessLine_Garbage(t *testing.T) {
	for _, line := range []string{"", "not a log line", "random 12345 text"} {
		if _, ok := parseAccessLine(line); ok {
			t.Errorf("expected %q to fail parsing", line)
		}
	}
}

func TestSuspiciousCategory(t *testing.T) {
	tests := []struct {
		name             string
		method, path, ua string
		want             string
	}{
		{"wordpress wp-login", "GET", "/wp-login.php", "x", "WordPress"},
		{"wordpress xmlrpc", "POST", "/xmlrpc.php", "x", "WordPress"},
		{"admin panel", "GET", "/admin/config", "x", "AdminPanel"},
		{"phpmyadmin", "GET", "/phpmyadmin/index.php", "x", "AdminPanel"},
		{"path traversal etc passwd", "GET", "/../../etc/passwd", "x", "PathTraversal"},
		{"known scanner path .env", "GET", "/.env", "x", "KnownScanner"},
		{"known scanner UA sqlmap", "GET", "/", "sqlmap/1.5", "KnownScanner"},
		{"suspicious method PROPFIND", "PROPFIND", "/", "x", "SuspiciousMethod"},
		{"benign request", "GET", "/index.html", "Mozilla/5.0", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := suspiciousCategory(tt.method, tt.path, tt.ua); got != tt.want {
				t.Errorf("suspiciousCategory(%q,%q,%q) = %q, want %q", tt.method, tt.path, tt.ua, got, tt.want)
			}
		})
	}
}

func TestParseImageTag(t *testing.T) {
	tests := []struct {
		in        string
		wantImage string
		wantTag   string
	}{
		{"nginx:1.2.3", "nginx", "1.2.3"},
		{"nginx", "nginx", "latest"},
		{"ghcr.io/org/image:v2", "ghcr.io/org/image", "v2"},
		// Registry port but no tag -> the colon precedes a path, so default latest.
		{"registry:5000/org/image", "registry:5000/org/image", "latest"},
		// Registry port and a tag.
		{"registry:5000/org/image:v3", "registry:5000/org/image", "v3"},
	}
	for _, tt := range tests {
		image, tag := parseImageTag(tt.in)
		if image != tt.wantImage || tag != tt.wantTag {
			t.Errorf("parseImageTag(%q) = (%q,%q), want (%q,%q)", tt.in, image, tag, tt.wantImage, tt.wantTag)
		}
	}
}
