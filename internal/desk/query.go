package desk

import "sort"

// BookView 是书目的各册状态与候补顺序视图。
type BookView struct {
	BookID    string       `json:"bookId"`
	Copies    []*CopyState `json:"copies"`
	HoldQueue []string     `json:"holdQueue"`
}

// QueryBook 返回书目的各册（按条码稳定排序）与候补队列（排队顺序）。
func (d *Desk) QueryBook(bookID string) (*BookView, *OpError) {
	if err := d.requireInit(); err != nil {
		return nil, err
	}
	if err := d.checkBook(bookID); err != nil {
		return nil, err
	}
	var copies []*CopyState
	for _, c := range d.Copies {
		if c.BookID == bookID {
			snapshot := *c
			copies = append(copies, &snapshot)
		}
	}
	sort.Slice(copies, func(i, j int) bool { return copies[i].Barcode < copies[j].Barcode })
	queue := append([]string{}, d.Holds[bookID]...)
	return &BookView{BookID: bookID, Copies: copies, HoldQueue: queue}, nil
}

// PatronView 是读者当前借阅与历史视图。
type PatronView struct {
	PatronID    string   `json:"patronId"`
	ActiveLoans []*Loan  `json:"activeLoans"`
	History     []*Loan  `json:"history"`
	Holds       []string `json:"holds"`
}

// QueryPatron 返回读者当前借阅、历史（均按借阅编号稳定排序）与预约（按各书目排队顺序汇总）。
func (d *Desk) QueryPatron(patronID string) (*PatronView, *OpError) {
	if err := d.requireInit(); err != nil {
		return nil, err
	}
	if err := d.checkPatron(patronID); err != nil {
		return nil, err
	}
	view := &PatronView{PatronID: patronID, ActiveLoans: []*Loan{}, History: []*Loan{}, Holds: []string{}}
	loanIDs := make([]string, 0, len(d.Loans))
	for id := range d.Loans {
		loanIDs = append(loanIDs, id)
	}
	sort.Strings(loanIDs)
	for _, id := range loanIDs {
		l := d.Loans[id]
		if l.PatronID != patronID {
			continue
		}
		snapshot := *l
		if l.Returned {
			view.History = append(view.History, &snapshot)
		} else {
			view.ActiveLoans = append(view.ActiveLoans, &snapshot)
		}
	}
	bookIDs := make([]string, 0, len(d.Holds))
	for bookID := range d.Holds {
		bookIDs = append(bookIDs, bookID)
	}
	sort.Strings(bookIDs)
	for _, bookID := range bookIDs {
		for _, id := range d.Holds[bookID] {
			if id == patronID {
				view.Holds = append(view.Holds, bookID)
			}
		}
	}
	return view, nil
}
