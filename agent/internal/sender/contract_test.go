package sender

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"testing"
	"time"
)

// updateGolden regenerates the shared protocol golden fixture instead of
// asserting against it. Run after an intentional protocol change:
//
//	go test ./internal/sender -run TestReportContractGolden -update
var updateGolden = flag.Bool("update", false, "regenerate the protocol golden fixture")

// goldenPath is the single source-of-truth contract fixture, shared with the
// server module's decode test (server/internal/handlers/agent_contract_test.go).
// Path is relative to this package directory (agent/internal/sender).
const goldenPath = "../../../protocol/agent_report.golden.json"

// fixedContractTime is an arbitrary but stable timestamp so the golden is
// deterministic across runs and machines.
var fixedContractTime = time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

// TestReportContractGolden marshals a fully-populated Report and compares it to
// the committed golden fixture. The Report is filled by reflection so EVERY
// field the agent can emit appears in the golden — when a new field is added to
// any nested collector type, this test forces the golden to be regenerated, and
// the server-side decode test then proves the server knows the new field too.
func TestReportContractGolden(t *testing.T) {
	var report Report
	fillValue(reflect.ValueOf(&report).Elem())

	got, err := json.MarshalIndent(&report, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	got = append(got, '\n')

	if *updateGolden {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", goldenPath, err)
		}
		t.Logf("golden regenerated: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s (regenerate with `go test ./internal/sender -run TestReportContractGolden -update`): %v", goldenPath, err)
	}

	if !bytes.Equal(normalizeNL(want), normalizeNL(got)) {
		t.Errorf("agent Report JSON shape drifted from the protocol golden.\n"+
			"Regenerate with: go test ./internal/sender -run TestReportContractGolden -update\n"+
			"--- got ---\n%s", got)
	}
}

// normalizeNL strips carriage returns so the byte comparison is stable on
// Windows checkouts regardless of git autocrlf settings.
func normalizeNL(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

// fillValue recursively populates v with deterministic, non-zero values so that
// every exported field (including omitempty ones) is emitted when marshaled.
// time.Time is special-cased to a fixed instant; all other structs recurse.
func fillValue(v reflect.Value) {
	if v.Type() == reflect.TypeOf(time.Time{}) {
		v.Set(reflect.ValueOf(fixedContractTime))
		return
	}

	switch v.Kind() {
	case reflect.Pointer:
		p := reflect.New(v.Type().Elem())
		fillValue(p.Elem())
		v.Set(p)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).PkgPath != "" {
				continue // unexported field — not part of the wire contract
			}
			fillValue(v.Field(i))
		}
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 1, 1)
		fillValue(s.Index(0))
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		fillValue(key)
		val := reflect.New(v.Type().Elem()).Elem()
		fillValue(val)
		m.SetMapIndex(key, val)
		v.Set(m)
	case reflect.String:
		v.SetString("contract")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(7)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.5)
	}
}
