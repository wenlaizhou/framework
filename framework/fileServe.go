package framework

import (
	"mime"
	"path/filepath"
	"mime/multipart"
	"io"
	"time"
	"os"
	"net/textproto"
	"net/http"
	"strings"
	"fmt"
	"errors"
	"strconv"
)

const TimeFormat = "Mon, 02 Jan 2006 15:04:05 GMT"
const sniffLen = 512

// if name is empty, filename is unknown. (used for mime type, before sniffing)
// if modtime.IsZero(), modtime is unknown.
// content must be seeked to the beginning of the file.
// The sizeFunc is called at most once. Its error, if any, is sent in the HTTP response.
func serveContent(context Context, name string, modtime time.Time, sizeFunc func() (int64, error), content io.ReadSeeker) {
	setLastModified(context, modtime)
	done, rangeReq := checkPreconditions(context, modtime)
	if done {
		return
	}

	code := StatusOK

	// If Content-Type isn't set, use the file's extension to find it, but
	// if the Content-Type is unset explicitly, do not sniff the type.
	ctypes, haveType := context.Request.Header["Content-Type"]
	var ctype string
	if !haveType {
		ctype = mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			// read a chunk to decide between utf-8 text and binary
			var buf [sniffLen]byte
			n, _ := io.ReadFull(content, buf[:])
			ctype = http.DetectContentType(buf[:n])
			_, err := content.Seek(0, io.SeekStart) // rewind to output whole file
			if err != nil {
				context.Error(StatusInternalServerError,
					fmt.Sprintf(StatusErrorTemp, StatusInternalServerError,
						"seeker can't seek"))
				return
			}
		}
		context.SetHeader("Content-Type", ctype)
	} else if len(ctypes) > 0 {
		ctype = ctypes[0]
	}

	size, err := sizeFunc()
	if err != nil {
		context.Error(StatusInternalServerError,
			fmt.Sprintf(StatusErrorTemp,
				StatusInternalServerError, err.Error()))
		return
	}

	// handle Content-Range header.
	sendSize := size
	var sendContent io.Reader = content
	if size >= 0 {
		ranges, err := parseRange(rangeReq, size)
		if err != nil {
			if err == errNoOverlap {
				context.SetHeader("Content-Range", fmt.Sprintf("bytes */%d", size))
			}
			context.Error(StatusRequestedRangeNotSatisfiable,
				fmt.Sprintf(StatusErrorTemp,
					StatusRequestedRangeNotSatisfiable, err.Error()))
			return
		}
		if sumRangesSize(ranges) > size {
			// The total number of bytes in all the ranges
			// is larger than the size of the file by
			// itself, so this is probably an attack, or a
			// dumb client. Ignore the range request.
			ranges = nil
		}
		switch {
		case len(ranges) == 1:
			// RFC 2616, Section 14.16:
			// "When an HTTP message includes the content of a single
			// range (for example, a response to a request for a
			// single range, or to a request for a set of ranges
			// that overlap without any holes), this content is
			// transmitted with a Content-Range header, and a
			// Content-Length header showing the number of bytes
			// actually transferred.
			// ...
			// A response to a request for a single range MUST NOT
			// be sent using the multipart/byteranges media type."
			ra := ranges[0]
			if _, err := content.Seek(ra.start, io.SeekStart); err != nil {
				context.Error(StatusRequestedRangeNotSatisfiable,
					fmt.Sprintf(StatusErrorTemp,
						StatusRequestedRangeNotSatisfiable, err.Error()))
				return
			}
			sendSize = ra.length
			code = StatusPartialContent
			context.SetHeader("Content-Range", ra.contentRange(size))
		case len(ranges) > 1:
			sendSize = rangesMIMESize(ranges, ctype, size)
			code = StatusPartialContent

			pr, pw := io.Pipe()
			mw := multipart.NewWriter(pw)
			context.SetHeader("Content-Type", "multipart/byteranges; boundary="+mw.Boundary())
			sendContent = pr
			defer pr.Close() // cause writing goroutine to fail and exit if CopyN doesn't finish.
			go func() {
				for _, ra := range ranges {
					part, err := mw.CreatePart(ra.mimeHeader(ctype, size))
					if err != nil {
						pw.CloseWithError(err)
						return
					}
					if _, err := content.Seek(ra.start, io.SeekStart); err != nil {
						pw.CloseWithError(err)
						return
					}
					if _, err := io.CopyN(part, content, ra.length); err != nil {
						pw.CloseWithError(err)
						return
					}
				}
				mw.Close()
				pw.Close()
			}()
		}

		context.SetHeader("Accept-Ranges", "bytes")
		if context.GetHeader("Content-Encoding") == "" {
			context.SetHeader("Content-Length", strconv.FormatInt(sendSize, 10))
		}
	}

	context.Code(code)

	if context.GetMethod() != "HEAD" {
		io.CopyN(context.Response, sendContent, sendSize)
	}
}

