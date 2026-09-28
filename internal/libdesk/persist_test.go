package libdesk

import "testing"

func TestSaveAndReopen(t *testing.T) {
	dir := t.TempDir()
	d, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Init(2); err != nil {
		t.Fatal(err)
	}
	mustReg(t, d, "r1", "b1", "c2", "c1")
	mustRegReader(t, d, "r2")
	id, _, err := d.Borrow("r1", "b1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Reserve("r2", "b1"); err != nil {
		t.Fatal(err)
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}

	reopened, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State.LoanLimit != 2 || !reopened.State.Initialized {
		t.Fatal("init state not persisted")
	}
	// 重复归还在重开后仍被拒绝，借阅记录保留历史。
	if _, err := reopened.Return(id); err != nil {
		t.Fatalf("return after reopen: %v", err)
	}
	if _, err := reopened.Return(id); errCode(err) != "already_returned" {
		t.Fatalf("want already_returned after reopen, got %v", err)
	}
	// 候补顺序保留；c1 归还后队首 r2 可借。
	st, _ := reopened.QueryTitle("b1")
	if got := joinQueue(st.WaitQueue); got != "r2" {
		t.Fatalf("queue after reopen = %v", st.WaitQueue)
	}
	if _, bc, err := reopened.Borrow("r2", "b1"); err != nil || bc != "c1" {
		t.Fatalf("head borrow after reopen = %q, %v", bc, err)
	}

	r1, _ := reopened.QueryReader("r1")
	if len(r1.History) != 1 || !r1.History[0].Returned {
		t.Fatalf("reader history not preserved: %+v", r1.History)
	}
	// 新借阅编号在已有编号之后，保证唯一。
	r2st, _ := reopened.QueryReader("r2")
	if len(r2st.ActiveLoans) != 1 || r2st.ActiveLoans[0].LoanID <= id {
		t.Fatalf("new loan id must exceed prior max: %+v", r2st.ActiveLoans)
	}
}

func TestQueryStableOrdering(t *testing.T) {
	d := newTestDesk(t, 5)
	mustReg(t, d, "r1", "b1", "c-z", "c-a", "c-m")
	mustRegReader(t, d, "r2")
	if _, _, err := d.Borrow("r2", "b1"); err != nil {
		t.Fatal(err)
	}
	st, err := d.QueryTitle("b1")
	if err != nil {
		t.Fatal(err)
	}
	if st.Copies[0].Barcode != "c-a" || !st.Copies[0].Out {
		t.Fatalf("copies not sorted by barcode or status wrong: %+v", st.Copies)
	}
	if st.Copies[0].LoanID == 0 {
		t.Fatal("out copy should carry loan id")
	}

	mustReg(t, d, "r1", "b2", "d1")
	mustReg(t, d, "r1", "b3", "e1")
	if _, _, err := d.Borrow("r1", "b2"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.Borrow("r1", "b3"); err != nil {
		t.Fatal(err)
	}
	r1, _ := d.QueryReader("r1")
	if len(r1.ActiveLoans) != 2 || r1.ActiveLoans[0].LoanID > r1.ActiveLoans[1].LoanID {
		t.Fatalf("active loans not id-sorted: %+v", r1.ActiveLoans)
	}
	if _, err := d.Reserve("r1", "b1"); err != nil {
		t.Fatal(err)
	}
	r1, _ = d.QueryReader("r1")
	if len(r1.Reservations) != 1 || r1.Reservations[0].TitleID != "b1" {
		t.Fatalf("reservations wrong: %+v", r1.Reservations)
	}
}
