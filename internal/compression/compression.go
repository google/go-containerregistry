package compression

import (
	"bufio"
	"bytes"
	"github.com/google/go-containerregistry/internal/gzip"
	"github.com/google/go-containerregistry/internal/zstd"
	"io"
)

type Compression string

// The collection of known MediaType values.
const (
	None Compression = "none"
	GZip Compression = "gzip"
	ZStd Compression = "zstd"
)

type Opener = func() (io.ReadCloser, error)

func CheckCompression(opener Opener, checker func(reader io.Reader) (bool, error)) (bool, error) {
	rc, err := opener()
	if err != nil {
		return false, err
	}
	defer rc.Close()

	return checker(rc)
}

func GetCompression(opener Opener) (Compression, error) {
	if compressed, err := CheckCompression(opener, gzip.Is); err != nil {
		return None, err
	} else if compressed {
		return GZip, nil
	}

	if compressed, err := CheckCompression(opener, zstd.Is); err != nil {
		return None, err
	} else if compressed {
		return ZStd, nil
	}

	return None, nil
}

// PeekReader is an io.Reader that also implements Peek a la bufio.Reader.
type PeekReader interface {
	io.Reader
	Peek(n int) ([]byte, error)
}

// PeekCompression detects whether the input stream is compressed and which algorithm is used.
//
// If r implements Peek, we will use that directly, otherwise a small number
// of bytes are buffered to Peek at the gzip header, and the returned
// PeekReader can be used as a replacement for the consumed input io.Reader.
func PeekCompression(r io.Reader) (Compression, PeekReader, error) {
	var pr PeekReader
	if p, ok := r.(PeekReader); ok {
		pr = p
	} else {
		pr = bufio.NewReader(r)
	}

	var header []byte
	var err error

	if header, err = pr.Peek(2); err != nil {
		// https://github.com/google/go-containerregistry/issues/367
		if err == io.EOF {
			return None, pr, nil
		}
		return None, pr, err
	}

	if bytes.Equal(header, gzip.MagicHeader) {
		return GZip, pr, nil
	}

	if header, err = pr.Peek(4); err != nil {
		// https://github.com/google/go-containerregistry/issues/367
		if err == io.EOF {
			return None, pr, nil
		}
		return None, pr, err
	}

	if bytes.Equal(header, zstd.MagicHeader) {
		return ZStd, pr, nil
	}

	return None, pr, nil
}
