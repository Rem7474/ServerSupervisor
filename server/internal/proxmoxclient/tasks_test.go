package proxmoxclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestGetNodeTasksTypeFilter(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)

	if _, err := c.GetNodeTasks("pve1", 50, ""); err != nil {
		t.Fatalf("GetNodeTasks: %v", err)
	}
	if gotQuery != "limit=50" {
		t.Errorf("no typeFilter: query = %q, want %q", gotQuery, "limit=50")
	}

	if _, err := c.GetNodeTasks("pve1", 50, "vzdump"); err != nil {
		t.Fatalf("GetNodeTasks: %v", err)
	}
	if gotQuery != "limit=50&typefilter=vzdump" {
		t.Errorf("with typeFilter: query = %q, want %q", gotQuery, "limit=50&typefilter=vzdump")
	}
}

// A long task's log is paginated by PVE, which defaults to the *first* 50
// lines. The closing "TASK OK"/"TASK ERROR" marker lives at the end, so a
// caller reading the default page can never see a task finish — the symptom
// being a completed backup stuck showing "running" forever.
func TestGetNodeTaskLogReadsTheTail(t *testing.T) {
	const total = 1200
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		start, _ := strconv.Atoi(r.URL.Query().Get("start"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		lines := make([]string, 0, limit)
		for i := start; i < start+limit && i < total; i++ {
			text := fmt.Sprintf("line %d", i+1)
			if i == total-1 {
				text = "TASK ERROR: job errors"
			}
			lines = append(lines, fmt.Sprintf(`{"n":%d,"t":%q}`, i+1, text))
		}
		_, _ = fmt.Fprintf(w, `{"total":%d,"data":[%s]}`, total, strings.Join(lines, ","))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	got, gotTotal, err := c.GetNodeTaskLog("pve1", "UPID:pve1:X")
	if err != nil {
		t.Fatalf("GetNodeTaskLog: %v", err)
	}

	if gotTotal != total {
		t.Errorf("total = %d, want %d", gotTotal, total)
	}
	if len(got) != taskLogPageSize {
		t.Fatalf("got %d lines, want %d", len(got), taskLogPageSize)
	}
	// The whole point: the last line of the task is present.
	if last := got[len(got)-1]; last.T != "TASK ERROR: job errors" {
		t.Errorf("last line = %q, want the closing task marker", last.T)
	}
	if first := got[0]; first.N != total-taskLogPageSize+1 {
		t.Errorf("first line n = %d, want %d (a trailing window)", first.N, total-taskLogPageSize+1)
	}
	// A cheap probe for `total`, then the trailing page.
	if len(queries) != 2 || queries[0] != "start=0&limit=1" {
		t.Errorf("queries = %v, want a limit=1 probe then a tail fetch", queries)
	}
}

func TestGetNodeTaskLogShortLogIsNotTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"total":2,"data":[{"n":1,"t":"start"},{"n":2,"t":"TASK OK"}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	got, total, err := c.GetNodeTaskLog("pve1", "UPID:pve1:X")
	if err != nil {
		t.Fatalf("GetNodeTaskLog: %v", err)
	}
	if total != 2 || len(got) != 2 {
		t.Errorf("got %d lines / total %d, want 2 / 2", len(got), total)
	}
}

func TestGetNodeTaskStatusFinished(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"running", `{"data":{"status":"running"}}`, false},
		{"stopped", `{"data":{"status":"stopped","exitstatus":"job errors"}}`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			st, err := New(srv.URL, "u!t", "s", false).GetNodeTaskStatus("pve1", "UPID:pve1:X")
			if err != nil {
				t.Fatalf("GetNodeTaskStatus: %v", err)
			}
			if st.Finished() != tc.want {
				t.Errorf("Finished() = %v, want %v (status %q)", st.Finished(), tc.want, st.Status)
			}
		})
	}
}
