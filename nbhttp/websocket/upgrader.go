package websocket

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lesismal/nbio"
	"github.com/lesismal/nbio/mempool"
	"github.com/lesismal/nbio/nbhttp"
)

const (
	stateMessageHeader = 0
	stateMessageBody   = 1
)

// Hijacker .
type Hijacker interface {
	Hijack() (net.Conn, error)
}

// Upgrader .
type Upgrader struct {
	state int

	EnableCompression bool

	HandshakeTimeout time.Duration

	Subprotocols []string

	CheckOrigin func(r *http.Request) bool

	opcode  int8
	buffer  []byte
	message []byte
}

func (u *Upgrader) HandleMessage() {
	fmt.Printf("+++ HandleMessage, opcode: %v, message: %v\n", u.opcode, len(u.message))
	mempool.Free(u.message)
	u.message = nil
	u.opcode = -1
}

// Read .
func (u *Upgrader) Read(p *nbhttp.Parser, data []byte) error {
	// fmt.Printf("Upgrader Read: %v\n", string(data))
	l := len(u.buffer)
	if l > 0 {
		u.buffer = mempool.Realloc(u.buffer, l+len(data))
		copy(u.buffer[l:], data)
	} else {
		u.buffer = data
	}
	fmt.Printf("-------- Read, buffer[0]: %b, buffer[1]: %b, buffer[2]: %b, buffer[3]: %b\n", u.buffer[0], u.buffer[1], u.buffer[2], u.buffer[3])
	buffer := u.buffer
	for {
		opcode, body, ok, fin := u.nextFrame()
		if ok {
			bl := len(body)
			if bl > 0 {
				ml := len(u.message)
				if ml == 0 {
					if bl < 1024 {
						u.message = mempool.Malloc(1024)[:bl]
					} else {
						u.message = mempool.Malloc(bl)
					}
				} else {
					rl := ml + len(body)
					if rl < 1024 {
						u.message = mempool.Realloc(u.message, 1024)[:rl]
					} else {
						u.message = mempool.Realloc(u.message, rl)
					}
				}
				copy(u.message[ml:], body)

				if u.opcode == -1 {
					u.opcode = opcode
				}
			}
		} else {
			break
		}

		if fin {
			u.HandleMessage()
			// defer func() {
			// 	fmt.Println("+++ Read Over:", len(u.message))
			// }()
		}

		if len(u.buffer) == 0 {
			break
		}
	}

	l = len(u.buffer)
	if l != len(buffer) {
		if l > 0 {
			var tmp []byte
			if l < 2048 {
				tmp = mempool.Malloc(2048)[:l]
			} else {
				tmp = mempool.Malloc(l)
			}
			copy(tmp, u.buffer)
			u.buffer = tmp
		}
		mempool.Free(buffer)
	}

	return nil
}

func (u *Upgrader) Fin() bool {
	return (u.buffer[0] & 0x1) != 0
}

func (u *Upgrader) RSV1() bool {
	return (u.buffer[0] & 0x2) != 0
}

func (u *Upgrader) RSV2() bool {
	return (u.buffer[0] & 0x4) != 0
}

func (u *Upgrader) RSV3() bool {
	return (u.buffer[0] & 0x8) != 0
}

