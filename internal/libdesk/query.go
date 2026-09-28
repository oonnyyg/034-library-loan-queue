package libdesk

import "sort"

// CopyInfo 描述一册图书的当前状态。
type CopyInfo struct {
	Barcode string `json:"barcode"`
	TitleID string `json:"title_id"`
	Out     bool   `json:"out"`
	LoanID  int64  `json:"loan_id,omitempty"`
}

// TitleStatus 是书目的查询结果：各册状态与候补顺序。
type TitleStatus struct {
	TitleID   string     `json:"title_id"`
	Copies    []CopyInfo `json:"copies"`
	WaitQueue []string   `json:"wait_queue"`
}

// ReaderStatus 是读者的查询结果：当前借阅与全部历史。
type ReaderStatus struct {
	ReaderID     string        `json:"reader_id"`
	ActiveLoans  []LoanRecord  `json:"active_loans"`
	History      []LoanRecord  `json:"history"`
	Reservations []Reservation `json:"reservations"`
}

// QueryTitle 查询书目各册状态（条码字典序）和候补顺序（排队顺序）。
func (d *Desk) QueryTitle(titleID string) (*TitleStatus, error) {
	if err := d.requireInit(); err != nil {
		return nil, err
	}
	if !d.State.Titles[titleID] {
		return nil, opError("unknown_title", "title %q is not registered", titleID)
	}
	barcodes := make([]string, 0)
	for barcode, t := range d.State.Copies {
		if t == titleID {
			barcodes = append(barcodes, barcode)
		}
	}
	sort.Strings(barcodes)

	copies := []CopyInfo{}
	for _, barcode := range barcodes {
		info := CopyInfo{Barcode: barcode, TitleID: titleID, Out: d.State.CopyStatus[barcode]}
		if info.Out {
			for id, rec := range d.State.Loans {
				if rec.CopyBarcode == barcode && !rec.Returned {
					info.LoanID = id
					break
				}
			}
		}
		copies = append(copies, info)
	}

	queue := []string{}
	queue = append(queue, d.State.Queues[titleID]...)
	return &TitleStatus{TitleID: titleID, Copies: copies, WaitQueue: queue}, nil
}

// QueryReader 查询读者当前借阅与历史（按借阅编号升序）。
func (d *Desk) QueryReader(readerID string) (*ReaderStatus, error) {
	if err := d.requireInit(); err != nil {
		return nil, err
	}
	if !d.State.Readers[readerID] {
		return nil, opError("unknown_reader", "reader %q is not registered", readerID)
	}
	ids := make([]int64, 0, len(d.State.Loans))
	for id, rec := range d.State.Loans {
		if rec.ReaderID == readerID {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	st := &ReaderStatus{
		ReaderID:     readerID,
		ActiveLoans:  []LoanRecord{},
		History:      []LoanRecord{},
		Reservations: []Reservation{},
	}
	for _, id := range ids {
		rec := *d.State.Loans[id]
		st.History = append(st.History, rec)
		if !rec.Returned {
			st.ActiveLoans = append(st.ActiveLoans, rec)
		}
	}

	titleIDs := sortedKeys(d.State.Reservations)
	for _, titleID := range titleIDs {
		if seq, ok := d.State.Reservations[titleID][readerID]; ok {
			st.Reservations = append(st.Reservations, Reservation{
				ReaderID: readerID,
				TitleID:  titleID,
				Seq:      seq,
			})
		}
	}
	return st, nil
}
