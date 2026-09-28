package desk

import (
	"reflect"
	"testing"
)

func setupQueueDesk(t *testing.T) *Desk {
	t.Helper()
	d := newDesk(t, 5)
	mustOK(t, d.RegisterPatron("p1"))
	mustOK(t, d.RegisterPatron("p2"))
	mustOK(t, d.RegisterPatron("p3"))
	mustOK(t, d.RegisterBook("b1"))
	mustOK(t, d.RegisterCopy("b1", "c1"))
	return d
}

func TestHoldRules(t *testing.T) {
	d := setupQueueDesk(t)

	// 已借该书目时不能预约。
	_, _, err := d.Checkout("p1", "b1")
	mustOK(t, err)
	wantCode(t, d.PlaceHold("p1", "b1"), ErrHoldWhileLoaning)
	wantCode(t, d.PlaceHold("p1", "b1"), ErrHoldWhileLoaning)

	// p2、p3 排队，重复预约拒绝；预约不占额度（另一本书仍可借）。
	mustOK(t, d.RegisterBook("b2"))
	mustOK(t, d.RegisterCopy("b2", "x1"))
	mustOK(t, d.PlaceHold("p2", "b1"))
	mustOK(t, d.PlaceHold("p3", "b1"))
	wantCode(t, d.PlaceHold("p2", "b1"), ErrHoldExists)
	_, _, err = d.Checkout("p2", "b2")
	mustOK(t, err)

	view, qerr := d.QueryBook("b1")
	mustOK(t, qerr)
	if !reflect.DeepEqual(view.HoldQueue, []string{"p2", "p3"}) {
		t.Fatalf("queue = %v", view.HoldQueue)
	}

	// 可借册未归还，借阅拒绝：不跳过队首，也不自动借给下一人。
	_, _, err = d.Checkout("p2", "b1")
	wantCode(t, err, ErrNoCopyAvailable)

	// 取消 p2 后 p3 成为队首；p2 重新预约排到队尾。
	mustOK(t, d.CancelHold("p2", "b1"))
	wantCode(t, d.CancelHold("p2", "b1"), ErrInvalidRequest)
	mustOK(t, d.PlaceHold("p2", "b1"))
	view, _ = d.QueryBook("b1")
	if !reflect.DeepEqual(view.HoldQueue, []string{"p3", "p2"}) {
		t.Fatalf("queue after rehold = %v", view.HoldQueue)
	}
}

func TestHeadOfQueueBorrowsAndHoldRemoved(t *testing.T) {
	d := setupQueueDesk(t)
	mustOK(t, d.PlaceHold("p2", "b1"))
	mustOK(t, d.PlaceHold("p3", "b1"))

	// 有候补时非队首不能借；p1 无预约也不能借。
	_, _, err := d.Checkout("p3", "b1")
	wantCode(t, err, ErrNotHeadOfQueue)
	_, _, err = d.Checkout("p1", "b1")
	wantCode(t, err, ErrNotHeadOfQueue)

	loanID, bc, err := d.Checkout("p2", "b1")
	mustOK(t, err)
	if bc != "c1" || loanID == "" {
		t.Fatalf("checkout returned %s %s", loanID, bc)
	}
	// 成功后移除队首预约，p3 变成新队首。
	view, _ := d.QueryBook("b1")
	if !reflect.DeepEqual(view.HoldQueue, []string{"p3"}) {
		t.Fatalf("queue after head checkout = %v", view.HoldQueue)
	}

	// 借阅编号唯一且稳定排序。
	mustOK(t, d.ReturnBook(loanID))
	loanID2, _, err := d.Checkout("p3", "b1")
	mustOK(t, err)
	if loanID2 <= loanID {
		t.Fatalf("loan ids not increasing: %s then %s", loanID, loanID2)
	}
}

func TestNoCopyAvailableWithoutHolds(t *testing.T) {
	d := setupQueueDesk(t)
	_, _, err := d.Checkout("p1", "b1")
	mustOK(t, err)
	_, _, err = d.Checkout("p2", "b1")
	wantCode(t, err, ErrNoCopyAvailable)
}

func TestFailuresDoNotMutateState(t *testing.T) {
	d := setupQueueDesk(t)
	mustOK(t, d.PlaceHold("p2", "b1"))

	snapshot := snapshotState(d)
	failures := []func() *OpError{
		func() *OpError { return d.RegisterPatron("p1") },
		func() *OpError { return d.RegisterBook("b1") },
		func() *OpError { return d.RegisterCopy("b1", "c1") },
		func() *OpError { return d.PlaceHold("p2", "b1") },
		func() *OpError { return d.PlaceHold("p1", "nope") },
		func() *OpError { return d.CancelHold("p3", "b1") },
		func() *OpError { return d.ReturnBook("ghost") },
	}
	for i, f := range failures {
		if f() == nil {
			t.Fatalf("failure case %d succeeded", i)
		}
		got := snapshotState(d)
		if !reflect.DeepEqual(got, snapshot) {
			t.Fatalf("state mutated after failure case %d:\nbefore=%#v\nafter =%#v", i, snapshot, got)
		}
	}
	if _, _, err := d.Checkout("p3", "b1"); err == nil {
		t.Fatal("expected checkout failure")
	}
	if got := snapshotState(d); !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("state mutated after failed checkout:\n%#v", got)
	}
}
