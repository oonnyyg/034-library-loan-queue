package libdesk

import "sort"

// LoanRecord 是一条借阅历史记录。归还后 Returned 为 true。
type LoanRecord struct {
	LoanID      int64  `json:"loan_id"`
	ReaderID    string `json:"reader_id"`
	TitleID     string `json:"title_id"`
	CopyBarcode string `json:"copy_barcode"`
	Returned    bool   `json:"returned"`
}

// Reservation 表示某书目的一个候补位置。
type Reservation struct {
	ReaderID string `json:"reader_id"`
	TitleID  string `json:"title_id"`
	Seq      int64  `json:"seq"`
}

// State 是图书馆工具的全部持久化状态。
type State struct {
	Initialized  bool                        `json:"initialized"`
	LoanLimit    int                         `json:"loan_limit"`
	NextLoanID   int64                       `json:"next_loan_id"`
	NextResvSeq  int64                       `json:"next_reservation_seq"`
	Readers      map[string]bool             `json:"readers"`
	Titles       map[string]bool             `json:"titles"`
	Copies       map[string]string           `json:"copy_title"` // 条码 -> 书目号
	CopyStatus   map[string]bool             `json:"copy_out"`   // 条码 -> 是否借出
	Loans        map[int64]*LoanRecord       `json:"loans"`
	Queues       map[string][]string         `json:"queues"`       // 书目号 -> 候补读者顺序
	Reservations map[string]map[string]int64 `json:"reservations"` // 书目号 -> 读者 -> 排队序号
}

func newState() *State {
	return &State{
		NextLoanID:   1,
		NextResvSeq:  1,
		Readers:      map[string]bool{},
		Titles:       map[string]bool{},
		Copies:       map[string]string{},
		CopyStatus:   map[string]bool{},
		Loans:        map[int64]*LoanRecord{},
		Queues:       map[string][]string{},
		Reservations: map[string]map[string]int64{},
	}
}

func sortedStrings(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