func (u *Upgrader) nextFrame() (int8, []byte, bool, bool) {
	var (
		ok     bool   = false
		fin    bool   = false
		body   []byte = nil
		opcode int8   = -1
	)
	l := int64(len(u.buffer))
	headLen := int64(2)
	if l >= 2 {
		payloadLen := u.buffer[1] & 0x7F
		bodyLen := int64(-1)

		switch payloadLen {
		case 126:
			if l >= 4 {
				bodyLen = int64(binary.BigEndian.Uint16(u.buffer[2:4]))
				headLen = 4
			}
		case 127:
			if len(u.buffer) >= 10 {
				bodyLen = int64(binary.BigEndian.Uint64(u.buffer[2:10]))
				headLen = 10
			}
		default:
			bodyLen = int64(payloadLen)
		}
		if bodyLen >= 0 {
			masked := (u.buffer[1] & 0x80) != 0
			if masked {
				headLen += 4
			}
			total := headLen + bodyLen
			if l >= total {
				body = u.buffer[headLen:total]
				if masked {
					mask := u.buffer[headLen-4 : headLen]
					for i := 0; i < len(body); i++ {
						body[i] ^= mask[i%4]
					}
				} else {

				}
				opcode = int8(u.buffer[0] & 0xF)
				ok = true
				fin = ((u.buffer[0] & 0x80) != 0)
				u.buffer = u.buffer[total:]
				// fmt.Println("body:", string(body))
				// fmt.Println("fin:", fin)
			}
		}
	}

	return opcode, body, ok, fin
}

