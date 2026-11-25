package netdiscovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const arpFixture = `IP address       HW type     Flags       HW address            Mask     Device
192.168.1.20     0x1         0x2         AA:BB:CC:00:11:22     *        eth0
192.168.1.3      0x1         0x2         bc:24:11:aa:bb:cc     *        eth0
192.168.1.99     0x1         0x0         00:00:00:00:00:00     *        eth0
192.168.1.98     0x1         0x2         00:00:00:00:00:00     *        eth0
192.168.1.3      0x1         0x2         bc:24:11:aa:bb:cc     *        eth0
10.10.0.3        0x1         0x6         52:54:00:12:34:56     *        vmbr1
not-an-ip        0x1         0x2         52:54:00:12:34:57     *        eth0
192.168.1.4      0x1         zz          52:54:00:12:34:58     *        eth0
192.168.1.5      0x1         0x2         nope                  *        eth0
short line
`

func TestParseProcNetARP(t *testing.T) {
	res, err := ParseProcNetARP(strings.NewReader(arpFixture))
	if err != nil {
		t.Fatal(err)
	}
	if res.Truncated {
		t.Error("unexpected truncation")
	}
	want := []ARPEntry{
		{IP: "10.10.0.3", MAC: "52:54:00:12:34:56", Interface: "vmbr1"},
		{IP: "192.168.1.3", MAC: "bc:24:11:aa:bb:cc", Interface: "eth0"},
		{IP: "192.168.1.20", MAC: "aa:bb:cc:00:11:22", Interface: "eth0"},
	}
	if len(res.Entries) != len(want) {
		t.Fatalf("entries = %+v, want %+v", res.Entries, want)
	}
	for i := range want {
		if res.Entries[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, res.Entries[i], want[i])
		}
	}
}

func TestParseProcNetARPEmptyTable(t *testing.T) {
	res, err := ParseProcNetARP(strings.NewReader("IP address HW type Flags HW address Mask Device\n"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Entries == nil || len(res.Entries) != 0 {
		t.Fatalf("expected a non-nil empty slice, got %#v", res.Entries)
	}
}

func TestParseProcNetARPTruncates(t *testing.T) {
	var b strings.Builder
	b.WriteString("header\n")
	for i := 0; i < maxARPEntries+5; i++ {
		fmt.Fprintf(&b, "10.%d.%d.%d 0x1 0x2 02:00:00:00:%02x:%02x * eth0\n", i/65536, (i/256)%256, i%256, (i/256)%256, i%256)
	}
	res, err := ParseProcNetARP(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || len(res.Entries) != maxARPEntries {
		t.Fatalf("truncated=%v len=%d", res.Truncated, len(res.Entries))
	}
}

func TestReadARPTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "arp")
	if err := os.WriteFile(path, []byte(arpFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	orig := procNetARP
	t.Cleanup(func() { procNetARP = orig })

	procNetARP = path
	res, err := ReadARPTable()
	if err != nil || len(res.Entries) != 3 {
		t.Fatalf("ReadARPTable = %+v, %v", res, err)
	}

	procNetARP = filepath.Join(dir, "missing")
	if _, err := ReadARPTable(); err == nil || !strings.Contains(err.Error(), "cannot read the ARP table") {
		t.Fatalf("missing file error = %v", err)
	}
}
