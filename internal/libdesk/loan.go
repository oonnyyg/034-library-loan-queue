package libdesk

import "sort"

// Reserve 为读者在书目候补队尾加入预约。
func (d *Desk) Reserve(readerID, titleID string) (int64, error) {
	if err := d.requireInit(); err != nil {
		return 0, err
	}
	if !d.State.Readers[readerID] {
		return 0, opError("unknown_reader", "reader %q is not registered", readerID)
	}
	if !d.State.Titles[titleID] {
		return 0, opError("unknown_title", "title %q is not registered", titleID)
	}
	if _, ok := d.State.Reservations[titleID][readerID]; ok {
		return 0, opError("duplicate_reservation", "reader %q already holds a reservation for title %q", readerID, titleID)
	}
	if d.readerHasTitle(readerID, titleID) {
		return 0, opError("already_holds_title", "reader %q currently borrows a copy of title %q", readerID, titleID)
	}
	seq := d.State.NextResvSeq
	d.State.NextResvSeq++
	d.State.Queues[titleID] = append(d.State.Queues[titleID], readerID)
	d.State.Reservations[titleID][readerID] = seq
	return seq, nil
}

// CancelReservation 取消读者对书目的预约；取消后重新预约排到队尾。
func (d *Desk) CancelReservation(readerID, titleID string) error {
	if err := d.requireInit(); err != nil {
		return err
	}
	if !d.State.Readers[readerID] {
		return opError("unknown_reader", "reader %q is not registered", readerID)
	}
	if !d.State.Titles[titleID] {
		return opError("unknown_title", "title %q is not registered", titleID)
	}
	resv := d.State.Reservations[titleID]
	if _, ok := resv[readerID]; !ok {
		return opError("no_reservation", "reader %q has no reservation for title %q", readerID, titleID)
	}
	queue := d.State.Queues[titleID]
	for i, r := range queue {
		if r == readerID {
			d.State.Queues[titleID] = append(queue[:i], queue[i+1:]...)
			break
		}
	}
	delete(resv, readerID)
	return nil
}

// Borrow 按规则借出一册：选条码字典序最小的可借册。
func (d *Desk) Borrow(readerID, titleID string) (loanID int64, barcode string, err error) {
	if err := d.requireInit(); err != nil {
		return 0, "", err
	}
	if !d.State.Readers[readerID] {
		return 0, "", opError("unknown_reader", "reader %q is not registered", readerID)
	}
	if !d.State.Titles[titleID] {
		return 0, "", opError("unknown_title", "title %q is not registered", titleID)
	}
	queue := d.State.Queues[titleID]
	if len(queue) > 0 && queue[0] != readerID {
		return 0, "", opError("queue_head_only", "title %q has a waiting queue; only reader %q may borrow it now", titleID, queue[0])
	}
	if d.readerActiveLoanCount(readerID) >= d.State.LoanLimit {
		return 0, "", opError("loan_limit_reached", "reader %q has reached the loan limit of %d", readerID, d.State.LoanLimit)
	}
	if d.readerHasTitle(readerID, titleID) {
		return 0, "", opError("already_holds_title", "reader %q already borrows a copy of title %q", readerID, titleID)
	}
	barcode = d.smallestAvailableCopy(titleID)
	if barcode == "" {
		return 0, "", opError("no_copy_available", "no available copy of title %q", titleID)
	}
	id := d.State.NextLoanID
	d.State.NextLoanID++
	d.State.Loans[id] = &LoanRecord{
		LoanID:      id,
		ReaderID:    readerID,
		TitleID:     titleID,
		CopyBarcode: barcode,
		Returned:    false,
	}
	d.State.CopyStatus[barcode] = true
	// 队首（必为该读者）预约随借阅移除。
	if len(queue) > 0 && queue[0] == readerID {
		d.State.Queues[titleID] = append([]string{}, queue[1:]...)
		delete(d.State.Reservations[titleID], readerID)
	}
	return id, barcode, nil
}

// Return 按借阅编号归还原册；重复归还与未知编号被拒绝。
func (d *Desk) Return(loanID int64) (string, error) {
	if err := d.requireInit(); err != nil {
		return "", err
	}
	rec, ok := d.State.Loans[loanID]
	if !ok {
		return "", opError("unknown_loan", "loan id %d does not exist", loanID)
	}
	if rec.Returned {
		return "", opError("already_returned", "loan id %d has already been returned", loanID)
	}
	rec.Returned = true
	d.State.CopyStatus[rec.CopyBarcode] = false
	return rec.CopyBarcode, nil
}

func (d *Desk) readerActiveLoanCount(readerID string) int {
	n := 0
	for _, rec := range d.State.Loans {
		if rec.ReaderID == readerID && !rec.Returned {
			n++
		}
	}
	return n
}

func (d *Desk) readerHasTitle(readerID, titleID string) bool {
	for _, rec := range d.State.Loans {
		if rec.ReaderID == readerID && rec.TitleID == titleID && !rec.Returned {
			return true
		}
	}
	return false
}

func (d *Desk) smallestAvailableCopy(titleID string) string {
	barcodes := make([]string, 0)
	for barcode, t := range d.State.Copies {
		if t == titleID && !d.State.CopyStatus[barcode] {
			barcodes = append(barcodes, barcode)
		}
	}
	if len(barcodes) == 0 {
		return ""
	}
	sort.Strings(barcodes)
	return barcodes[0]
}
