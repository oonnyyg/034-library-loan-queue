package libdesk

// Desk 封装借阅台状态与全部业务操作。
type Desk struct {
	dir   string
	State *State
}

func requireID(kind, id string) error {
	if id == "" {
		return opError("invalid_id", "%s id must not be empty", kind)
	}
	return nil
}

// Init 设置正整数借阅上限。仅可初始化一次，之后不可变更。
func (d *Desk) Init(limit int) error {
	if d.State.Initialized {
		return opError("already_initialized", "library already initialized with loan limit %d", d.State.LoanLimit)
	}
	if limit <= 0 {
		return opError("invalid_limit", "loan limit must be a positive integer, got %d", limit)
	}
	d.State.LoanLimit = limit
	d.State.Initialized = true
	return nil
}

func (d *Desk) requireInit() error {
	if !d.State.Initialized {
		return opError("not_initialized", "library is not initialized; send init first")
	}
	return nil
}

// RegisterReader 登记唯一读者号。
func (d *Desk) RegisterReader(readerID string) error {
	if err := d.requireInit(); err != nil {
		return err
	}
	if err := requireID("reader", readerID); err != nil {
		return err
	}
	if d.State.Readers[readerID] {
		return opError("duplicate_reader", "reader %q already registered", readerID)
	}
	d.State.Readers[readerID] = true
	return nil
}

// RegisterTitle 登记唯一书目号。
func (d *Desk) RegisterTitle(titleID string) error {
	if err := d.requireInit(); err != nil {
		return err
	}
	if err := requireID("title", titleID); err != nil {
		return err
	}
	if d.State.Titles[titleID] {
		return opError("duplicate_title", "title %q already registered", titleID)
	}
	d.State.Titles[titleID] = true
	d.State.Queues[titleID] = []string{}
	d.State.Reservations[titleID] = map[string]int64{}
	return nil
}

// RegisterCopy 登记唯一册条码，每册只属于一个已登记书目。
func (d *Desk) RegisterCopy(barcode, titleID string) error {
	if err := d.requireInit(); err != nil {
		return err
	}
	if err := requireID("copy", barcode); err != nil {
		return err
	}
	if err := requireID("title", titleID); err != nil {
		return err
	}
	if _, ok := d.State.Copies[barcode]; ok {
		return opError("duplicate_copy", "copy %q already registered", barcode)
	}
	if !d.State.Titles[titleID] {
		return opError("unknown_title", "title %q is not registered", titleID)
	}
	d.State.Copies[barcode] = titleID
	d.State.CopyStatus[barcode] = false
	return nil
}
