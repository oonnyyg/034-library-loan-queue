package libdesk

import (
	"errors"
	"testing"
)

func newTestDesk(t *testing.T, limit int) *Desk {
	t.Helper()
	d := &Desk{dir: t.TempDir(), State: newState()}
	if err := d.Init(limit); err != nil {
		t.Fatalf("init: %v", err)
	}
	return d
}

func errCode(err error) string {
	var oe *OpError
	if errors.As(err, &oe) {
		return oe.Code
	}
	return ""
}

func TestInitRules(t *testing.T) {
	d := &Desk{dir: t.TempDir(), State: newState()}
	if err := d.Init(0); errCode(err) != "invalid_limit" {
		t.Fatalf("want invalid_limit, got %v", err)
	}
	if err := d.Init(-3); errCode(err) != "invalid_limit" {
		t.Fatalf("want invalid_limit, got %v", err)
	}
	if err := d.Init(2); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := d.Init(5); errCode(err) != "already_initialized" {
		t.Fatalf("want already_initialized, got %v", err)
	}
	if d.State.LoanLimit != 2 {
		t.Fatalf("limit changed to %d", d.State.LoanLimit)
	}
}

func TestDuplicateRegistration(t *testing.T) {
	d := newTestDesk(t, 2)
	if err := d.RegisterReader("r1"); err != nil {
		t.Fatal(err)
	}
	if err := d.RegisterReader("r1"); errCode(err) != "duplicate_reader" {
		t.Fatalf("want duplicate_reader, got %v", err)
	}
	if err := d.RegisterTitle("b1"); err != nil {
		t.Fatal(err)
	}
	if err := d.RegisterTitle("b1"); errCode(err) != "duplicate_title" {
		t.Fatalf("want duplicate_title, got %v", err)
	}
	if err := d.RegisterCopy("c1", "b1"); err != nil {
		t.Fatal(err)
	}
	if err := d.RegisterCopy("c1", "b1"); errCode(err) != "duplicate_copy" {
		t.Fatalf("want duplicate_copy, got %v", err)
	}
	if err := d.RegisterCopy("c2", "nope"); errCode(err) != "unknown_title" {
		t.Fatalf("want unknown_title, got %v", err)
	}
}

func TestMultiCopyBorrowPicksSmallestBarcode(t *testing.T) {
	d := newTestDesk(t, 5)
	mustReg(t, d, "r1", "b1", "c3", "c1", "c2")

	id1, bc1, err := d.Borrow("r1", "b1")
	if err != nil || bc1 != "c1" {
		t.Fatalf("first borrow = %q, %v", bc1, err)
	}
	// 同一读者不能同时借阅同一书目的多册。
	if _, _, err := d.Borrow("r1", "b1"); errCode(err) != "already_holds_title" {
		t.Fatalf("want already_holds_title, got %v", err)
	}
	mustRegReader(t, d, "r2")
	id2, bc2, err := d.Borrow("r2", "b1")
	if err != nil || bc2 != "c2" {
		t.Fatalf("second borrow = %q, %v", bc2, err)
	}
	if id1 == id2 {
		t.Fatal("loan ids must be unique")
	}
	if _, err := d.Return(id1); err != nil {
		t.Fatal(err)
	}
	// c1 归还后再次成为字典序最小的可借册。
	if _, bc, err := d.Borrow("r1", "b1"); err != nil || bc != "c1" {
		t.Fatalf("re-borrow = %q, %v", bc, err)
	}
}

func TestReturnRules(t *testing.T) {
	d := newTestDesk(t, 2)
	mustReg(t, d, "r1", "b1", "c1")
	id, _, err := d.Borrow("r1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Return(999); errCode(err) != "unknown_loan" {
		t.Fatalf("want unknown_loan, got %v", err)
	}
	if _, err := d.Return(id); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Return(id); errCode(err) != "already_returned" {
		t.Fatalf("want already_returned, got %v", err)
	}
}

func TestLoanLimit(t *testing.T) {
	d := newTestDesk(t, 2)
	mustReg(t, d, "r1", "b1", "c1")
	mustReg(t, d, "r1", "b2", "c2")
	mustReg(t, d, "r1", "b3", "c3")
	for _, title := range []string{"b1", "b2"} {
		if _, _, err := d.Borrow("r1", title); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := d.Borrow("r1", "b3"); errCode(err) != "loan_limit_reached" {
		t.Fatalf("want loan_limit_reached, got %v", err)
	}
	// 书目 b3 唯一的册未被借出，状态不应改变。
	st, err := d.QueryTitle("b3")
	if err != nil {
		t.Fatal(err)
	}
	if st.Copies[0].Out {
		t.Fatal("failed borrow must not mark copy as out")
	}
}

func mustReg(t *testing.T, d *Desk, reader, title string, copies ...string) {
	t.Helper()
	mustRegReader(t, d, reader)
	if err := d.RegisterTitle(title); err != nil && errCode(err) != "duplicate_title" {
		t.Fatal(err)
	}
	for _, bc := range copies {
		if err := d.RegisterCopy(bc, title); err != nil {
			t.Fatal(err)
		}
	}
}

func mustRegReader(t *testing.T, d *Desk, reader string) {
	t.Helper()
	if err := d.RegisterReader(reader); err != nil && errCode(err) != "duplicate_reader" {
		t.Fatal(err)
	}
}