// Upgrade .
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) error {
	const badHandshake = "websocket: the client is not using the websocket protocol: "

	if !headerContains(r.Header, "Connection", "upgrade") {
		return u.returnError(w, r, http.StatusBadRequest, ErrUpgradeTokenNotFound)
	}

	if !headerContains(r.Header, "Upgrade", "websocket") {
		return u.returnError(w, r, http.StatusBadRequest, ErrUpgradeTokenNotFound)
	}

	if r.Method != "GET" {
		return u.returnError(w, r, http.StatusMethodNotAllowed, ErrUpgradeMethodIsGet)
	}

	if !headerContains(r.Header, "Sec-Websocket-Version", "13") {
		return u.returnError(w, r, http.StatusBadRequest, ErrUpgradeInvalidWebsocketVersion)
	}

	if _, ok := responseHeader["Sec-Websocket-Extensions"]; ok {
		return u.returnError(w, r, http.StatusInternalServerError, ErrUpgradeUnsupportedExtensions)
	}

	checkOrigin := u.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = checkSameOrigin
	}
	if !checkOrigin(r) {
		return u.returnError(w, r, http.StatusForbidden, ErrUpgradeOriginNotAllowed)
	}

	challengeKey := r.Header.Get("Sec-Websocket-Key")
	if challengeKey == "" {
		return u.returnError(w, r, http.StatusBadRequest, ErrUpgradeMissingWebsocketKey)
	}

	subprotocol := u.selectSubprotocol(r, responseHeader)

	// Negotiate PMCE
	var compress bool
	if u.EnableCompression {
		for _, ext := range parseExtensions(r.Header) {
			if ext[""] != "permessage-deflate" {
				continue
			}
			compress = true
			break
		}
	}

	h, ok := w.(nbhttp.Hijacker)
	if !ok {
		return u.returnError(w, r, http.StatusInternalServerError, ErrUpgradeNotHijacker)
	}
	conn, err := h.Hijack()
	if err != nil {
		return u.returnError(w, r, http.StatusInternalServerError, err)
	}

	nbc, ok := conn.(*nbio.Conn)
	if !ok {
		return u.returnError(w, r, http.StatusInternalServerError, err)
	}
	parser, ok := nbc.Session().(*nbhttp.Parser)
	if !ok {
		return u.returnError(w, r, http.StatusInternalServerError, err)
	}

	parser.Upgrader = u

	wsConn := newConn(conn, compress, subprotocol)

	w.WriteHeader(http.StatusSwitchingProtocols)
	w.Header().Add("Upgrade", "websocket")
	w.Header().Add("Connection", "Upgrade")
	w.Header().Add("Sec-WebSocket-Accept", string(computeAcceptKey(challengeKey)))
	if subprotocol != "" {
		w.Header().Add("Sec-WebSocket-Protocol", subprotocol)
	}
	if compress {
		w.Header().Add("Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
	}
	for k, vv := range responseHeader {
		if k != "Sec-Websocket-Protocol" {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
	}

	// var p []byte
	// p = append(p, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: "...)
	// p = append(p, computeAcceptKey(challengeKey)...)
	// p = append(p, "\r\n"...)
	// if subprotocol != "" {
	// 	p = append(p, "Sec-WebSocket-Protocol: "...)
	// 	p = append(p, subprotocol...)
	// 	p = append(p, "\r\n"...)
	// }
	// if compress {
	// 	p = append(p, "Sec-WebSocket-Extensions: permessage-deflate; server_no_context_takeover; client_no_context_takeover\r\n"...)
	// }
	// for k, vs := range responseHeader {
	// 	if k == "Sec-Websocket-Protocol" {
	// 		continue
	// 	}
	// 	for _, v := range vs {
	// 		p = append(p, k...)
	// 		p = append(p, ": "...)
	// 		for i := 0; i < len(v); i++ {
	// 			b := v[i]
	// 			if b <= 31 {
	// 				// prevent response splitting.
	// 				b = ' '
	// 			}
	// 			p = append(p, b)
	// 		}
	// 		p = append(p, "\r\n"...)
	// 	}
	// }
	// p = append(p, "\r\n"...)
	if u.HandshakeTimeout > 0 {
		wsConn.SetWriteDeadline(time.Now().Add(u.HandshakeTimeout))
	}
	// ps := string(p)
	// p = []byte(ps)
	// fmt.Println("ps:")
	// fmt.Println(ps)
	// if _, err = wsConn.Write(p); err != nil {
	// 	wsConn.Close()
	// 	return err
	// }

	return nil
}

// OnClose .
func (u *Upgrader) OnClose(p *nbhttp.Parser, err error) {
	fmt.Println("onClose:", p.Processor.Conn().RemoteAddr().String())
}

func NewUpgrader() *Upgrader {
	return &Upgrader{opcode: -1}
}

func (u *Upgrader) returnError(w http.ResponseWriter, r *http.Request, status int, err error) error {
	w.Header().Set("Sec-Websocket-Version", "13")
	http.Error(w, http.StatusText(status), status)
	return err
}

func Subprotocols(r *http.Request) []string {
	h := strings.TrimSpace(r.Header.Get("Sec-Websocket-Protocol"))
	if h == "" {
		return nil
	}
	protocols := strings.Split(h, ",")
	for i := range protocols {
		protocols[i] = strings.TrimSpace(protocols[i])
	}
	return protocols
}

func (u *Upgrader) selectSubprotocol(r *http.Request, responseHeader http.Header) string {
	if u.Subprotocols != nil {
		clientProtocols := Subprotocols(r)
		for _, serverProtocol := range u.Subprotocols {
			for _, clientProtocol := range clientProtocols {
				if clientProtocol == serverProtocol {
					return clientProtocol
				}
			}
		}
	} else if responseHeader != nil {
		return responseHeader.Get("Sec-Websocket-Protocol")
	}
	return ""
}

var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func computeAcceptKey(challengeKey string) string {
	h := sha1.New()
	h.Write([]byte(challengeKey))
	h.Write(keyGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// checkSameOrigin returns true if the origin is not set or is equal to the request host.
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header["Origin"]
	if len(origin) == 0 {
		return true
	}
	u, err := url.Parse(origin[0])
	if err != nil {
		return false
	}
	return equalASCIIFold(u.Host, r.Host)
}

func headerContains(header http.Header, name string, value string) bool {
	value = strings.ToLower(value)
	for _, v := range header[name] {
		if strings.ToLower(v) == value {
			return true
		}
	}
	return false
}

func equalASCIIFold(s, t string) bool {
	for s != "" && t != "" {
		sr, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		tr, size := utf8.DecodeRuneInString(t)
		t = t[size:]
		if sr == tr {
			continue
		}
		if 'A' <= sr && sr <= 'Z' {
			sr = sr + 'a' - 'A'
		}
		if 'A' <= tr && tr <= 'Z' {
			tr = tr + 'a' - 'A'
		}
		if sr != tr {
			return false
		}
	}
	return s == t
}

func nextToken(s string) (token, rest string) {
	i := 0
	for ; i < len(s); i++ {
		if !isTokenOctet[s[i]] {
			break
		}
	}
	return s[:i], s[i:]
}
func skipSpace(s string) (rest string) {
	i := 0
	for ; i < len(s); i++ {
		if b := s[i]; b != ' ' && b != '\t' {
			break
		}
	}
	return s[i:]
}

// parseExtensions parses WebSocket extensions from a header.
func parseExtensions(header http.Header) []map[string]string {
	var result []map[string]string
headers:
	for _, s := range header["Sec-Websocket-Extensions"] {
		for {
			var t string
			t, s = nextToken(skipSpace(s))
			if t == "" {
				continue headers
			}
			ext := map[string]string{"": t}
			for {
				s = skipSpace(s)
				if !strings.HasPrefix(s, ";") {
					break
				}
				var k string
				k, s = nextToken(skipSpace(s[1:]))
				if k == "" {
					continue headers
				}
				s = skipSpace(s)
				var v string
				if strings.HasPrefix(s, "=") {
					v, s = nextTokenOrQuoted(skipSpace(s[1:]))
					s = skipSpace(s)
				}
				if s != "" && s[0] != ',' && s[0] != ';' {
					continue headers
				}
				ext[k] = v
			}
			if s != "" && s[0] != ',' {
				continue headers
			}
			result = append(result, ext)
			if s == "" {
				continue headers
			}
			s = s[1:]
		}
	}
	return result
}

func nextTokenOrQuoted(s string) (value string, rest string) {
	if !strings.HasPrefix(s, "\"") {
		return nextToken(s)
	}
	s = s[1:]
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			return s[:i], s[i+1:]
		case '\\':
			p := make([]byte, len(s)-1)
			j := copy(p, s[:i])
			escape := true
			for i = i + 1; i < len(s); i++ {
				b := s[i]
				switch {
				case escape:
					escape = false
					p[j] = b
					j++
				case b == '\\':
					escape = true
				case b == '"':
					return string(p[:j]), s[i+1:]
				default:
					p[j] = b
					j++
				}
			}
			return "", ""
		}
	}
	return "", ""
}

