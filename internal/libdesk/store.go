package libdesk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const stateFileName = "librarydesk-state.json"

// OpError 携带可区分的失败原因，业务操作失败时不改变任何状态。
type OpError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *OpError) Error() string { return e.Code + ": " + e.Message }

func opError(code, format string, args ...any) error {
	return &OpError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// LoadOrCreate 从数据目录载入状态；目录不存在时创建。
func LoadOrCreate(dir string) (*Desk, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("state directory must not be empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	path := filepath.Join(dir, stateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Desk{dir: dir, State: newState()}, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}
	st := newState()
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	return &Desk{dir: dir, State: st}, nil
}

// Save 以临时文件加重命名的方式原子写入全部状态。
func (d *Desk) Save() error {
	data, err := json.MarshalIndent(d.State, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	path := filepath.Join(d.dir, stateFileName)
	tmp, err := os.CreateTemp(d.dir, ".librarydesk-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}
