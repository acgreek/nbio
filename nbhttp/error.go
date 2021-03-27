// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package nbhttp

import (
	"errors"
)

var (
	// ErrInvalidCRLF .
	ErrInvalidCRLF = errors.New("invalid cr/lf at the end of line")

	// ErrInvalidHTTPVersion .
	ErrInvalidHTTPVersion = errors.New("invalid HTTP version")

	// ErrInvalidHTTPStatusCode .
	ErrInvalidHTTPStatusCode = errors.New("invalid HTTP status code")
	// ErrInvalidHTTPStatus .
	ErrInvalidHTTPStatus = errors.New("invalid HTTP status")

	// ErrInvalidMethod .
	ErrInvalidMethod = errors.New("invalid HTTP method")

	// ErrInvalidRequestURI .
	ErrInvalidRequestURI = errors.New("invalid URL")

	// ErrInvalidHost .
	ErrInvalidHost = errors.New("invalid host")

	// ErrInvalidPort .
	ErrInvalidPort = errors.New("invalid port")

	// ErrInvalidPath .
	ErrInvalidPath = errors.New("invalid path")

	// ErrInvalidQueryString .
	ErrInvalidQueryString = errors.New("invalid query string")

	// ErrInvalidFragment .
	ErrInvalidFragment = errors.New("invalid fragment")

	// ErrCRExpected .
	ErrCRExpected = errors.New("CR character expected")

	// ErrLFExpected .
	ErrLFExpected = errors.New("LF character expected")

	// ErrInvalidCharInHeader .
	ErrInvalidCharInHeader = errors.New("invalid character in header")

	// ErrUnexpectedContentLength .
	ErrUnexpectedContentLength = errors.New("unexpected content-length header")

	// ErrInvalidContentLength .
	ErrInvalidContentLength = errors.New("invalid ContentLength")

	// ErrInvalidChunkSize .
	ErrInvalidChunkSize = errors.New("invalid chunk size")

	// ErrTrailerExpected .
	ErrTrailerExpected = errors.New("trailer expected")

	// ErrTooLong .
	ErrTooLong = errors.New("invalid http message: too long")
)

var (
	// ErrInvalidH2SM .
	ErrInvalidH2SM = errors.New("invalid http2 SM characters")

	// ErrInvalidH2HeaderR .
	ErrInvalidH2HeaderR = errors.New("invalid http2 SM characters")
)

var (
	// ErrNilConn .
	ErrNilConn = errors.New("nil Conn")
)

const (
	// The associated condition is not a result of an error. For example, a GOAWAY might include this code to indicate graceful shutdown of a connection.
	H2_ERRCODE_NO_ERROR = 0x0

	// The endpoint detected an unspecific protocol error. This error is for use when a more specific error code is not available.
	H2_ERRCODE_PROTOCOL_ERROR = 0x1

	// The endpoint encountered an unexpected internal error.
	H2_ERRCODE_INTERNAL_ERROR = 0x2

	// The endpoint detected that its peer violated the flow-control protocol.
	H2_ERRCODE_FLOW_CONTROL_ERROR = 0x3

	// The endpoint sent a SETTINGS frame but did not receive a response in a timely manner. See Section 6.5.3 ("Settings Synchronization").
	H2_ERRCODE_SETTINGS_TIMEOUT = 0x4

	// The endpoint received a frame after a stream was half-closed.
	H2_ERRCODE_STREAM_CLOSED = 0x5

	// The endpoint received a frame with an invalid size.
	H2_ERRCODE_FRAME_SIZE_ERROR = 0x6

	// The endpoint refused the stream prior to performing any application processing (see Section 8.1.4 for details).
	H2_ERRCODE_REFUSED_STREAM = 0x7

	// Used by the endpoint to indicate that the stream is no longer needed.
	H2_ERRCODE_CANCEL = 0x8

	// The endpoint is unable to maintain the header compression context for the connection.
	H2_ERRCODE_COMPRESSION_ERROR = 0x9

	// The connection established in response to a CONNECT request (Section 8.3) was reset or abnormally closed.
	H2_ERRCODE_CONNECT_ERROR = 0xa

	// The endpoint detected that its peer is exhibiting a behavior that might be generating excessive load.
	H2_ERRCODE_ENHANCE_YOUR_CALM = 0xb

	// The underlying transport has properties that do not meet minimum security requirements (see Section 9.2).
	H2_ERRCODE_INADEQUATE_SECURITY = 0xc

	// The endpoint requires that HTTP/1.1 be used instead of HTTP/2.
	H2_ERRCODE_HTTP_1_1_REQUIRED = 0xd

	// Unknown or unsupported error codes MUST NOT trigger any special behavior. These MAY be treated by an implementation as being equivalent to INTERNAL_ERROR.
)
