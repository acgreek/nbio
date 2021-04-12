// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package nbhttp

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	requestPool = sync.Pool{
		New: func() interface{} {
			return &http.Request{}
		},
	}

	responsePool = sync.Pool{
		New: func() interface{} {
			return &Response{}
		},
	}
)

// Response represents the server side of an HTTP response.
type Response struct {
	processor Processor

	index    int
	sequence uint64
	request  *http.Request // request for this response

	statusCode int // status code passed to WriteHeader
	status     string
	header     http.Header
	trailers   http.Header

	bodySize int
	bodyList [][]byte
}

// Hijack .
func (res *Response) Hijack() (net.Conn, error) {
	if res.processor == nil {
		return nil, errors.New("nil Proccessor")
	}
	return res.processor.Conn(), nil
}

// Header .
func (res *Response) Header() http.Header {
	return res.header
}

// WriteHeader .
func (res *Response) WriteHeader(statusCode int) {
	if res.statusCode == 0 {
		status := http.StatusText(statusCode)
		if status != "" {
			res.status = status
			res.statusCode = statusCode
		}
	}
}

// Write .
func (res *Response) Write(data []byte) (int, error) {
	res.WriteHeader(http.StatusOK)
	n := len(data)
	if n > 0 {
		if n <= 4096 {
			res.bodyList = append(res.bodyList, data)
			res.bodySize += len(data)
		} else {
			res.bodySize += len(data)

			n = 4096
			for len(data) > 0 {
				if len(data) < n {
					n = len(data)
				}
				res.bodyList = append(res.bodyList, data[:n])
				data = data[n:]
			}
		}
	}
	return len(data), nil
}

// flush .
func (res *Response) flush(conn net.Conn) error {
	res.WriteHeader(http.StatusOK)
	statusCode := res.statusCode
	status := res.status

	chunked := false
	encodingFound := false
	if res.request.ProtoAtLeast(1, 1) {
		for _, v := range res.header["Transfer-Encoding"] {
			if v == "chunked" {
				chunked = true
				encodingFound = true
			}
		}
		if !chunked {
			if len(res.header["Trailer"]) > 0 {
				chunked = true
			}
		}
	}
	if chunked {
		if !encodingFound {
			hs := res.header["Transfer-Encoding"]
			res.header["Transfer-Encoding"] = append(hs, "chunked")
		}
		delete(res.header, "Content-Length")
	} else if res.bodySize > 0 {
		hs := res.header["Content-Length"]
		res.header["Content-Length"] = append(hs, strconv.Itoa(res.bodySize))
	}

	size := res.bodySize + 1024

	if size < 2048 {
		size = 2048
	}

	data := []byte(res.request.Proto)
	data = append(data, ' ', '0'+byte(statusCode/100), '0'+byte(statusCode%100)/10, '0'+byte(statusCode%10), ' ')
	data = append(data, status...)
	data = append(data, '\r', '\n')

	trailer := map[string]string{}
	for k, vv := range res.header {
		if strings.HasPrefix(k, "Trailer-") {
			if len(vv) > 0 {
				trailer[k] = vv[0]
			}
			continue
		}
		for _, value := range vv {
			data = append(data, k...)
			data = append(data, ':', ' ')
			data = append(data, value...)
			data = append(data, '\r', '\n')
		}
	}

	if len(res.header["Content-Type"]) == 0 {
		const contentType = "Content-Type: text/plain; charset=utf-8\r\n"
		data = append(data, contentType...)
	}

	if len(res.header["Date"]) == 0 {
		const days = "SunMonTueWedThuFriSat"
		const months = "JanFebMarAprMayJunJulAugSepOctNovDec"
		t := time.Now().UTC()
		yy, mm, dd := t.Date()
		hh, mn, ss := t.Clock()
		day := days[3*t.Weekday():]
		mon := months[3*(mm-1):]
		data = append(data,
			'D', 'a', 't', 'e', ':', ' ',
			day[0], day[1], day[2], ',', ' ',
			byte('0'+dd/10), byte('0'+dd%10), ' ',
			mon[0], mon[1], mon[2], ' ',
			byte('0'+yy/1000), byte('0'+(yy/100)%10), byte('0'+(yy/10)%10), byte('0'+yy%10), ' ',
			byte('0'+hh/10), byte('0'+hh%10), ':',
			byte('0'+mn/10), byte('0'+mn%10), ':',
			byte('0'+ss/10), byte('0'+ss%10), ' ',
			'G', 'M', 'T',
			'\r', '\n')
	}
	data = append(data, '\r', '\n')

	if !chunked {
		if res.bodySize == 0 {
			_, err := conn.Write(data)
			return err
		}
		if len(data)+res.bodySize <= 8192 {
			for _, v := range res.bodyList {
				data = append(data, v...)
			}
			_, err := conn.Write(data)
			return err
		} else {
			data = append(data, res.bodyList[0]...)
			_, err := conn.Write(data)
			if err != nil {
				return err
			}
			for i := 1; i < len(res.bodyList); i++ {
				_, err = conn.Write(res.bodyList[i])
				if err != nil {
					return err
				}
			}
		}
	} else {
		for _, v := range res.bodyList {
			lenStr := strconv.FormatInt(int64(len(v)), 16)
			data = append(data, lenStr...)
			data = append(data, '\r', '\n')
			data = append(data, v...)
			data = append(data, '\r', '\n')
			if len(data) > 4096 {
				_, err := conn.Write(data)
				if err != nil {
					return err
				}
				data = nil
			}
		}
		if len(trailer) == 0 {
			data = append(data, '0', '\r', '\n', '\r', '\n')
		} else {
			data = append(data, '0', '\r', '\n')
			for k, v := range trailer {
				data = append(data, k...)
				data = append(data, ':', ' ')
				data = append(data, v...)
				data = append(data, '\r', '\n')
			}
			data = append(data, '\r', '\n')
		}
		_, err := conn.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}

// NewResponse .
func NewResponse(processor Processor, request *http.Request, sequence uint64) *Response {
	response := responsePool.Get().(*Response)
	response.processor = processor
	response.request = request
	response.sequence = sequence
	response.header = http.Header{"Server": []string{"nbio"}}
	return response
}

type responseQueue []*Response

func (h responseQueue) Len() int           { return len(h) }
func (h responseQueue) Less(i, j int) bool { return h[i].sequence < h[j].sequence }
func (h responseQueue) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *responseQueue) Push(x interface{}) {
	*h = append(*h, x.(*Response))
	n := len(*h)
	(*h)[n-1].index = n - 1
}
func (h *responseQueue) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[0 : n-1]
	return x
}
