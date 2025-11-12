package transport

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/google/go-containerregistry/pkg/logs"
)

// NewResumable creates a http.RoundTripper that resumes http GET from error,
// and the inner should be wrapped with retry transport, otherwise, the
// transport will abort if resume() returns error.
func NewResumable(inner http.RoundTripper) http.RoundTripper {
	return &resumableTransport{inner: inner}
}

var (
	contentRangeRe = regexp.MustCompile(`^bytes (\d+)-(\d+)/(\d+|\*)$`)
)

type resumableTransport struct {
	inner http.RoundTripper
}

func (rt *resumableTransport) RoundTrip(in *http.Request) (*http.Response, error) {
	if in.Method != http.MethodGet {
		return rt.inner.RoundTrip(in)
	}

	req := in.Clone(in.Context())
	req.Header.Set("Range", "bytes=0-")
	resp, err := rt.inner.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		// fallback to previous behavior
		resp.Body.Close()
		return rt.inner.RoundTrip(in)
	default:
		return resp, nil
	}

	var contentLength int64
	if _, _, contentLength, err = parseContentRange(resp.Header.Get("Content-Range")); err != nil || contentLength <= 0 {
		// fallback to previous behavior
		resp.Body.Close()
		return rt.inner.RoundTrip(in)
	}

	// modify response status to 200, ensure caller error checking works
	resp.StatusCode = http.StatusOK
	resp.Status = "200 OK"
	resp.ContentLength = contentLength
	resp.Body = &resumableBody{
		rc:          resp.Body,
		inner:       rt.inner,
		req:         req,
		total:       contentLength,
		transferred: 0,
	}

	return resp, nil
}

type resumableBody struct {
	rc io.ReadCloser

	inner http.RoundTripper
	req   *http.Request

	transferred int64
	total       int64

	closed uint32
}

func (rb *resumableBody) Read(p []byte) (n int, err error) {
	if atomic.LoadUint32(&rb.closed) == 1 {
		// response body already closed
		return 0, http.ErrBodyReadAfterClose
	} else if rb.total >= 0 && rb.transferred >= rb.total {
		return 0, io.EOF
	}

resume:
	if n, err = rb.rc.Read(p); n > 0 {
		rb.transferred += int64(n)
	}

	if err == nil {
		return
	}

	if errors.Is(err, io.EOF) && rb.total >= 0 && rb.transferred == rb.total {
		return
	}

	if err = rb.resume(err); err == nil {
		if n == 0 {
			// zero bytes read, try reading again with new response.Body
			goto resume
		}

		// already read some bytes from previous response.Body, returns and waits for next Read operation
	}

	return n, err
}

func (rb *resumableBody) Close() (err error) {
	if !atomic.CompareAndSwapUint32(&rb.closed, 0, 1) {
		return nil
	}

	return rb.rc.Close()
}

func (rb *resumableBody) resume(reason error) error {
	if reason != nil {
		logs.Debug.Printf("Resume http transporting from error: %v", reason)
	}

	ctx := rb.req.Context()
	select {
	case <-ctx.Done():
		// context already done, stop resuming from error
		return ctx.Err()
	default:
	}

	req := rb.req.Clone(ctx)
	req.Header.Set("Range", "bytes="+strconv.FormatInt(rb.transferred, 10)+"-")
	resp, err := rb.inner.RoundTrip(req)
	if err != nil {
		return err
	}

	if err = rb.validate(resp); err != nil {
		resp.Body.Close()
		return err
	}

	if atomic.LoadUint32(&rb.closed) == 1 {
		resp.Body.Close()
		return http.ErrBodyReadAfterClose
	}

	rb.rc.Close()
	rb.rc = resp.Body

	return nil
}

func (rb *resumableBody) validate(resp *http.Response) (err error) {
	var start, total int64
	switch resp.StatusCode {
	case http.StatusPartialContent:
		if start, _, total, err = parseContentRange(resp.Header.Get("Content-Range")); err != nil {
			return err
		}

		if total > rb.total {
			rb.total = total
		}

		if start == rb.transferred {
			break
		} else if start < rb.transferred {
			if _, err := io.CopyN(io.Discard, resp.Body, rb.transferred-start); err != nil {
				return fmt.Errorf("discard overlapped data failed, %v", err)
			}
		} else {
			return fmt.Errorf("unexpected resume start %d, wanted: %d", start, rb.transferred)
		}
	case http.StatusOK:
		if rb.transferred > 0 {
			if _, err = io.CopyN(io.Discard, resp.Body, rb.transferred); err != nil {
				return err
			}
		}
	case http.StatusRequestedRangeNotSatisfiable:
		if contentRange := resp.Header.Get("Content-Range"); contentRange != "" && strings.HasPrefix(contentRange, "bytes */") {
			if total, err = strconv.ParseInt(strings.TrimPrefix(contentRange, "bytes */"), 10, 64); err == nil && total >= 0 && rb.transferred >= total {
				return io.EOF
			}
		}

		fallthrough
	default:
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func parseContentRange(contentRange string) (start, end, size int64, err error) {
	if contentRange == "" {
		return -1, -1, -1, errors.New("unexpected empty content range")
	}

	matches := contentRangeRe.FindStringSubmatch(contentRange)
	if len(matches) != 4 {
		return -1, -1, -1, fmt.Errorf("invalid content range: %s", contentRange)
	}

	if start, err = strconv.ParseInt(matches[1], 10, 64); err != nil {
		return -1, -1, -1, fmt.Errorf("unexpected start from content range '%s', %v", contentRange, err)
	}

	if end, err = strconv.ParseInt(matches[2], 10, 64); err != nil {
		return -1, -1, -1, fmt.Errorf("unexpected end from content range '%s', %v", contentRange, err)
	}

	if start > end {
		return -1, -1, -1, fmt.Errorf("invalid content range: %s", contentRange)
	}

	if matches[3] == "*" {
		size = -1
	} else {
		size, err = strconv.ParseInt(matches[3], 10, 64)
		if err != nil {
			return -1, -1, -1, fmt.Errorf("unexpected total from content range '%s', %v", contentRange, err)
		}

		if end >= size {
			return -1, -1, -1, fmt.Errorf("invalid content range: %s", contentRange)
		}
	}

	return
}
