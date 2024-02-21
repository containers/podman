package magic

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
)

// Csv matches a comma-separated values file.
func Csv(raw []byte, limit uint32) bool {
	return sv(raw, ',', limit)
}

// Tsv matches a tab-separated values file.
func Tsv(raw []byte, limit uint32) bool {
	return sv(raw, '\t', limit)
}

func sv(in []byte, comma rune, limit uint32) bool {
	r := csv.NewReader(dropLastLine(in, limit))
	r.Comma = comma
	r.ReuseRecord = true
	r.LazyQuotes = true
	r.Comment = '#'

	lines := 0
	for {
		_, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false
		}
		lines++
	}

	return r.FieldsPerRecord > 1 && lines > 1
}

// dropLastLine drops the last incomplete line from b.
//
// mimetype limits itself to ReadLimit bytes when performing a detection.
// This means, for file formats like CSV for NDJSON, the last line of the input
// can be an incomplete line.
func dropLastLine(b []byte, cutAt uint32) io.Reader {
	if cutAt == 0 {
		return bytes.NewReader(b)
	}
	if uint32(len(b)) >= cutAt {
		for i := cutAt - 1; i > 0; i-- {
			if b[i] == '\n' {
				return bytes.NewReader(b[:i])
			}
		}

		// No newline was found between the 0 index and cutAt.
		return bytes.NewReader(b[:cutAt])
	}

	return bytes.NewReader(b)
}