func FileProcessor(context Context) {

	fileName := context.Request.RequestURI[1:]
	f, err := os.Open(fileName)
	if ProcessError(err) {
		msg, code := toHTTPError(err)
		context.Error(code, msg)
		return
	}
	defer f.Close()
	d, err := f.Stat()
	if ProcessError(err) {
		msg, code := toHTTPError(err)
		context.Error(code, msg)
		return
	}
	// Still a directory? (we didn't find an index.html file)
	if d.IsDir() { //golang原处理逻辑为目录列表
		//if checkIfModifiedSince(context, d.ModTime()) == condFalse {
		//	writeNotModified(context)
		//	return
		//}
		//context.SetHeader("Last-Modified", d.ModTime().UTC().Format(TimeFormat))
		//dirList(w, r, f)

		context.Error(StatusNotFound, StatusNotFoundView)
		return
	}

	// serveContent will check modification time
	sizeFunc := func() (int64, error) { return d.Size(), nil }
	serveContent(context, d.Name(), d.ModTime(), sizeFunc, f)
}

// toHTTPError returns a non-specific HTTP error message and status code
// for a given non-nil error value. It's important that toHTTPError does not
// actually return err.Error(), since msg and httpStatus are returned to users,
// and historically Go's ServeContent always returned just "404 Not Found" for
// all errors. We don't want to start leaking information in error messages.
func toHTTPError(err error) (msg string, httpStatus int) {
	if os.IsNotExist(err) {
		return "404 page not found", StatusNotFound
	}
	if os.IsPermission(err) {
		return "403 Forbidden", StatusForbidden
	}
	// Default:
	return "500 Internal Server Error", StatusInternalServerError
}

// condResult is the result of an HTTP request precondition check.
// See https://tools.ietf.org/html/rfc7232 section 3.
type condResult int

const (
	condNone  condResult = iota
	condTrue
	condFalse
)

// checkPreconditions evaluates request preconditions and reports whether a precondition
// resulted in sending StatusNotModified or StatusPreconditionFailed.
func checkPreconditions(context Context, modtime time.Time) (done bool, rangeHeader string) {
	// This function carefully follows RFC 7232 section 6.
	ch := checkIfMatch(context)
	if ch == condNone {
		ch = checkIfUnmodifiedSince(context, modtime)
	}
	if ch == condFalse {
		context.Code(StatusPreconditionFailed)
		return true, ""
	}
	switch checkIfNoneMatch(context) {
	case condFalse:
		if context.GetMethod() == "GET" || context.GetMethod() == "HEAD" {
			writeNotModified(context)
			return true, ""
		} else {
			context.Code(StatusPreconditionFailed)
			return true, ""
		}
	case condNone:
		if checkIfModifiedSince(context, modtime) == condFalse {
			writeNotModified(context)
			return true, ""
		}
	}

	rangeHeader = context.GetHeader("Range")
	if rangeHeader != "" && checkIfRange(context, modtime) == condFalse {
		rangeHeader = ""
	}
	return false, rangeHeader
}