// Token octets per RFC 2616.
var isTokenOctet = [256]bool{
	'!':  true,
	'#':  true,
	'$':  true,
	'%':  true,
	'&':  true,
	'\'': true,
	'*':  true,
	'+':  true,
	'-':  true,
	'.':  true,
	'0':  true,
	'1':  true,
	'2':  true,
	'3':  true,
	'4':  true,
	'5':  true,
	'6':  true,
	'7':  true,
	'8':  true,
	'9':  true,
	'A':  true,
	'B':  true,
	'C':  true,
	'D':  true,
	'E':  true,
	'F':  true,
	'G':  true,
	'H':  true,
	'I':  true,
	'J':  true,
	'K':  true,
	'L':  true,
	'M':  true,
	'N':  true,
	'O':  true,
	'P':  true,
	'Q':  true,
	'R':  true,
	'S':  true,
	'T':  true,
	'U':  true,
	'W':  true,
	'V':  true,
	'X':  true,
	'Y':  true,
	'Z':  true,
	'^':  true,
	'_':  true,
	'`':  true,
	'a':  true,
	'b':  true,
	'c':  true,
	'd':  true,
	'e':  true,
	'f':  true,
	'g':  true,
	'h':  true,
	'i':  true,
	'j':  true,
	'k':  true,
	'l':  true,
	'm':  true,
	'n':  true,
	'o':  true,
	'p':  true,
	'q':  true,
	'r':  true,
	's':  true,
	't':  true,
	'u':  true,
	'v':  true,
	'w':  true,
	'x':  true,
	'y':  true,
	'z':  true,
	'|':  true,
	'~':  true,
}
