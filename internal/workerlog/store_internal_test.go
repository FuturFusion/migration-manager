package workerlog

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type countingReader struct {
	io.ReaderAt
	bytesRead int
}

// ReadAt records the number of bytes read from the wrapped reader.
func (r *countingReader) ReadAt(data []byte, offset int64) (int, error) {
	n, err := r.ReaderAt.ReadAt(data, offset)
	r.bytesRead += n
	return n, err
}

func TestReadPreview(t *testing.T) {
	cases := []struct {
		name      string
		readLines func(io.ReaderAt, int64, int) ([]byte, bool, error)
	}{
		{name: "oldest first", readLines: readHead},
		{name: "latest lines", readLines: readTail},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, line := range []string{"short line\n", strings.Repeat("long line ", 100) + "\n"} {
				content := strings.Repeat(line, 2000)
				reader := &countingReader{ReaderAt: strings.NewReader(content)}
				data, truncated, err := tc.readLines(reader, int64(len(content)), 200)
				require.NoError(t, err)
				require.Equal(t, strings.Repeat(line, 200), string(data))
				require.True(t, truncated)
				require.LessOrEqual(t, reader.bytesRead, len(data)+api.WorkerLogChunkSize)
			}

			for _, content := range []string{"", "first line\nlast line", "first line\nlast line\n"} {
				data, truncated, err := tc.readLines(strings.NewReader(content), int64(len(content)), 200)
				require.NoError(t, err)
				require.Equal(t, content, string(data))
				require.False(t, truncated)
			}
		})
	}
}
