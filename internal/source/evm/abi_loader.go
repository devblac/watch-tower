package evm

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// LoadABIs loads ABI JSON files from the provided directories.
func LoadABIs(dirs []string) (map[string]*abi.ABI, error) {
	abis := map[string]*abi.ABI{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read abi %s: %w", path, err)
			}
			a, err := abi.JSON(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("parse abi %s: %w", path, err)
			}
			abis[path] = &a
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return abis, nil
}

// FindEvent searches loaded ABIs for an event with the given name.
func FindEvent(abis map[string]*abi.ABI, eventName string) (*abi.Event, bool) {
	for _, a := range abis {
		if ev, ok := a.Events[eventName]; ok {
			return &ev, true
		}
	}
	return nil, false
}
