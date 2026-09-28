package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/library-loan-queue/internal/libdesk"
)

type request struct {
	Op          string `json:"op"`
	LoanLimit   int    `json:"loan_limit,omitempty"`
	ReaderID    string `json:"reader_id,omitempty"`
	TitleID     string `json:"title_id,omitempty"`
	CopyBarcode string `json:"copy_barcode,omitempty"`
	LoanID      int64  `json:"loan_id,omitempty"`
}

type response struct {
	OK     bool             `json:"ok"`
	Result map[string]any   `json:"result,omitempty"`
	Error  *libdesk.OpError `json:"error,omitempty"`
}

func main() {
	stateDir := flag.String("state", "", "directory for persistent state data")
	flag.Parse()
	if *stateDir == "" {
		fmt.Fprintln(os.Stderr, "missing required --state directory")
		os.Exit(2)
	}
	desk, err := libdesk.LoadOrCreate(*stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := run(desk, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(desk *libdesk.Desk, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	writer := bufio.NewWriter(out)
	defer writer.Flush()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(writer, response{Error: &libdesk.OpError{Code: "bad_json", Message: err.Error()}})
			continue
		}
		resp := handle(desk, req)
		if err := writeResponse(writer, resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func writeResponse(w *bufio.Writer, resp response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

func handle(desk *libdesk.Desk, req request) response {
	var opErr error
	result := map[string]any{}

	switch req.Op {
	case "init":
		opErr = desk.Init(req.LoanLimit)
	case "register_reader":
		opErr = desk.RegisterReader(req.ReaderID)
		result["reader_id"] = req.ReaderID
	case "register_title":
		opErr = desk.RegisterTitle(req.TitleID)
		result["title_id"] = req.TitleID
	case "register_copy":
		opErr = desk.RegisterCopy(req.CopyBarcode, req.TitleID)
		result["copy_barcode"] = req.CopyBarcode
		result["title_id"] = req.TitleID
	case "reserve":
		seq, err := desk.Reserve(req.ReaderID, req.TitleID)
		opErr = err
		if err == nil {
			result["reader_id"] = req.ReaderID
			result["title_id"] = req.TitleID
			result["seq"] = seq
		}
	case "cancel_reservation":
		opErr = desk.CancelReservation(req.ReaderID, req.TitleID)
	case "borrow":
		loanID, barcode, err := desk.Borrow(req.ReaderID, req.TitleID)
		opErr = err
		if err == nil {
			result["loan_id"] = loanID
			result["copy_barcode"] = barcode
			result["reader_id"] = req.ReaderID
			result["title_id"] = req.TitleID
		}
	case "return":
		barcode, err := desk.Return(req.LoanID)
		opErr = err
		if err == nil {
			result["loan_id"] = req.LoanID
			result["copy_barcode"] = barcode
		}
	case "query_title":
		st, err := desk.QueryTitle(req.TitleID)
		opErr = err
		if err == nil {
			result["title"] = st
		}
	case "query_reader":
		st, err := desk.QueryReader(req.ReaderID)
		opErr = err
		if err == nil {
			result["reader"] = st
		}
	default:
		return response{Error: &libdesk.OpError{Code: "unknown_op", Message: fmt.Sprintf("unsupported op %q", req.Op)}}
	}

	var oe *libdesk.OpError
	if errors.As(opErr, &oe) {
		return response{Error: oe}
	}
	if opErr != nil {
		return response{Error: &libdesk.OpError{Code: "internal_error", Message: opErr.Error()}}
	}
	if err := desk.Save(); err != nil {
		return response{Error: &libdesk.OpError{Code: "persist_failed", Message: err.Error()}}
	}
	return response{OK: true, Result: result}
}
