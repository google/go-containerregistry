package limitio

import (
	"errors"
	"fmt"
	"io"
)

var ErrLimitExceeded = errors.New("read limit exceeded")

func ReadAll(r io.Reader, max int64) ([]byte, error) {
	if max < 0 {
		return nil, fmt.Errorf("invalid max: %d", max)
	}

	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("%w: read=%d max=%d", ErrLimitExceeded, len(b), max)
	}
	return b, nil
}
