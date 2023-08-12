package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/shaovie/goev"
	"github.com/shaovie/goev/netfd"
)

// Launch args
var (
	evPollNum int = 0
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

var (
	reactor     *goev.Reactor
	asynBufPool sync.Pool
)

const (
	FrameNull     = -1
	FrameContinue = 0
	FrameText     = 1
	FrameBinary   = 2
	FrameClose    = 8
	FramePing     = 9
	FramePong     = 10
)

type CloseCode uint16

const (
	// 正常关闭; 无论为何目的而创建, 该链接都已成功完成任务.
	CloseNormalClosure CloseCode = 1000

	// 终端离开：可能因为服务端错误, 也可能因为浏览器正从打开连接的页面跳转离开.
	CloseGoingAway CloseCode = 1001

	// 协议错误：由于协议错误而中断连接.
	CloseProtocolError CloseCode = 1002

	// 数据格式错误：由于接收到不允许的数据类型而断开连接
	CloseUnsupportedData CloseCode = 1003

	// 保留
	CloseReserved CloseCode = 1004

	// 没有收到预期的状态码.
	CloseNoCloseRcvd CloseCode = 1005

	// 异常关闭：用于期望收到状态码时连接非正常关闭 (也就是说, 没有发送关闭帧).
	CloseAbnormalClosure CloseCode = 1006

	// 由于收到了格式不符的数据而断开连接 (如文本消息中包含了非 UTF-8 数据).
	CloseInvalidPayload CloseCode = 1007

	// 由于收到不符合约定的数据而断开连接. 这是一个通用状态码, 用于不适合使用 1003 和 1009 状态码的场景.
	ClosePolicyViolation CloseCode = 1008

	// 由于收到过大的数据帧而断开连接.
	CloseMessageTooBig CloseCode = 1009

	// 缺少扩展：客户端终止连接，因为期望一个或多个拓展, 但服务器没有.
	CloseMandatoryExtension CloseCode = 1010

	// 内部错误：服务器终止连接，因为遇到异常
	CloseInternalError CloseCode = 1011

	// 服务重启：服务器由于重启而断开连接.
	CloseServiceRestart CloseCode = 1012

	// 稍后再试：服务器由于临时原因断开连接。
	CloseTryAgainLater CloseCode = 1013

	// 错误的网关.
	CloseBadGateway CloseCode = 1014

	// 握手错误：表示连接由于无法完成 TLS 握手而关闭 (例如无法验证服务器证书).
	CloseTLSHandshake CloseCode = 1015
)

type CloseInfo struct {
	Code CloseCode
	Info string
}

func (cc *CloseInfo) Error() string {
	return cc.Info
}

const maxFramePayloadSize = 4 * 1024 * 1024
const maxControlFramePayloadSize = 125
const maxFreamHeaderSize = 14

type wsFrame struct {
	isfin   bool
	flate   bool
	masked  bool
	hlen    int8 // header len
	opcode  int
	payload int64
	maskKey [4]byte
}

type continueWsFrame struct {
	wsFrame

	start bool
	buf   []byte
}

type Conn struct {
	goev.IOHandle

	upgraded        bool
	compressEnabled bool
	closed          bool

	partialBuf []byte

	continueWsFrame continueWsFrame
}

const switchHeaderS = "HTTP/1.1 101 Switching Protocols\r\n" +
	"Upgrade: websocket\r\n" +
	"Connection: Upgrade\r\n" +
	"Sec-WebSocket-Accept: "

const switchHeaderWithFlateS = "HTTP/1.1 101 Switching Protocols\r\n" +
	"Upgrade: websocket\r\n" +
	"Connection: Upgrade\r\n" +
	"Sec-WebSocket-Extensions: permessage-deflate; server_no_context_takeover; client_no_context_takeover\r\n" +
	"Sec-WebSocket-Accept: "

var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func genAcceptKey(challengeKey string) string {
	h := sha1.New()
	bf := unsafe.Slice(unsafe.StringData(challengeKey), len(challengeKey))
	h.Write(bf)
	h.Write(keyGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
func isControlFrame(frameType int) bool {
	if frameType == FrameClose {
		return true
	} else if frameType == FramePing {
		return true
	} else if frameType == FramePong {
		return true
	}
	return false
}
func isMessageFrame(frameType int) bool {
	if frameType == FrameBinary {
		return true
	} else if frameType == FrameText {
		return true
	} else if frameType == FrameContinue {
		return true
	}
	return false
}

func maskBytes(b []byte, key [4]byte) {
	var maskKey = binary.LittleEndian.Uint32(key[:])
	var key64 = uint64(maskKey)<<32 + uint64(maskKey)

	for len(b) >= 64 {
		v := binary.LittleEndian.Uint64(b)
		binary.LittleEndian.PutUint64(b, v^key64)
		v = binary.LittleEndian.Uint64(b[8:16])
		binary.LittleEndian.PutUint64(b[8:16], v^key64)
		v = binary.LittleEndian.Uint64(b[16:24])
		binary.LittleEndian.PutUint64(b[16:24], v^key64)
		v = binary.LittleEndian.Uint64(b[24:32])
		binary.LittleEndian.PutUint64(b[24:32], v^key64)
		v = binary.LittleEndian.Uint64(b[32:40])
		binary.LittleEndian.PutUint64(b[32:40], v^key64)
		v = binary.LittleEndian.Uint64(b[40:48])
		binary.LittleEndian.PutUint64(b[40:48], v^key64)
		v = binary.LittleEndian.Uint64(b[48:56])
		binary.LittleEndian.PutUint64(b[48:56], v^key64)
		v = binary.LittleEndian.Uint64(b[56:64])
		binary.LittleEndian.PutUint64(b[56:64], v^key64)
		b = b[64:]
	}

	for len(b) >= 8 {
		v := binary.LittleEndian.Uint64(b[:8])
		binary.LittleEndian.PutUint64(b[:8], v^key64)
		b = b[8:]
	}

	var n = len(b)
	for i := 0; i < n; i++ {
		idx := i & 3
		b[i] ^= key[idx]
	}
}
func (c *Conn) OnOpen(fd int) bool {
	netfd.SetNoDelay(fd, 1)
	// AddEvHandler 尽量放在最后, (OnOpen 和ORead可能不在一个线程)
	if err := reactor.AddEvHandler(c, fd, goev.EvIn); err != nil {
		return false
	}
	return true
}
func (c *Conn) OnRead() bool {
	if c.closed == true {
		return false
	}
	buf, n, _ := c.Read()
	if n > 0 {
		if c.upgraded == false {
			return c.onUpgrade(buf[0:n])
		}
		return c.onFrame(buf[0:n])
	} else if n == 0 { // Abnormal connection
		return false
	}
	return true
}
func (c *Conn) OnClose() {
	if c.Fd() != -1 {
		netfd.Close(c.Fd())
		c.Destroy(c)
	}
}
func (c *Conn) OnWrite() bool {
	c.AsyncOrderedFlush(c)
	return true
}
func (c *Conn) OnAsyncWriteBufDone(bf []byte, flag int) {
	if flag == 0 {
		asynBufPool.Put(bf)
	}
}

func (c *Conn) onUpgrade(buf []byte) bool {
	// parse http header
	// 不支持不完整的http header, 第个消息就不完整, 这种八成都是非法请求

	var bufLen = len(buf)
	// 1. METHOD
	method := 0 // 1:get 2:post 3unsupport
	bufOffset := 0
	if bufLen < 18 { // GET / HTTP/1.1\r\n\r\n
		return false // Abnormal connection. close it
	}
	if buf[0] == 'G' || buf[0] == 'g' { // GET
		if buf[1] == 'E' || buf[1] == 'e' {
			if buf[2] == 'T' || buf[2] == 't' {
				if buf[3] == ' ' || buf[3] == '\t' {
					method = 1
					bufOffset += 4
				}
			}
		}
	}
	if method == 0 {
		return false // Abnormal connection. close it
	}

	// 2. URI  /a/b/c?p=x&p2=2#yyy
	if buf[bufOffset] != '/' {
		return false // Abnormal connection. close it
	}
	pos := bytes.IndexByte(buf[bufOffset+1:], ' ') // \t ?
	if pos < 0 {
		return false // Abnormal connection. close it
	}
	var uri string
	if pos == 0 {
		uri = "/"
		bufOffset += 2
	} else {
		uri = string(buf[bufOffset : bufOffset+1+pos])
		bufOffset += 2 + pos
	}
	_ = uri // TODO

	// 3. Parse headers
	var CRLF []byte = []byte{'\r', '\n'}
	// skip first \r\n
	pos = bytes.Index(buf[bufOffset:], CRLF)
	if pos < 0 {
		return false // Abnormal connection. close it
	}
	bufOffset += pos + 2
	headers := make([][2]string, 0, 8)
	for bufOffset < bufLen {
		pos = bytes.Index(buf[bufOffset:], CRLF)
		if pos > 0 {
			if (buf[bufOffset] < 'A' || buf[bufOffset] > 'Z') &&
				(buf[bufOffset] < 'a' || buf[bufOffset] > 'z') {
				// check first char
				return false // Abnormal connection. close it
			}
			sepP := bytes.IndexByte(buf[bufOffset:bufOffset+pos], ':')
			if sepP < 0 {
				return false // Abnormal connection. close it
			}
			var header [2]string
			header[0] = strings.TrimSpace(string(buf[bufOffset : bufOffset+sepP]))
			header[1] = strings.TrimSpace(string(buf[bufOffset+sepP+1 : bufOffset+pos]))
			headers = append(headers, header)
			bufOffset += pos + 2
		} else if pos == 0 {
			break // EOF
		} else {
			return false // Abnormal connection. close it
		}
	}

	// check
	// Host: xx
	// Connection: Upgrade
	// Upgrade: websocket
	// Sec-WebSocket-Version: 13
	//
	// Sec-WebSocket-Key: base64
	// Sec-Websocket-Protocol: a, b
	// Sec-WebSocket-Extensions: permessage-deflate; server_no_context_takeover; client_no_context_takeover
	subProtocol := "Sec-Websocket-Protocol"
	var okUpgrade, okWebsocket, okVersion bool
	var key, host string
	subProtocols := make([]string, 0, 2)
	for i := range headers {
		header := headers[i]
		if strings.EqualFold(header[0], "Connection") &&
			strings.EqualFold(header[1], "Upgrade") {
			okUpgrade = true
		} else if strings.EqualFold(header[0], "Host") {
			host = header[1]
		} else if strings.EqualFold(header[0], "Upgrade") &&
			strings.EqualFold(header[1], "websocket") {
			okWebsocket = true
		} else if strings.EqualFold(header[0], "Sec-WebSocket-Version") &&
			header[1] == "13" {
			okVersion = true
		} else if strings.EqualFold(header[0], "Sec-WebSocket-Extensions") {
			if strings.Contains(header[1], "permessage-deflate") {
				c.compressEnabled = true
			}
		} else if strings.EqualFold(header[0], "Sec-WebSocket-Key") {
			key = header[1]
		} else if strings.EqualFold(header[0], subProtocol) {
			arr := strings.Split(header[1], ",")
			for j := range arr {
				if s := strings.TrimSpace(arr[j]); len(s) > 0 {
					subProtocols = append(subProtocols, s)
				}
			}
		}
	}
	_ = host // TODO
	if !(okUpgrade && okWebsocket && okVersion) {
		return false // Abnormal connection. close it
	}
	if len(key) == 0 {
		return false // Abnormal connection. close it
	}

	var resp = make([]byte, 0, 256)
	c.compressEnabled = false // 还不支持
	if c.compressEnabled {
		resp = append(resp, (unsafe.Slice(unsafe.StringData(switchHeaderWithFlateS),
			len(switchHeaderWithFlateS)))...)
	} else {
		resp = append(resp, (unsafe.Slice(unsafe.StringData(switchHeaderS), len(switchHeaderS)))...)
	}

	acceptKey := genAcceptKey(key)
	resp = append(resp, (unsafe.Slice(unsafe.StringData(acceptKey), len(acceptKey)))...)
	resp = append(resp, CRLF...)

	if len(subProtocols) > 0 {
		resp = append(resp, (unsafe.Slice(unsafe.StringData(subProtocol), len(subProtocol)))...)
		sps := strings.Join(subProtocols, ",")
		resp = append(resp, (unsafe.Slice(unsafe.StringData(sps), len(sps)))...)
		resp = append(resp, CRLF...)
	}

	// end
	resp = append(resp, CRLF...)

	writen, _ := c.Write(resp)
	if writen < len(resp) {
		bf := asynBufPool.Get().([]byte)
		n := copy(bf, buf[writen:])
		c.AsyncWrite(c, goev.AsyncWriteBuf{
			Len: n,
			Buf: bf,
		})
	}
	c.upgraded = true
	//c.ScheduleTimer(c, 20*1000, 20*1000)
	return true
}
func (c *Conn) onFrame(buf []byte) bool {
	bufLen := len(buf)
	if len(c.partialBuf) > 0 { // build partial data
		c.partialBuf = append(c.partialBuf, buf...)
		buf = c.partialBuf
		bufLen = len(c.partialBuf)
		c.partialBuf = c.partialBuf[:0] // reset
	}

	bufOffset := 0
	for bufLen > 0 {
		wsf, ret := c.parseFrameHeader(buf[bufOffset:])
		if ret == false {
			return false
		}
		if wsf.hlen == 0 { // partial header
			c.partialBuf = append(c.partialBuf, buf[bufOffset:]...)
			break
		}

		var payloadBuf []byte
		hlen := int(wsf.hlen)
		payloadLen := int(wsf.payload)
		if payloadLen > 0 {
			if bufLen-hlen < payloadLen { // partial payload
				c.partialBuf = append(c.partialBuf, buf[bufOffset:]...)
				break
			}
			payloadBuf = buf[bufOffset+hlen : bufOffset+hlen+payloadLen]
		}

		// get a complete frame
		bufOffset += hlen + payloadLen
		bufLen -= hlen + payloadLen

		if wsf.isfin {
			if len(payloadBuf) > 0 {
				maskBytes(payloadBuf, wsf.maskKey)
			}

			if isControlFrame(wsf.opcode) {
				if wsf.opcode == FramePing {
					c.OnPing(payloadBuf)
				} else if wsf.opcode == FramePong {
					c.OnPong(payloadBuf)
				} else if wsf.opcode == FrameClose {
					ce := CloseInfo{Code: CloseNoCloseRcvd}
					if len(payloadBuf) > 1 {
						ce.Code = CloseCode(binary.BigEndian.Uint16(payloadBuf))
						ce.Info = string(payloadBuf[2:])
					}
					c.OnCloseFrame(ce)
					return false // close
				}
			} else {
				if wsf.opcode == FrameContinue {
					if c.continueWsFrame.start == false {
						return false // except
					}
					// continue end
					c.continueWsFrame.buf = append(c.continueWsFrame.buf, payloadBuf...)
					payloadBuf = c.continueWsFrame.buf
					c.continueWsFrame.buf = nil // release buf
					c.continueWsFrame.start = false
					c.OnMessage(FrameText, payloadBuf)
				} else {
					if isMessageFrame(wsf.opcode) {
						c.OnMessage(wsf.opcode, payloadBuf)
					}
				}
			}
		} else {
			if wsf.opcode != FrameContinue { // first continue frame
				c.continueWsFrame.wsFrame = wsf
				c.continueWsFrame.start = true
				c.continueWsFrame.buf = make([]byte, 0, len(payloadBuf))
				c.continueWsFrame.buf = append(c.continueWsFrame.buf, payloadBuf...)
			} else { // other continue frame
				if c.continueWsFrame.start == false {
					return false // except
				}
				c.continueWsFrame.buf = append(c.continueWsFrame.buf, payloadBuf...)
			}
		}
	}
	return true
}
func (c *Conn) parseFrameHeader(buf []byte) (wsFrame, bool) {
	var fh wsFrame
	bufLen := len(buf)
	if bufLen < 2 {
		return fh, true // partial header
	}
	// byte1 获取FIN标志、操作码(Opcode)、压缩标志
	b1 := buf[0]
	bufOffset := 1
	fh.isfin = b1&(1<<7) != 0
	fh.flate = b1&(1<<6) != 0
	rsv1 := (b1 << 1 >> 7) == 1
	rsv2 := (b1 << 2 >> 7) == 1
	rsv3 := (b1 << 3 >> 7) == 1
	if !c.compressEnabled && (rsv1 || rsv2 || rsv3) {
		return fh, false // Abnormal connection. close it
	}
	fh.opcode = int(b1 & 0xf)
	if isControlFrame(fh.opcode) == false && isMessageFrame(fh.opcode) == false {
		return fh, false // Abnormal connection. close it
	}
	if isControlFrame(fh.opcode) && fh.isfin == false { // control frame not support continue frame
		return fh, false // Abnormal connection. close it
	}

	// byte2 获取掩码标志、数据长度
	b2 := buf[bufOffset]
	fh.masked = b2&(1<<7) != 0
	// RFC6455: All frames sent from client to server have this bit set to 1
	if fh.masked == false {
		return fh, false // Abnormal connection. close it
	}
	fh.payload = int64(b2 & 0x7f)
	if fh.payload == 126 {
		bufOffset++
		if bufOffset+2 > bufLen {
			return fh, true // partial header
		}
		fh.payload = int64(binary.BigEndian.Uint16(buf[bufOffset : bufOffset+2]))
		bufOffset += 2
	} else if fh.payload == 127 {
		bufOffset++
		if bufOffset+8 > bufLen {
			return fh, true // partial header
		}
		fh.payload = int64(binary.BigEndian.Uint64(buf[bufOffset : bufOffset+8]))
		bufOffset += 8
	} else {
		bufOffset++
	}

	if fh.payload < 0 {
		return fh, false // Abnormal connection. close it
	}
	if fh.payload > maxFramePayloadSize {
		return fh, false // Abnormal connection. close it
	}
	if isControlFrame(fh.opcode) && fh.payload > maxControlFramePayloadSize {
		return fh, false // Abnormal connection. close it
	}

	// 获取掩码(Mask)标志，并读取掩码（4个字节）
	if fh.masked {
		if bufOffset+4 > bufLen {
			return fh, true // partial header
		}
		copy(fh.maskKey[:], buf[bufOffset:bufOffset+4])
		bufOffset += 4
	}
	fh.hlen = int8(bufOffset)
	// parse end
	return fh, true
}
func (c *Conn) writeControlFrame(opcode int, payload []byte) {
	if c.closed {
		return
	}
	payloadLen := len(payload)
	if payloadLen > maxControlFramePayloadSize {
		// TODO handle panic
		return
	}
	hlen := 2
	buf := make([]byte, maxControlFramePayloadSize+hlen)
	buf[0] = byte(opcode) | 1<<7
	buf[1] = byte(payloadLen)
	copy(buf[hlen:], payload)
	c.Write(buf[0 : payloadLen+hlen])
}
func (c *Conn) writeMessageFrame(opcode int, data []byte, flate, fin bool) {
	if c.closed {
		return
	}
	buff := c.WriteBuff() // poll shared buffer
	hlen := 2
	b0 := byte(opcode)
	if fin {
		b0 |= 1 << 7
	}
	if flate {
		b0 |= 1 << 6
	}
	buff[0] = b0

	payloadLen := len(data) // int Enough
	if payloadLen > maxFramePayloadSize {
		return // TODO handle panic
	}

	if payloadLen >= math.MaxUint16 {
		buff[1] = 127
		binary.BigEndian.PutUint64(buff[2:], uint64(payloadLen))
		hlen += 8
	} else if payloadLen > 125 {
		buff[1] = 126
		binary.BigEndian.PutUint16(buff[2:], uint16(payloadLen))
		hlen += 2
	} else {
		buff[1] = byte(payloadLen)
	}
	// no mask in server side
	copy(buff[hlen:], data)
	wlen := hlen + payloadLen
	writen, err := c.Write(buff[:wlen])
	if err == nil && writen < wlen {
		var flag, n int
		var bf []byte
		if wlen-writen < 1024 {
			bf = asynBufPool.Get().([]byte)
			n = copy(bf, buf[writen:])
		} else {
			bf = make([]byte, wlen-writen)
			n = copy(bf, buff[writen:])
			flat = 2
		}
		c.AsyncWrite(c, goev.AsyncWriteBuf{
			Flag: flag,
			Len:  n,
			Buf:  bf,
		})
	}
}
func (c *Conn) OnTimeout(now int64) bool {
	c.writeControlFrame(FramePing, nil)
	return true
}
func (c *Conn) OnMessage(opcode int, data []byte) {
	c.writeMessageFrame(opcode, data, false, true)
}
func (c *Conn) OnPing(data []byte) {
	c.writeControlFrame(FramePong, data)
}
func (c *Conn) OnPong(data []byte) {
}
func (c *Conn) OnCloseFrame(ci CloseInfo) {
	if c.closed == false {
		bf := (*(*[2]byte)(unsafe.Pointer(&ci.Code)))[:]
		binary.BigEndian.PutUint16(bf, uint16(CloseNormalClosure))
		c.writeControlFrame(FrameClose, bf[0:2])
		c.closed = true
	}
}
func main() {
	parseFlag()
	fmt.Printf("hello boy! GOMAXPROCS=%d evpoll num=%d\n", procNum, evPollNum)
	runtime.GOMAXPROCS(procNum)

	asynBufPool.New = func() any {
		return make([]byte, 1024)
	}

	var err error
	reactor, err = goev.NewReactor(
		goev.EvPollNum(evPollNum),
		goev.EvPollWriteBuffSize(maxFramePayloadSize+32),
	)
	if err != nil {
		panic(err.Error())
	}
	_, err = goev.NewAcceptor(reactor, func() goev.EvHandler { return new(Conn) }, ":8080")
	if err != nil {
		panic(err.Error())
	}
	if err = reactor.Run(); err != nil {
		panic(err.Error())
	}
}
