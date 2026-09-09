package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileArchive is an idempotent local/evaluation archive. Production object
// storage implementations should implement Archive directly; none is faked here.
type FileArchive struct{ Root string }
type archivedRecord struct {
	Topic     string `json:"topic"`
	Partition int    `json:"partition"`
	Offset    int64  `json:"offset"`
	Key       []byte `json:"key"`
	Value     []byte `json:"value"`
	EventID   string `json:"event_id"`
	Tenant    string `json:"tenant"`
}

func (f *FileArchive) path(r ArchiveRecord) string {
	topic := sha256.Sum256([]byte(r.Topic))
	return filepath.Join(f.Root, hex.EncodeToString(topic[:8]), fmt.Sprintf("%06d", r.Partition), fmt.Sprintf("%020d.json", r.Offset))
}
func (f *FileArchive) Write(ctx context.Context, records []ArchiveRecord) error {
	for _, r := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := f.path(r)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		b, err := json.Marshal(archivedRecord{Topic: r.Topic, Partition: r.Partition, Offset: r.Offset, Key: r.Key, Value: r.Value, EventID: r.Event.ID, Tenant: r.Event.Tenant})
		if err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(path), ".archive-*")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()
		if err = tmp.Chmod(0o640); err == nil {
			_, err = tmp.Write(b)
		}
		if closeErr := tmp.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmpName, path)
		}
		if err != nil {
			_ = os.Remove(tmpName)
			return err
		}
	}
	return nil
}
func (f *FileArchive) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.Root == "" {
		return errors.New("archive root is empty")
	}
	return os.MkdirAll(f.Root, 0o750)
}
