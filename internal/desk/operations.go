package desk

import "sort"

func (d *Desk) checkPatron(patronID string) *OpError {
	if !d.Patrons[patronID] {
		return opErr(ErrUnknownPatron, "patron %q is not registered", patronID)
	}
	return nil
}

func (d *Desk) checkBook(bookID string) *OpError {
	if !d.Books[bookID] {
		return opErr(ErrUnknownBook, "book %q is not registered", bookID)
	}
	return nil
}

func (d *Desk) holdsContains(bookID, patronID string) int {
	for i, id := range d.Holds[bookID] {
		if id == patronID {
			return i
		}
	}
	return -1
}

// activeLoans 返回读者当前在借的记录。
func (d *Desk) activeLoans(patronID string) []*Loan {
	var out []*Loan
	for _, l := range d.Loans {
		if l.PatronID == patronID && !l.Returned {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PlaceHold 为读者预约书目，排队到队尾。
func (d *Desk) PlaceHold(patronID, bookID string) *OpError {
	if err := d.requireInit(); err != nil {
		return err
	}
	if err := d.checkPatron(patronID); err != nil {
		return err
	}
	if err := d.checkBook(bookID); err != nil {
		return err
	}
	for _, l := range d.activeLoans(patronID) {
		if l.BookID == bookID {
			return opErr(ErrHoldWhileLoaning, "patron %q is currently loaning book %q", patronID, bookID)
		}
	}
	if d.holdsContains(bookID, patronID) >= 0 {
		return opErr(ErrHoldExists, "patron %q already holds book %q", patronID, bookID)
	}
	d.Holds[bookID] = append(d.Holds[bookID], patronID)
	return nil
}

// CancelHold 取消预约；取消后重新预约排到队尾。
func (d *Desk) CancelHold(patronID, bookID string) *OpError {
	if err := d.requireInit(); err != nil {
		return err
	}
	if err := d.checkPatron(patronID); err != nil {
		return err
	}
	if err := d.checkBook(bookID); err != nil {
		return err
	}
	idx := d.holdsContains(bookID, patronID)
	if idx < 0 {
		return opErr(ErrInvalidRequest, "patron %q has no hold on book %q", patronID, bookID)
	}
	queue := d.Holds[bookID]
	d.Holds[bookID] = append(queue[:idx], queue[idx+1:]...)
	return nil
}

// Checkout 按规则借书：有候补时仅队首可借，选字典序最小的可借条码。
func (d *Desk) Checkout(patronID, bookID string) (string, string, *OpError) {
	if err := d.requireInit(); err != nil {
		return "", "", err
	}
	if err := d.checkPatron(patronID); err != nil {
		return "", "", err
	}
	if err := d.checkBook(bookID); err != nil {
		return "", "", err
	}

	queue := d.Holds[bookID]
	if len(queue) > 0 && queue[0] != patronID {
		return "", "", opErr(ErrNotHeadOfQueue, "patron %q is not at the head of the hold queue for book %q", patronID, bookID)
	}

	active := d.activeLoans(patronID)
	if len(active) >= d.LoanLimit {
		return "", "", opErr(ErrLimitReached, "patron %q reached the loan limit of %d", patronID, d.LoanLimit)
	}
	for _, l := range active {
		if l.BookID == bookID {
			return "", "", opErr(ErrAlreadyLoanedBook, "patron %q already loans a copy of book %q", patronID, bookID)
		}
	}

	var barcodes []string
	for barcode, c := range d.Copies {
		if c.BookID == bookID && !c.OnLoan {
			barcodes = append(barcodes, barcode)
		}
	}
	if len(barcodes) == 0 {
		return "", "", opErr(ErrNoCopyAvailable, "no available copy of book %q", bookID)
	}
	sort.Strings(barcodes)
	barcode := barcodes[0]

	d.LoanSeq++
	loanID := formatLoanID(d.LoanSeq)
	copy := d.Copies[barcode]
	copy.OnLoan = true
	copy.LoanID = loanID
	copy.PatronID = patronID
	d.Loans[loanID] = &Loan{ID: loanID, PatronID: patronID, BookID: bookID, Barcode: barcode, Returned: false}

	// 借阅成功后移除队首预约（即读者本人的预约）。
	if len(queue) > 0 && queue[0] == patronID {
		d.Holds[bookID] = append([]string{}, queue[1:]...)
	}
	return loanID, barcode, nil
}

func formatLoanID(seq int) string {
	// 固定宽度让编号字典序与生成顺序一致。
	return "L" + padLeft(itoa(seq), 10)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func padLeft(s string, width int) string {
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// ReturnBook 按借阅编号归还，释放原册；重复或未知编号明确拒绝。
func (d *Desk) ReturnBook(loanID string) *OpError {
	if err := d.requireInit(); err != nil {
		return err
	}
	loan, ok := d.Loans[loanID]
	if !ok {
		return opErr(ErrUnknownLoan, "loan %q does not exist", loanID)
	}
	if loan.Returned {
		return opErr(ErrAlreadyReturned, "loan %q has already been returned", loanID)
	}
	loan.Returned = true
	copy := d.Copies[loan.Barcode]
	copy.OnLoan = false
	copy.LoanID = ""
	copy.PatronID = ""
	return nil
}