func checkIfMatch(context Context) condResult {
	im := context.GetHeader("If-Match")
	if im == "" {
		return condNone
	}
	for {
		im = textproto.TrimString(im)
		if len(im) == 0 {
			break
		}
		if im[0] == ',' {
			im = im[1:]
			continue
		}
		if im[0] == '*' {
			return condTrue
		}
		etag, remain := scanETag(im)
		if etag == "" {
			break
		}
		if etagStrongMatch(etag, context.GetHeader("Etag")) {
			return condTrue
		}
		im = remain
	}

	return condFalse
}

// scanETag determines if a syntactically valid ETag is present at s. If so,
// the ETag and remaining text after consuming ETag is returned. Otherwise,
// it returns "", "".
func scanETag(s string) (etag string, remain string) {
	s = textproto.TrimString(s)
	start := 0
	if strings.HasPrefix(s, "W/") {
		start = 2
	}
	if len(s[start:]) < 2 || s[start] != '"' {
		return "", ""
	}
	// ETag is either W/"text" or "text".
	// See RFC 7232 2.3.
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		switch {
		// Character values allowed in ETags.
		case c == 0x21 || c >= 0x23 && c <= 0x7E || c >= 0x80:
		case c == '"':
			return s[:i+1], s[i+1:]
		default:
			return "", ""
		}
	}
	return "", ""
}

// etagStrongMatch reports whether a and b match using strong ETag comparison.
// Assumes a and b are valid ETags.
func etagStrongMatch(a, b string) bool {
	return a == b && a != "" && a[0] == '"'
}

// etagWeakMatch reports whether a and b match using weak ETag comparison.
// Assumes a and b are valid ETags.
func etagWeakMatch(a, b string) bool {
	return strings.TrimPrefix(a, "W/") == strings.TrimPrefix(b, "W/")
}

func checkIfUnmodifiedSince(context Context, modtime time.Time) condResult {
	ius := context.GetHeader("If-Unmodified-Since")
	if ius == "" || isZeroTime(modtime) {
		return condNone
	}
	if t, err := http.ParseTime(ius); err == nil {
		// The Date-Modified header truncates sub-second precision, so
		// use mtime < t+1s instead of mtime <= t to check for unmodified.
		if modtime.Before(t.Add(1 * time.Second)) {
			return condTrue
		}
		return condFalse
	}
	return condNone
}

func checkIfNoneMatch(context Context) condResult {
	inm := context.GetHeader("If-None-Match")
	if inm == "" {
		return condNone
	}
	buf := inm
	for {
		buf = textproto.TrimString(buf)
		if len(buf) == 0 {
			break
		}
		if buf[0] == ',' {
			buf = buf[1:]
		}
		if buf[0] == '*' {
			return condFalse
		}
		etag, remain := scanETag(buf)
		if etag == "" {
			break
		}
		if etagWeakMatch(etag, context.GetHeader("Etag")) {
			return condFalse
		}
		buf = remain
	}
	return condTrue
}

func checkIfModifiedSince(context Context, modtime time.Time) condResult {
	if context.GetMethod() != "GET" && context.GetMethod() != "HEAD" {
		return condNone
	}
	ims := context.GetHeader("If-Modified-Since")
	if ims == "" || isZeroTime(modtime) {
		return condNone
	}
	t, err := http.ParseTime(ims)
	if err != nil {
		return condNone
	}
	// The Date-Modified header truncates sub-second precision, so
	// use mtime < t+1s instead of mtime <= t to check for unmodified.
	if modtime.Before(t.Add(1 * time.Second)) {
		return condFalse
	}
	return condTrue
}

func checkIfRange(context Context, modtime time.Time) condResult {
	if context.GetMethod() != "GET" && context.GetMethod() != "HEAD" {
		return condNone
	}
	ir := context.GetHeader("If-Range")
	if ir == "" {
		return condNone
	}
	etag, _ := scanETag(ir)
	if etag != "" {
		if etagStrongMatch(etag, context.GetHeader("Etag")) {
			return condTrue
		} else {
			return condFalse
		}
	}
	// The If-Range value is typically the ETag value, but it may also be
	// the modtime date. See golang.org/issue/8367.
	if modtime.IsZero() {
		return condFalse
	}
	t, err := http.ParseTime(ir)
	if err != nil {
		return condFalse
	}
	if t.Unix() == modtime.Unix() {
		return condTrue
	}
	return condFalse
}

