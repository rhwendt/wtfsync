// Package syncdir manages wtfsync's folder inside the user's cloud-synced folder.
package syncdir

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/rhwendt/wtfsync/internal/fsutil"
)

const (
	dirName    = "wtfsync"
	configName = "wtfsync.json"
	format     = 1
)

var (
	ErrUnreachable        = errors.New("sync folder unreachable")
	ErrNotInitialized     = errors.New("no wtfsync data in this folder")
	ErrAlreadyInitialized = errors.New("this folder already has wtfsync data")
)

// Dir is wtfsync's folder inside the user's cloud-synced folder.
type Dir struct {
	Root     string // <sync folder>/wtfsync
	Hydrator Hydrator
}

type config struct {
	Format     int       `json:"format"`
	Encryption string    `json:"encryption"`
	Created    time.Time `json:"created"`
}

// MachineRecord describes one PC. UUID is its identity; Name is only a label the user can change.
type MachineRecord struct {
	UUID     string    `json:"uuid"`
	Name     string    `json:"name"`
	LastSeen time.Time `json:"lastSeen"`
}

func reachable(folder string) error {
	fi, err := os.Stat(folder)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w: %s is not a folder", ErrUnreachable, folder)
	}
	return nil
}

// Init creates wtfsync's folder inside folder (setup on the first PC).
func Init(folder string, now time.Time, h Hydrator) (*Dir, error) {
	if err := reachable(folder); err != nil {
		return nil, err
	}
	d := &Dir{Root: filepath.Join(folder, dirName), Hydrator: h}
	if _, err := os.Stat(filepath.Join(d.Root, configName)); err == nil {
		return nil, ErrAlreadyInitialized
	}
	cfg := config{Format: format, Encryption: "none", Created: now.UTC()}
	if err := fsutil.WriteJSON(filepath.Join(d.Root, configName), cfg); err != nil {
		return nil, err
	}
	return d, nil
}

// Open opens the wtfsync folder a first PC created inside folder.
func Open(folder string, h Hydrator) (*Dir, error) {
	if err := reachable(folder); err != nil {
		return nil, err
	}
	d := &Dir{Root: filepath.Join(folder, dirName), Hydrator: h}
	var cfg config
	if err := d.readJSON(filepath.Join(d.Root, configName), &cfg); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotInitialized
		}
		return nil, err
	}
	if cfg.Format != format || cfg.Encryption != "none" {
		return nil, fmt.Errorf("unsupported wtfsync data (format %d, encryption %q): update wtfsync", cfg.Format, cfg.Encryption)
	}
	return d, nil
}

func (d *Dir) readJSON(path string, v any) error {
	b, err := d.Hydrator.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

func (d *Dir) machinePath(uuid string) string {
	return filepath.Join(d.Root, "machines", uuid+".json")
}

// WriteMachine writes this PC's record. Each PC only ever writes its own file.
func (d *Dir) WriteMachine(r MachineRecord) error {
	r.LastSeen = r.LastSeen.UTC()
	return fsutil.WriteJSON(d.machinePath(r.UUID), r)
}

// Machine reads a PC's record.
func (d *Dir) Machine(uuid string) (MachineRecord, error) {
	var r MachineRecord
	err := d.readJSON(d.machinePath(uuid), &r)
	return r, err
}
