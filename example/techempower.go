package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/shaovie/goev"
)

var (
	httpRespHeader        []byte
	httpRespContentLength []byte
	ticker                *time.Ticker
	liveDate              atomic.Value
	forNewFdReactor       *goev.Reactor
)

// Launch args
var (
	evPollNum int = runtime.NumCPU()
	procNum   int = runtime.NumCPU() * 2
)

func usage() {
	fmt.Println(`
    Server options:
    -c N                   Evpoll num
    -p N                   PROC num

    Common options:
    -h                     Show this message
    `)
	os.Exit(0)
}
func parseFlag() {
	flag.IntVar(&evPollNum, "c", evPollNum, "evpoll num.")
	flag.IntVar(&procNum, "p", procNum, "proc num.")

	flag.Usage = usage
	flag.Parse()
}

const httpHeaderS = "HTTP/1.1 200 OK\r\nConnection: keep-alive\r\nServer: goev\r\nContent-Type: text/plain\r\nDate: "
const contentLengthS = "\r\nContent-Length: 13\r\n\r\nHello, World!"

type Request struct {
	method        int
	contentLength int
	uri           string
	host          string
	headers       [][2]string
}

type Http struct {
	goev.IOHandle

	partialBuf []byte
}

func (h *Http) OnOpen() bool {
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := forNewFdReactor.AddEvHandler(h, h.Fd(), goev.EvIn); err != nil {
		return false
	}
	return true
}
func (h *Http) parseHeader(buf []byte) (req Request, headerLen int, res int) {
	var bufLen = len(buf)
	// 1. METHOD
	bufOffset := 0
	if bufLen < 18 { // GET / HTTP/1.x\r\nr\n
		res = 1 // partial
		return
	}
	method := 0                                          // 1:get 2:post 3:unsupport
	if buf[0] == 'G' && buf[1] == 'E' && buf[2] == 'T' { // GET
		if buf[3] == ' ' || buf[3] == '\t' {
			method = 1
			bufOffset += 4
		}
	} else if buf[0] == 'P' && buf[1] == 'O' && buf[2] == 'S' && buf[3] == 'T' { // POST
		if buf[4] == ' ' || buf[4] == '\t' {
			method = 2
			bufOffset += 5
		}
	}
	if method == 0 {
		res = -1 // Abnormal connection. close it
		return
	}
	req.method = method

	// 2. URI  /a/b/c?p=x&p2=2#yyy
	if buf[bufOffset] != '/' {
		res = -1 // Abnormal connection. close it
		return
	}
	pos := bytes.IndexByte(buf[bufOffset+1:], ' ') // \t ?
	if pos < 0 {
		res = 1 // partial
		return
	}
	if pos == 0 {
		req.uri = "/"
		bufOffset += 2
	} else {
		req.uri = string(buf[bufOffset : bufOffset+1+pos])
		bufOffset += 2 + pos
	}
	if bufOffset >= bufLen {
		res = 1 // partial
		return
	}

	// 3. Parse headers
	var CRLF []byte = []byte{'\r', '\n'}
	pos = bytes.Index(buf[bufOffset:], CRLF) // skip first \r\n
	if pos < 0 {
		res = 1 // partial
		return
	}
	bufOffset += pos + 2
	headers := make([][2]string, 0, 8)
	for bufOffset < bufLen {
		pos = bytes.Index(buf[bufOffset:], CRLF)
		if pos > 0 {
			if (buf[bufOffset] < 'A' || buf[bufOffset] > 'Z') &&
				(buf[bufOffset] < 'a' || buf[bufOffset] > 'z') {
				// check first char
				res = -1
				return // Abnormal connection. close it
			}
			sepP := bytes.IndexByte(buf[bufOffset:bufOffset+pos], ':')
			if sepP < 0 {
				res = -1
				return // Abnormal connection. close it
			}
			var header [2]string
			header[0] = strings.TrimSpace(string(buf[bufOffset : bufOffset+sepP]))
			header[1] = strings.TrimSpace(string(buf[bufOffset+sepP+1 : bufOffset+pos]))
			headers = append(headers, header)
			if strings.EqualFold(header[0], "Content-Length") {
				if method != 2 { // only post
					res = -1
					return // Abnormal connection. close it
				}
				if len(header[1]) > 10 {
					res = -1
					return // Abnormal connection. close it
				} else {
					if l, err := strconv.ParseInt(header[1], 10, 64); err == nil {
						req.contentLength = int(l)
					}
				}
			} else if strings.EqualFold(header[0], "Host") {
				req.host = header[1]
			}
			bufOffset += pos + 2
		} else if pos == 0 {
			bufOffset += pos + 2
			break // EOF
		} else {
			res = 1 // partial
			return
		}
	}
	req.headers = headers
	headerLen = bufOffset
	res = 0
	return
}
func (h *Http) OnRead() bool {
	buf, n, _ := h.Read()
	if n == 0 { // Abnormal connection
		return false
	} else if n < 0 {
		return true
	}

	if idx := bytes.Index(buf, []byte{'\r', '\n', '\r', '\n'}); idx == -1 {
		return false
	}
	buf = h.WriteBuff()[:0]
	buf = append(buf, httpRespHeader...)
	buf = append(buf, []byte(liveDate.Load().(string))...)
	buf = append(buf, httpRespContentLength...)
	h.Write(buf)
	return true
}
func (h *Http) OnRead2() bool {
	rawBuf, n, _ := h.Read()
	if n == 0 { // Abnormal connection
		return false
	} else if n < 0 {
		return true
	}

	rawBufLen := len(rawBuf)
	if len(h.partialBuf) > 0 { // build partial data
		h.partialBuf = append(h.partialBuf, rawBuf...)
		rawBuf = h.partialBuf
		rawBufLen = len(h.partialBuf)
		h.partialBuf = h.partialBuf[:0] // reset
	}

	bufOffset := 0
	for rawBufLen > 0 {
		req, headerLen, res := h.parseHeader(rawBuf[bufOffset:])
		if res == -1 {
			return false
		} else if res == 1 { // partial header
			h.partialBuf = append(h.partialBuf, rawBuf[bufOffset:]...)
			break
		}

		var payloadBuf []byte
		payloadLen := req.contentLength
		if headerLen > 0 {
			if rawBufLen-headerLen < payloadLen { // partial payload
				h.partialBuf = append(h.partialBuf, rawBuf[bufOffset:]...)
				break
			}
			payloadBuf = rawBuf[bufOffset+headerLen : bufOffset+headerLen+payloadLen]
		}

		bufOffset += headerLen + payloadLen
		rawBufLen -= headerLen + payloadLen
		// get a complete request
		// handle req
		if req.method == 1 { // GET
			buf := h.WriteBuff()[:0]
			buf = append(buf, httpRespHeader...)
			buf = append(buf, []byte(liveDate.Load().(string))...)
			buf = append(buf, httpRespContentLength...)
			h.Write(buf)
		} else {
			_ = payloadBuf
			return false // close it
		}
	}
	return true
}
func (h *Http) OnClose() {
	h.Destroy(h)
}

