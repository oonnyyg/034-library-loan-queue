package desk

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type stateSnapshot struct {
	LoanLimit int
	Patrons   []string
	Books     []string
	Copies    map[string]*CopyState
	Holds     map[string][]string
	Loans     map[string]*Loan
	Seq       int
}

func snapshotState(d *Desk) stateSnapshot {
	s := stateSnapshot{
		LoanLimit: d.LoanLimit,
		Copies:    map[string]*CopyState{},
		Holds:     map[string][]string{},
		Loans:     map[string]*Loan{},
		Seq:       d.LoanSeq,
	}
	for k := range d.Patrons {
		s.Patrons = append(s.Patrons, k)
	}
	for k := range d.Books {
		s.Books = append(s.Books, k)
	}
	sort.Strings(s.Patrons)
	sort.Strings(s.Books)
	for k, v := range d.Copies {
		cp := *v
		s.Copies[k] = &cp
	}
	for k, v := range d.Holds {
		s.Holds[k] = append([]string{}, v...)
	}
	for k, v := range d.Loans {
		cp := *v
		s.Loans[k] = &cp
	}
	return s
}

func TestQueryViewsAndSortedOutput(t *testing.T) {
	d := newDesk(t, 5)
	mustOK(t, d.RegisterPatron("p1"))
	mustOK(t, d.RegisterBook("b1"))
	mustOK(t, d.RegisterCopy("b1", "c2"))
	mustOK(t, d.RegisterCopy("b1", "c1"))

	loan1, _, err := d.Checkout("p1", "b1")
	mustOK(t, err)
	mustOK(t, d.ReturnBook(loan1))
	loan2, _, err := d.Checkout("p1", "b1")
	mustOK(t, err)

	book, qerr := d.QueryBook("b1")
	mustOK(t, qerr)
	if len(book.Copies) != 2 || book.Copies[0].Barcode != "c1" || book.Copies[1].Barcode != "c2" {
		t.Fatalf("copies not sorted: %+v", book.Copies)
	}
	if !book.Copies[0].OnLoan || book.Copies[0].LoanID != loan2 {
		t.Fatalf("c1 should be out on %s: %+v", loan2, book.Copies[0])
	}
	if book.Copies[1].OnLoan {
		t.Fatalf("c2 should be on shelf: %+v", book.Copies[1])
	}

	patron, qerr := d.QueryPatron("p1")
	mustOK(t, qerr)
	if len(patron.ActiveLoans) != 1 || patron.ActiveLoans[0].ID != loan2 {
		t.Fatalf("active loans wrong: %+v", patron.ActiveLoans)
	}
	if len(patron.History) != 1 || patron.History[0].ID != loan1 {
		t.Fatalf("history wrong: %+v", patron.History)
	}

	wantCode(t, func() *OpError { _, e := d.QueryBook("x"); return e }(), ErrUnknownBook)
	wantCode(t, func() *OpError { _, e := d.QueryPatron("x"); return e }(), ErrUnknownPatron)
}

func TestPersistAndReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")

	d, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	mustOK(t, d.Init(2))
	mustOK(t, d.RegisterPatron("p1"))
	mustOK(t, d.RegisterPatron("p2"))
	mustOK(t, d.RegisterBook("b1"))
	mustOK(t, d.RegisterCopy("b1", "c1"))
	mustOK(t, d.PlaceHold("p1", "b1"))
	mustOK(t, d.PlaceHold("p2", "b1"))
	loanID, _, checkoutErr := d.Checkout("p1", "b1")
	if checkoutErr != nil {
		t.Fatal(checkoutErr)
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}

	d2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d2.LoanLimit != 2 {
		t.Fatalf("limit lost: %d", d2.LoanLimit)
	}
	view, qerr := d2.QueryBook("b1")
	mustOK(t, qerr)
	if !reflect.DeepEqual(view.HoldQueue, []string{"p2"}) {
		t.Fatalf("queue lost: %v", view.HoldQueue)
	}
	if len(view.Copies) != 1 || !view.Copies[0].OnLoan || view.Copies[0].LoanID != loanID {
		t.Fatalf("copy state lost: %+v", view.Copies)
	}

	// 历史保留、重复归还仍被拒绝；借阅编号继续递增且唯一。
	mustOK(t, d2.ReturnBook(loanID))
	wantCode(t, d2.ReturnBook(loanID), ErrAlreadyReturned)
	newLoan, _, cerr := d2.Checkout("p2", "b1")
	if cerr != nil {
		t.Fatal(cerr)
	}
	if newLoan == loanID {
		t.Fatalf("loan id reused: %s", newLoan)
	}
	patron, _ := d2.QueryPatron("p1")
	if len(patron.History) != 1 || patron.History[0].ID != loanID {
		t.Fatalf("history lost after reopen: %+v", patron.History)
	}
}

func TestProtocolResponses(t *testing.T) {
	d := newDesk(t, 1)

	handle := func(s string) Response {
		return d.Handle([]byte(s))
	}
	resp := handle(`{"op":"registerPatron","patronId":"p1"}`)
	if !resp.OK {
		t.Fatalf("register failed: %+v", resp)
	}
	resp = handle(`{"op":"registerPatron","patronId":"p1"}`)
	if resp.OK || resp.ErrorCode != string(ErrDuplicatePatron) {
		t.Fatalf("duplicate response wrong: %+v", resp)
	}
	resp = handle(`{not json`)
	if resp.OK || resp.ErrorCode != string(ErrInvalidRequest) {
		t.Fatalf("bad json response wrong: %+v", resp)
	}
	resp = handle(`{"op":"frobnicate"}`)
	if resp.OK || resp.ErrorCode != string(ErrInvalidRequest) {
		t.Fatalf("unknown op response wrong: %+v", resp)
	}

	mustOK(t, d.RegisterBook("b1"))
	mustOK(t, d.RegisterCopy("b1", "c1"))
	resp = handle(`{"op":"checkout","patronId":"p1","bookId":"b1"}`)
	if !resp.OK {
		t.Fatalf("checkout failed: %+v", resp)
	}
	data, _ := json.Marshal(resp.Data)
	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil || got["loanId"] == "" || got["barcode"] != "c1" {
		t.Fatalf("checkout data wrong: %s", data)
	}
}
