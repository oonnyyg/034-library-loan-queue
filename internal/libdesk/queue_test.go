package libdesk

import (
	"strings"
	"testing"
)

func TestReserveRulesAndQueueOrder(t *testing.T) {
	d := newTestDesk(t, 2)
	mustReg(t, d, "r1", "b1", "c1")
	mustRegReader(t, d, "r2")
	mustRegReader(t, d, "r3")

	// r1 借出唯一一册。
	if _, _, err := d.Borrow("r1", "b1"); err != nil {
		t.Fatal(err)
	}
	// 已借该书目时不能预约。
	if _, err := d.Reserve("r1", "b1"); errCode(err) != "already_holds_title" {
		t.Fatalf("want already_holds_title, got %v", err)
	}
	if _, err := d.Reserve("r2", "b1"); err != nil {
		t.Fatal(err)
	}
	// 同一读者对同一书目只能有一个预约。
	if _, err := d.Reserve("r2", "b1"); errCode(err) != "duplicate_reservation" {
		t.Fatalf("want duplicate_reservation, got %v", err)
	}
	if _, err := d.Reserve("r3", "b1"); err != nil {
		t.Fatal(err)
	}

	st, _ := d.QueryTitle("b1")
	if got := joinQueue(st.WaitQueue); got != "r2,r3" {
		t.Fatalf("queue = %s", got)
	}

	// r1 归还后，队首之外的人不能借，也不能跳过队首。
	loans, _ := d.QueryReader("r1")
	if err := firstReturn(d, loans.ActiveLoans); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.Borrow("r3", "b1"); errCode(err) != "queue_head_only" {
		t.Fatalf("want queue_head_only, got %v", err)
	}
	// 失败不变更：队首仍是 r2，r3 仍在队列中。
	st, _ = d.QueryTitle("b1")
	if got := joinQueue(st.WaitQueue); got != "r2,r3" {
		t.Fatalf("queue changed after failed borrow: %s", got)
	}
	if st.Copies[0].Out {
		t.Fatal("failed borrow must not lend the returned copy")
	}

	// 队首借阅成功后其预约被移除，r3 升为队首。
	if _, bc, err := d.Borrow("r2", "b1"); err != nil || bc != "c1" {
		t.Fatalf("head borrow = %q, %v", bc, err)
	}
	st, _ = d.QueryTitle("b1")
	if got := joinQueue(st.WaitQueue); got != "r3" {
		t.Fatalf("queue after head borrow = %s", got)
	}
}

func TestCancelReservationRequeuesAtTail(t *testing.T) {
	d := newTestDesk(t, 2)
	mustReg(t, d, "r1", "b1", "c1")
	for _, r := range []string{"r2", "r3", "r4"} {
		mustRegReader(t, d, r)
	}
	if _, _, err := d.Borrow("r1", "b1"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"r2", "r3", "r4"} {
		if _, err := d.Reserve(r, "b1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.CancelReservation("r2", "b1"); err != nil {
		t.Fatal(err)
	}
	// 取消一个不存在的预约应明确拒绝。
	if err := d.CancelReservation("r2", "b1"); errCode(err) != "no_reservation" {
		t.Fatalf("want no_reservation, got %v", err)
	}
	// 重新预约排到队尾。
	if _, err := d.Reserve("r2", "b1"); err != nil {
		t.Fatal(err)
	}
	st, _ := d.QueryTitle("b1")
	if got := joinQueue(st.WaitQueue); got != "r3,r4,r2" {
		t.Fatalf("queue = %s", got)
	}
}

func TestBorrowWithoutAvailableCopyRejected(t *testing.T) {
	d := newTestDesk(t, 5)
	mustReg(t, d, "r1", "b1", "c1")
	mustRegReader(t, d, "r2")
	if _, _, err := d.Borrow("r1", "b1"); err != nil {
		t.Fatal(err)
	}
	// 无候补队列时无可借册也要拒绝，且不能自动借给下一人。
	if _, _, err := d.Borrow("r2", "b1"); errCode(err) != "no_copy_available" {
		t.Fatalf("want no_copy_available, got %v", err)
	}
}

func TestFailedOperationMutatesNothing(t *testing.T) {
	d := newTestDesk(t, 1)
	mustReg(t, d, "r1", "b1", "c1")
	mustRegReader(t, d, "r2")
	if _, _, err := d.Borrow("r1", "b1"); err != nil {
		t.Fatal(err)
	}
	before := d.State.NextLoanID
	// 未知读者借阅失败，借阅编号游标不变。
	if _, _, err := d.Borrow("ghost", "b1"); errCode(err) != "unknown_reader" {
		t.Fatalf("want unknown_reader, got %v", err)
	}
	if d.State.NextLoanID != before {
		t.Fatal("loan id cursor advanced on failed borrow")
	}
	// 未初始化的新台拒绝一切业务操作。
	fresh := &Desk{dir: t.TempDir(), State: newState()}
	if err := fresh.RegisterReader("r9"); errCode(err) != "not_initialized" {
		t.Fatalf("want not_initialized, got %v", err)
	}
}

func joinQueue(xs []string) string { return strings.Join(xs, ",") }

func firstReturn(d *Desk, loans []LoanRecord) error {
	if len(loans) == 0 {
		return nil
	}
	_, err := d.Return(loans[0].LoanID)
	return err
}
