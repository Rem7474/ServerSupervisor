package proxmoxclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPVEStorageSupportsBackup(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"backup", true},
		{"images,rootdir,backup,iso", true},
		{"images, backup ", true}, // PVE has been seen padding the list
		{"images,rootdir,iso", false},
		{"", false},
		{"backups", false}, // near-miss must not match
	}
	for _, tc := range tests {
		if got := (PVEStorage{Content: tc.content}).SupportsBackup(); got != tc.want {
			t.Errorf("content %q: SupportsBackup() = %v, want %v", tc.content, got, tc.want)
		}
	}
}

func TestPVEStorageContentGuestID(t *testing.T) {
	// PVE has returned vmid as both a JSON number and a string across versions.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
			{"volid":"s:backup/a","vmid":100,"ctime":1000},
			{"volid":"s:backup/b","vmid":"101","ctime":2000},
			{"volid":"s:backup/c","ctime":3000},
			{"volid":"s:backup/d","vmid":"not-a-number","ctime":4000}
		]}`))
	}))
	defer srv.Close()

	entries, err := New(srv.URL, "u!t", "s", false).GetStorageBackups("pve1", "backups")
	if err != nil {
		t.Fatalf("GetStorageBackups: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(entries))
	}
	want := []int{100, 101, 0, 0}
	for i, w := range want {
		if got := entries[i].GuestID(); got != w {
			t.Errorf("entry %d: GuestID() = %d, want %d", i, got, w)
		}
	}
}

func TestGetStorageBackupsRequestsBackupContentOnly(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "u!t", "s", false).GetStorageBackups("pve1", "local-lvm"); err != nil {
		t.Fatalf("GetStorageBackups: %v", err)
	}
	// Listing every content type on a large storage would be needlessly heavy.
	if gotQuery != "content=backup" {
		t.Errorf("query = %q, want %q", gotQuery, "content=backup")
	}
	if gotPath != "/nodes/pve1/storage/local-lvm/content" {
		t.Errorf("path = %q", gotPath)
	}
}
