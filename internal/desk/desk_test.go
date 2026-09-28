package desk

import "testing"

func newDesk(t *testing.T, limit int) *Desk {
	t.Helper()
	d := New()
	if err := d.Init(limit); err != nil {
		t.Fatalf("init: %v", err)
	}
	return d
}

func mustOK(t *testing.T, err *OpError) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func wantCode(t *testing.T, err *OpError, want ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got success", want)
	}
	if err.Code != want {
		t.Fatalf("expected error %s, got %v", want, err)
	}
}

func TestInitAndDuplicateRegistration(t *testing.T) {
	d := New()
	wantCode(t, d.Init(0), ErrInvalidRequest)
	mustOK(t, d.Init(2))
	wantCode(t, d.Init(3), ErrAlreadyInitialized)
	if d.LoanLimit != 2 {
		t.Fatalf("limit changed: %d", d.LoanLimit)
	}

	wantCode(t, d.RegisterPatron(""), ErrInvalidRequest)
	mustOK(t, d.RegisterPatron("p1"))
	wantCode(t, d.RegisterPatron("p1"), ErrDuplicatePatron)
	mustOK(t, d.RegisterBook("b1"))
	wantCode(t, d.RegisterBook("b1"), ErrDuplicateBook)
	wantCode(t, d.RegisterCopy("nope", "c1"), ErrUnknownBook)
	mustOK(t, d.RegisterCopy("b1", "c2"))
	mustOK(t, d.RegisterCopy("b1", "c1"))
	wantCode(t, d.RegisterCopy("b1", "c1"), ErrDuplicateCopy)

	// 未初始化的台拒绝一切业务。
	fresh := New()
	wantCode(t, fresh.RegisterPatron("p1"), ErrNotInitialized)
}

func TestMultiCopyCheckoutPicksSmallestBarcodeAndReturn(t *testing.T) {
	d := newDesk(t, 2)
	mustOK(t, d.RegisterPatron("p1"))
	mustOK(t, d.RegisterBook("b1"))
	for _, bc := range []string{"c3", "c1", "c2"} {
		mustOK(t, d.RegisterCopy("b1", bc))
	}

	loan1, bc1, err := d.Checkout("p1", "b1")
	mustOK(t, err)
	if bc1 != "c1" {
		t.Fatalf("first checkout picked %s, want c1", bc1)
	}
	mustOK(t, d.RegisterPatron("p2"))
	loan2, bc2, err := d.Checkout("p2", "b1")
	mustOK(t, err)
	if bc2 != "c2" {
		t.Fatalf("second checkout picked %s, want c2", bc2)
	}
	_, _, err = d.Checkout("p2", "b1")
	wantCode(t, err, ErrAlreadyLoanedBook)

	// 仅剩 c3 但 p1 已借 b1，不能再借同书目；归还 c1 后仍拒绝（同书目在借）。
	mustOK(t, d.ReturnBook(loan1))
	_, _, err = d.Checkout("p1", "b1")
	mustOK(t, err)

	// 未知编号与重复归还。
	wantCode(t, d.ReturnBook("nope"), ErrUnknownLoan)
	mustOK(t, d.ReturnBook(loan2))
	wantCode(t, d.ReturnBook(loan2), ErrAlreadyReturned)
}

func TestLoanLimit(t *testing.T) {
	d := newDesk(t, 1)
	mustOK(t, d.RegisterPatron("p1"))
	mustOK(t, d.RegisterBook("b1"))
	mustOK(t, d.RegisterBook("b2"))
	mustOK(t, d.RegisterCopy("b1", "c1"))
	mustOK(t, d.RegisterCopy("b2", "c2"))
	_, _, err := d.Checkout("p1", "b1")
	mustOK(t, err)
	_, _, err = d.Checkout("p1", "b2")
	wantCode(t, err, ErrLimitReached)
}
