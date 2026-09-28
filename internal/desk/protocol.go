package desk

import "encoding/json"

// Request 是逐行 JSON 请求；Op 决定所需字段。
type Request struct {
	Op       string `json:"op"`
	Limit    int    `json:"limit,omitempty"`
	PatronID string `json:"patronId,omitempty"`
	BookID   string `json:"bookId,omitempty"`
	Barcode  string `json:"barcode,omitempty"`
	LoanID   string `json:"loanId,omitempty"`
}

// Response 是统一 JSON 结果；失败时 ok=false 并带 errorCode/reason。
type Response struct {
	OK        bool        `json:"ok"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Reason    string      `json:"reason,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// Handle 处理单个请求。
func (d *Desk) Handle(raw []byte) Response {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return fail(ErrInvalidRequest, "request is not valid JSON: "+err.Error())
	}
	switch req.Op {
	case "init":
		return errResp(d.Init(req.Limit))
	case "registerPatron":
		return errResp(d.RegisterPatron(req.PatronID))
	case "registerBook":
		return errResp(d.RegisterBook(req.BookID))
	case "registerCopy":
		return errResp(d.RegisterCopy(req.BookID, req.Barcode))
	case "placeHold":
		return errResp(d.PlaceHold(req.PatronID, req.BookID))
	case "cancelHold":
		return errResp(d.CancelHold(req.PatronID, req.BookID))
	case "checkout":
		loanID, barcode, oerr := d.Checkout(req.PatronID, req.BookID)
		if oerr != nil {
			return errResp(oerr)
		}
		return ok(map[string]string{"loanId": loanID, "barcode": barcode})
	case "return":
		return errResp(d.ReturnBook(req.LoanID))
	case "queryBook":
		view, oerr := d.QueryBook(req.BookID)
		if oerr != nil {
			return errResp(oerr)
		}
		return ok(view)
	case "queryPatron":
		view, oerr := d.QueryPatron(req.PatronID)
		if oerr != nil {
			return errResp(oerr)
		}
		return ok(view)
	default:
		return fail(ErrInvalidRequest, "unknown op "+quote(req.Op))
	}
}

func ok(data interface{}) Response { return Response{OK: true, Data: data} }

func errResp(e *OpError) Response {
	if e == nil {
		return ok(struct{}{})
	}
	return fail(e.Code, e.Message)
}

func fail(code ErrorCode, reason string) Response {
	return Response{OK: false, ErrorCode: string(code), Reason: reason}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
