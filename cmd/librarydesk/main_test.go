package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"example.com/library-loan-queue/internal/libdesk"
)

func runLines(t *testing.T, dir, in string) []response {
	t.Helper()
	desk, err := libdesk.LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(desk, strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	var resps []response
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var r response
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		resps = append(resps, r)
	}
	return resps
}

func TestCLISession(t *testing.T) {
	dir := t.TempDir()
	session := strings.Join([]string{
		`{"op":"init","loan_limit":2}`,
		`{"op":"register_reader","reader_id":"r1"}`,
		`{"op":"register_reader","reader_id":"r1"}`,
		`{"op":"register_title","title_id":"b1"}`,
		`{"op":"register_copy","copy_barcode":"c1","title_id":"b1"}`,
		`{"op":"borrow","reader_id":"r1","title_id":"b1"}`,
		`{"op":"borrow","reader_id":"r1","title_id":"b1"}`,
		`{"op":"query_reader","reader_id":"r1"}`,
		`{"op":"return","loan_id":1}`,
		`{"op":"return","loan_id":1}`,
		`{"op":"frobnicate"}`,
		`{bad json`,
	}, "\n")

	resps := runLines(t, dir, session)
	if len(resps) != 12 {
		t.Fatalf("got %d responses", len(resps))
	}
	wantOK := []bool{true, true, false, true, true, true, false, true, true, false}
	for i, ok := range wantOK {
		if resps[i].OK != ok {
			t.Fatalf("resp %d ok=%v err=%+v", i, resps[i].OK, resps[i].Error)
		}
	}
	codes := []string{
		"", "", "duplicate_reader", "", "", "",
		"already_holds_title", "", "", "already_returned",
	}
	for i, code := range codes {
		if code != "" && (resps[i].Error == nil || resps[i].Error.Code != code) {
			t.Fatalf("resp %d want %s, got %+v", i, code, resps[i].Error)
		}
	}
	if resps[5].Result["loan_id"].(float64) != 1 {
		t.Fatalf("borrow result = %+v", resps[5].Result)
	}
	if resps[10].Error.Code != "unknown_op" || resps[11].Error.Code != "bad_json" {
		t.Fatalf("last errors = %+v %+v", resps[10].Error, resps[11].Error)
	}

	// 重新打开同一数据目录，历史仍在。
	reopened := runLines(t, dir, `{"op":"query_reader","reader_id":"r1"}`+"\n")
	if !reopened[0].OK {
		t.Fatalf("reopen query failed: %+v", reopened[0].Error)
	}
	reader := reopened[0].Result["reader"].(map[string]any)
	hist := reader["history"].([]any)
	if len(hist) != 1 {
		t.Fatalf("history after reopen = %v", hist)
	}
}