func updateLiveSecond() {
	for {
		select {
		case now := <-ticker.C:
			liveDate.Store(now.Format("Mon, 02 Jan 2006 15:04:05 GMT"))
		}
	}
}

func main() {
	if runtime.NumCPU() > 3 {
		evPollNum = runtime.NumCPU() * 3 / 2 // 根据实际环境调整一下 pollnum 会有意想不到的提升
	}
	parseFlag()
	fmt.Printf("hello boy! GOMAXPROCS=%d evpoll num=%d\n", procNum, evPollNum)
	runtime.GOMAXPROCS(procNum)

	liveDate.Store(time.Now().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	ticker = time.NewTicker(time.Millisecond * 1000)

	httpRespHeader = []byte(httpHeaderS)
	httpRespContentLength = []byte(contentLengthS)

	forAcceptReactor, err := goev.NewReactor(
		goev.EvFdMaxSize(20480), // default val
		goev.EvPollNum(1),
	)
	if err != nil {
		panic(err.Error())
	}
	forNewFdReactor, err = goev.NewReactor(
		goev.EvFdMaxSize(20480), // default val
		goev.EvPollNum(evPollNum),
	)
	if err != nil {
		panic(err.Error())
	}
	//= http
	_, err = goev.NewAcceptor(forAcceptReactor, ":8080", func() goev.EvHandler { return new(Http) },
		goev.ListenBacklog(256),
		//goev.SockRcvBufSize(16*1024), // 短链接, 不需要很大的缓冲区
	)
	if err != nil {
		panic(err.Error())
	}

	go updateLiveSecond()
	go func() {
		if err = forAcceptReactor.Run(); err != nil {
			panic(err.Error())
		}
	}()
	if err = forNewFdReactor.Run(); err != nil {
		panic(err.Error())
	}
}