var unixEpochTime = time.Unix(0, 0)

// isZeroTime reports whether t is obviously unspecified (either zero or Unix()=0).
func isZeroTime(t time.Time) bool {
	return t.IsZero() || t.Equal(unixEpochTime)
}

func setLastModified(context Context, modtime time.Time) {
	if !isZeroTime(modtime) {
		context.SetHeader("Last-Modified", modtime.UTC().Format(TimeFormat))
	}
}

func writeNotModified(context Context) {
	// RFC 7232 section 4.1:
	// a sender SHOULD NOT generate representation metadata other than the
	// above listed fields unless said metadata exists for the purpose of
	// guiding cache updates (e.g., Last-Modified might be useful if the
	// response does not have an ETag field).
	context.DelHeader("Content-Type")
	context.DelHeader("Content-Length")
	if context.GetHeader("Etag") != "" {
		context.DelHeader("Last-Modified")
	}
	context.Code(StatusNotModified)
}

// parseRange parses a Range header string as per RFC 2616.
// errNoOverlap is returned if none of the ranges overlap.
func parseRange(s string, size int64) ([]httpRange, error) {
	if s == "" {
		return nil, nil // header not present
	}
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid range")
	}
	var ranges []httpRange
	noOverlap := false
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = strings.TrimSpace(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return nil, errors.New("invalid range")
		}
		start, end := strings.TrimSpace(ra[:i]), strings.TrimSpace(ra[i+1:])
		var r httpRange
		if start == "" {
			// If no start is specified, end specifies the
			// range start relative to the end of the file.
			i, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, errors.New("invalid range")
			}
			if i > size {
				i = size
			}
			r.start = size - i
			r.length = size - r.start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i < 0 {
				return nil, errors.New("invalid range")
			}
			if i >= size {
				// If the range begins after the size of the content,
				// then it does not overlap.
				noOverlap = true
				continue
			}
			r.start = i
			if end == "" {
				// If no end is specified, range extends to end of the file.
				r.length = size - r.start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r.start > i {
					return nil, errors.New("invalid range")
				}
				if i >= size {
					i = size - 1
				}
				r.length = i - r.start + 1
			}
		}
		ranges = append(ranges, r)
	}
	if noOverlap && len(ranges) == 0 {
		// The specified ranges did not overlap with the content.
		return nil, errNoOverlap
	}
	return ranges, nil
}

// httpRange specifies the byte range to be sent to the client.
type httpRange struct {
	start, length int64
}

// errSeeker is returned by ServeContent's sizeFunc when the content
// doesn't seek properly. The underlying Seeker's error text isn't
// included in the sizeFunc reply so it's not sent over HTTP to end
// users.
var errSeeker = errors.New("seeker can't seek")

// errNoOverlap is returned by serveContent's parseRange if first-byte-pos of
// all of the byte-range-spec values is greater than the content size.
var errNoOverlap = errors.New("invalid range: failed to overlap")

func sumRangesSize(ranges []httpRange) (size int64) {
	for _, ra := range ranges {
		size += ra.length
	}
	return
}

func (r httpRange) contentRange(size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, size)
}

func (r httpRange) mimeHeader(contentType string, size int64) textproto.MIMEHeader {
	return textproto.MIMEHeader{
		"Content-Range": {r.contentRange(size)},
		"Content-Type":  {contentType},
	}
}

// countingWriter counts how many bytes have been written to it.
type countingWriter int64

func (w *countingWriter) Write(p []byte) (n int, err error) {
	*w += countingWriter(len(p))
	return len(p), nil
}

// rangesMIMESize returns the number of bytes it takes to encode the
// provided ranges as a multipart response.
func rangesMIMESize(ranges []httpRange, contentType string, contentSize int64) (encSize int64) {
	var w countingWriter
	mw := multipart.NewWriter(&w)
	for _, ra := range ranges {
		mw.CreatePart(ra.mimeHeader(contentType, contentSize))
		encSize += ra.length
	}
	mw.Close()
	encSize += int64(w)
	return
}
