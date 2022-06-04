package logstore

import (
	"bufio"
	"errors"
	"io"
)

// Scanner is a custom bufio.Scanner to read the database file and create records from it
type Scanner struct {
	*bufio.Scanner
}

func NewScanner(r io.Reader, maxTokenSize int) (*Scanner, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 4096)
	scanner.Buffer(buf, maxTokenSize+headerLength)
	scanner.Split(split)
	return &Scanner{scanner}, nil
}

// split implements the SplitFunc interface for the custom scanner
func split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	record, err := Deserialize(data)
	if err != nil {
		if errors.Is(err, ErrInsufficientData) {
			// The scanner read not enough bytes, to parse a whole record. Let's continue
			return 0, nil, nil
		}

		return 0, nil, err
	}

	advance = record.Size()
	token = data[:advance]
	return
}

func (s *Scanner) Record() *Record {
	r, _ := Deserialize(s.Bytes())
	return r
}
