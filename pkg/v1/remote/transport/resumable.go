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
	"time"

	"github.com/google/go-containerregistry/pkg/logs"
)

// NewResumable creates a http.RoundTripper that resumes http GET from error, and continue
// transfer data from last successful transfer offset.
func NewResumable(inner http.RoundTripper, backoff Backoff) http.RoundTripper {
	if backoff.Steps <= 0 {
		// resume once
		backoff.Steps = 1
	}

	if backoff.Duration <= 0 {
		backoff.Duration = 100 * time.Millisecond
	}

	return &resumableTransport{inner: inner, backoff: backoff}
}

var (
	contentRangeRe = regexp.MustCompile(`^bytes (\d+)-(\d+)/(\d+|\*)$`)
	rangeRe        = regexp.MustCompile(`bytes=(\d+)-(\d+)?`)
)

type resumableTransport struct {
	inner   http.RoundTripper
	backoff Backoff
}

func (rt *resumableTransport) RoundTrip(in *http.Request) (resp *http.Response, err error) {
	var total, start, end int64
	// check initial request, maybe resumable transport is already enabled
	if contentRange := in.Header.Get("Range"); contentRange != "" {
		if matches := rangeRe.FindStringSubmatch(contentRange); len(matches) == 3 {
			if start, err = strconv.ParseInt(matches[1], 10, 64); err != nil {
				return nil, fmt.Errorf("invalid content range %q: %w", contentRange, err)
			}

			if len(matches[2]) == 0 {
				// request whole file
				end = -1
			} else if end, err = strconv.ParseInt(matches[2], 10, 64); err == nil {
				if start > end {
					return nil, fmt.Errorf("invalid content range %q", contentRange)
				}
			} else {
				return nil, fmt.Errorf("invalid content range %q: %w", contentRange, err)
			}
		}
	}

	if resp, err = rt.inner.RoundTrip(in); err != nil {
		return resp, err
	}

	if in.Method != http.MethodGet {
		return resp, nil
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if end != 0 {
			// request range content, but unexpected status code, cant not resume for this request
			return resp, nil
		}

		total = resp.ContentLength
	case http.StatusPartialContent:
		// keep original response status code, which should be processed by original transport or operation
		if start, _, total, err = parseContentRange(resp.Header.Get("Content-Range")); err != nil || total <= 0 {
			return resp, nil
		} else if end > 0 {
			total = end + 1
		}
	default:
		return resp, nil
	}

	if total > 0 {
		resp.Body = &resumableBody{
			rc:          resp.Body,
			inner:       rt.inner,
			req:         in,
			total:       total,
			transferred: start,
			backoff:     rt.backoff,
		}
	}

	return resp, nil
}

type resumableBody struct {
	rc io.ReadCloser

	inner http.RoundTripper
	req   *http.Request

	backoff Backoff

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

	for {
		if n, err = rb.rc.Read(p); n > 0 {
			if rb.transferred+int64(n) >= rb.total {
				n = int(rb.total - rb.transferred)
				err = io.EOF
			}
			rb.transferred += int64(n)
		}

		if err == nil {
			return
		}

		if errors.Is(err, io.EOF) && rb.total >= 0 && rb.transferred >= rb.total {
			return
		}

		if err = rb.resume(rb.backoff, err); err == nil {
			if n == 0 {
				// zero bytes read, try reading again with new response.Body
				continue
			}

			// already read some bytes from previous response.Body, returns and waits for next Read operation
		}

		return n, err
	}
}

func (rb *resumableBody) Close() (err error) {
	if !atomic.CompareAndSwapUint32(&rb.closed, 0, 1) {
		return nil
	}

	return rb.rc.Close()
}

func (rb *resumableBody) resume(backoff Backoff, reason error) error {
	if backoff.Steps <= 0 {
		// resumable transport is disabled
		return reason
	}

	if reason != nil {
		logs.Debug.Printf("Resume http transporting from error: %v", reason)
	}

	var (
		resp *http.Response
		err  error
	)

	for backoff.Steps > 0 {
		time.Sleep(backoff.Step())

		ctx := rb.req.Context()
		select {
		case <-ctx.Done():
			// context already done, stop resuming from error
			return ctx.Err()
		default:
		}

		req := rb.req.Clone(ctx)
		req.Header.Set("Range", "bytes="+strconv.FormatInt(rb.transferred, 10)+"-")
		if resp, err = rb.inner.RoundTrip(req); err != nil {
			err = fmt.Errorf("unable to resume from '%v', %w", reason, err)
			continue
		}

		if err = rb.validate(resp); err != nil {
			resp.Body.Close()
			// wraps original error
			return fmt.Errorf("%w, %v", reason, err)
		}

		if atomic.LoadUint32(&rb.closed) == 1 {
			resp.Body.Close()
			return http.ErrBodyReadAfterClose
		}

		rb.rc.Close()
		rb.rc = resp.Body

		break
	}

	return err
}

const size100m = 100 << 20

func (rb *resumableBody) validate(resp *http.Response) (err error) {
	var start, total int64
	switch resp.StatusCode {
	case http.StatusPartialContent:
		// donot using total size from Content-Range header, keep rb.total unchanged
		if start, _, _, err = parseContentRange(resp.Header.Get("Content-Range")); err != nil {
			return err
		}

		if start == rb.transferred {
			break
		} else if start < rb.transferred {
			// incoming data is overlapped for somehow, just discard it
			if _, err := io.CopyN(io.Discard, resp.Body, rb.transferred-start); err != nil {
				return fmt.Errorf("discard overlapped data failed, %v", err)
			}
		} else {
			return fmt.Errorf("unexpected resume start %d, wanted: %d", start, rb.transferred)
		}
	case http.StatusOK:
		if rb.transferred > 0 {
			// range is not supported, and transferred data is too large, stop resuming
			if rb.transferred > size100m {
				return fmt.Errorf("too large data transferred: %d", rb.transferred)
			}

			// try resume from unsupported range request
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
