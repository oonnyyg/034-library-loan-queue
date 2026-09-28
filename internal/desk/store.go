package desk

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const stateFile = "librarydesk-state.json"

// Load 从数据目录读取状态；目录不存在时创建；无状态文件时返回未初始化的空台。
func Load(dir string) (*Desk, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			d := New()
			d.dir = dir
			return d, nil
		}
		return nil, err
	}
	d := New()
	if err := json.Unmarshal(data, d); err != nil {
		return nil, err
	}
	d.afterLoad()
	d.dir = dir
	return d, nil
}

// afterLoad 补齐旧数据中可能为 nil 的 map。
func (d *Desk) afterLoad() {
	if d.Patrons == nil {
		d.Patrons = map[string]bool{}
	}
	if d.Books == nil {
		d.Books = map[string]bool{}
	}
	if d.Copies == nil {
		d.Copies = map[string]*CopyState{}
	}
	if d.Holds == nil {
		d.Holds = map[string][]string{}
	}
	if d.Loans == nil {
		d.Loans = map[string]*Loan{}
	}
}

// Save 将状态原子写入数据目录：先写临时文件再改名，避免半截文件。
func (d *Desk) Save() error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(d.dir, stateFile)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}
