package workerlog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// ErrOffset indicates an invalid log chunk or a conflict with stored content.
var ErrOffset = errors.New("Log offset does not match stored content")

// Striped locks serialize writes for each VM without retaining locks for deleted VMs.
var writeLocks [64]sync.Mutex

// Store persists worker logs in separate files for each instance and session.
type Store struct {
	directory string
}

// NewStore creates a worker log store rooted at directory.
func NewStore(directory string) *Store {
	return &Store{directory: directory}
}

// Path returns the log file path for an instance and worker session.
func (s *Store) Path(instance uuid.UUID, session uuid.UUID) string {
	return filepath.Join(s.directory, instance.String(), session.String()+".log")
}

// Append stores a chunk idempotently and returns the next byte offset after syncing it.
func (s *Store) Append(instance uuid.UUID, session uuid.UUID, offset int64, data []byte) (int64, error) {
	if offset < 0 || len(data) == 0 || len(data) > api.WorkerLogChunkSize {
		return 0, ErrOffset
	}

	lock := &writeLocks[int(instance[0])%len(writeLocks)]
	lock.Lock()
	defer lock.Unlock()

	err := os.MkdirAll(filepath.Dir(s.Path(instance, session)), 0o700)
	if err != nil {
		return 0, err
	}

	flags := os.O_RDWR
	if offset == 0 {
		flags |= os.O_CREATE
	}

	f, err := os.OpenFile(s.Path(instance, session), flags, 0o600)
	if err != nil {
		return 0, err
	}

	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	size := info.Size()
	if offset > size {
		return size, ErrOffset
	}

	// Compare retransmitted bytes before appending anything not already stored.
	overlap := min(int64(len(data)), size-offset)
	previous := make([]byte, overlap)
	_, err = f.ReadAt(previous, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return size, err
	}

	if !bytes.Equal(previous, data[:overlap]) {
		return size, ErrOffset
	}

	_, err = f.WriteAt(data[overlap:], offset+overlap)
	if err != nil {
		return size, err
	}

	err = f.Sync()
	if err != nil {
		return size, err
	}

	return max(size, offset+int64(len(data))), nil
}

// Sessions lists an instance's log sessions by descending modification time.
func (s *Store) Sessions(instance uuid.UUID) ([]api.WorkerLogSession, error) {
	entries, err := os.ReadDir(filepath.Join(s.directory, instance.String()))
	if errors.Is(err, os.ErrNotExist) {
		return []api.WorkerLogSession{}, nil
	}

	if err != nil {
		return nil, err
	}

	sessions := make([]api.WorkerLogSession, 0, len(entries))
	for _, entry := range entries {
		name, found := strings.CutSuffix(entry.Name(), ".log")
		if !found || entry.IsDir() {
			continue
		}

		_, err = uuid.Parse(name)
		if err != nil {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, api.WorkerLogSession{Session: name, Size: info.Size(), Modified: info.ModTime()})
	}

	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Modified.After(sessions[j].Modified) })
	return sessions, nil
}

// Read returns a bounded preview from the start when oldestFirst is true, or the end otherwise.
func (s *Store) Read(instance uuid.UUID, session uuid.UUID, lines int, oldestFirst bool) (api.WorkerLogContent, error) {
	if lines < 1 || lines > api.WorkerLogMaxLines {
		return api.WorkerLogContent{}, fmt.Errorf("Line count must be between 1 and %d", api.WorkerLogMaxLines)
	}

	f, err := os.Open(s.Path(instance, session))
	if err != nil {
		return api.WorkerLogContent{}, err
	}

	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return api.WorkerLogContent{}, err
	}

	readLines := readTail
	if oldestFirst {
		readLines = readHead
	}

	data, truncated, err := readLines(f, info.Size(), lines)
	if err != nil {
		return api.WorkerLogContent{}, err
	}

	return api.WorkerLogContent{Content: string(data), Size: info.Size(), Truncated: truncated}, nil
}

func readHead(file io.ReaderAt, size int64, lines int) ([]byte, bool, error) {
	data := make([]byte, min(size, api.WorkerLogPreviewSize))
	count := 0
	for start := 0; start < len(data); {
		end := min(start+api.WorkerLogChunkSize, len(data))
		_, err := file.ReadAt(data[start:end], int64(start))
		if err != nil {
			return nil, false, err
		}

		for i := start; i < end; i++ {
			if data[i] != '\n' {
				continue
			}

			count++
			if count == lines {
				return data[:i+1], int64(i+1) < size, nil
			}
		}

		start = end
	}

	return data, int64(len(data)) < size, nil
}

func readTail(file io.ReaderAt, size int64, lines int) ([]byte, bool, error) {
	base := max(0, size-api.WorkerLogPreviewSize)
	data := make([]byte, size-base)
	start := len(data)
	count := 0

	// Read backwards until enough lines are found, up to the preview byte limit.
	for start > 0 {
		end := start
		start = max(0, end-api.WorkerLogChunkSize)
		_, err := file.ReadAt(data[start:end], base+int64(start))
		if err != nil {
			return nil, false, err
		}

		if end == len(data) && data[end-1] == '\n' {
			end--
		}

		for i := end - 1; i >= start; i-- {
			if data[i] != '\n' {
				continue
			}

			count++
			if count == lines {
				return data[i+1:], base+int64(i+1) > 0, nil
			}
		}
	}

	return data, base > 0, nil
}
