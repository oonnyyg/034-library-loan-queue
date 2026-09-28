package desk

import "fmt"

// ErrorCode 为可区分的失败原因。
type ErrorCode string

const (
	ErrNotInitialized     ErrorCode = "not_initialized"
	ErrAlreadyInitialized ErrorCode = "already_initialized"
	ErrInvalidRequest     ErrorCode = "invalid_request"
	ErrUnknownPatron      ErrorCode = "unknown_patron"
	ErrUnknownBook        ErrorCode = "unknown_book"
	ErrDuplicatePatron    ErrorCode = "duplicate_patron"
	ErrDuplicateBook      ErrorCode = "duplicate_book"
	ErrDuplicateCopy      ErrorCode = "duplicate_copy"
	ErrUnknownLoan        ErrorCode = "unknown_loan"
	ErrAlreadyReturned    ErrorCode = "already_returned"
	ErrLimitReached       ErrorCode = "loan_limit_reached"
	ErrAlreadyLoanedBook  ErrorCode = "already_loaned_same_book"
	ErrHoldExists         ErrorCode = "hold_exists"
	ErrHoldWhileLoaning   ErrorCode = "hold_blocked_while_loaning"
	ErrNotHeadOfQueue     ErrorCode = "not_head_of_queue"
	ErrNoCopyAvailable    ErrorCode = "no_copy_available"
)

// OpError 携带稳定错误码，失败操作不改变任何状态。
type OpError struct {
	Code    ErrorCode
	Message string
}

func (e *OpError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func opErr(code ErrorCode, format string, args ...any) *OpError {
	return &OpError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Loan 是一条借阅记录，归还后 RetiredAt 非空并保留在历史中。
type Loan struct {
	ID       string `json:"id"`
	PatronID string `json:"patronId"`
	BookID   string `json:"bookId"`
	Barcode  string `json:"barcode"`
	Returned bool   `json:"returned"`
}

// CopyState 描述一册书的当前状态。
type CopyState struct {
	Barcode  string `json:"barcode"`
	BookID   string `json:"bookId"`
	OnLoan   bool   `json:"onLoan"`
	LoanID   string `json:"loanId,omitempty"`
	PatronID string `json:"patronId,omitempty"`
}

// Desk 是图书馆借还台的全部可持久化状态。
type Desk struct {
	Initialized bool `json:"initialized"`
	LoanLimit   int  `json:"loanLimit"`

	Patrons map[string]bool       `json:"patrons"`
	Books   map[string]bool       `json:"books"`
	Copies  map[string]*CopyState `json:"copies"`

	// Holds: 书目 -> 按排队顺序排列的读者号。
	Holds map[string][]string `json:"holds"`
	// Loans: 借阅编号 -> 记录。
	Loans   map[string]*Loan `json:"loans"`
	LoanSeq int              `json:"loanSeq"`
	dir     string           `json:"-"`
}

// New 返回未初始化的空借还台。
func New() *Desk {
	return &Desk{
		Patrons: map[string]bool{},
		Books:   map[string]bool{},
		Copies:  map[string]*CopyState{},
		Holds:   map[string][]string{},
		Loans:   map[string]*Loan{},
	}
}

func (d *Desk) requireInit() *OpError {
	if !d.Initialized {
		return opErr(ErrNotInitialized, "desk is not initialized; send an init request first")
	}
	return nil
}

// Init 设置正整数借阅上限，只能设置一次。
func (d *Desk) Init(limit int) *OpError {
	if d.Initialized {
		return opErr(ErrAlreadyInitialized, "loan limit already set to %d", d.LoanLimit)
	}
	if limit <= 0 {
		return opErr(ErrInvalidRequest, "loan limit must be a positive integer, got %d", limit)
	}
	d.LoanLimit = limit
	d.Initialized = true
	return nil
}

// RegisterPatron 登记唯一读者号。
func (d *Desk) RegisterPatron(patronID string) *OpError {
	if err := d.requireInit(); err != nil {
		return err
	}
	if patronID == "" {
		return opErr(ErrInvalidRequest, "patronId must not be empty")
	}
	if d.Patrons[patronID] {
		return opErr(ErrDuplicatePatron, "patron %q already registered", patronID)
	}
	d.Patrons[patronID] = true
	return nil
}

// RegisterBook 登记唯一书目号。
func (d *Desk) RegisterBook(bookID string) *OpError {
	if err := d.requireInit(); err != nil {
		return err
	}
	if bookID == "" {
		return opErr(ErrInvalidRequest, "bookId must not be empty")
	}
	if d.Books[bookID] {
		return opErr(ErrDuplicateBook, "book %q already registered", bookID)
	}
	d.Books[bookID] = true
	return nil
}

// RegisterCopy 为已登记书目登记唯一条码的一册。
func (d *Desk) RegisterCopy(bookID, barcode string) *OpError {
	if err := d.requireInit(); err != nil {
		return err
	}
	if barcode == "" {
		return opErr(ErrInvalidRequest, "barcode must not be empty")
	}
	if !d.Books[bookID] {
		return opErr(ErrUnknownBook, "cannot register copy for unknown book %q", bookID)
	}
	if _, ok := d.Copies[barcode]; ok {
		return opErr(ErrDuplicateCopy, "copy %q already registered", barcode)
	}
	d.Copies[barcode] = &CopyState{Barcode: barcode, BookID: bookID}
	return nil
}
