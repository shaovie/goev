package main

import (
    "fmt"
    "net"
    "os"
	"runtime"
	"sync/atomic"
	"time"
)

var (
	httpRespHeader        []byte
	httpRespContentLength []byte
	ticker                *time.Ticker
	liveDate              atomic.Value
)

const httpHeaderS = "HTTP/1.1 200 OK\r\nConnection: keep-alive\r\nServer: goev\r\nContent-Type: text/plain\r\nDate: "
const contentLengthS = "\r\nContent-Length: 13\r\n\r\nHello, World!"

func updateLiveSecond() {
	for {
		select {
		case now := <-ticker.C:
			liveDate.Store(now.Format("Mon, 02 Jan 2006 15:04:05 GMT"))
		}
	}
}

func oneConn(conn net.Conn) {
    buf := make([]byte, 1024)
    for {
        n, _ := conn.Read(buf)
        if n == 0 {
            conn.Close()
            break
        }


        buf = buf[:0]
        buf = append(buf, httpRespHeader...)
        buf = append(buf, []byte(liveDate.Load().(string))...)
        buf = append(buf, httpRespContentLength...)
        conn.Write(buf)
    }
}
func main() {
	fmt.Println("hello boy")
	runtime.GOMAXPROCS(runtime.NumCPU()*2 - 1)
	liveDate.Store(time.Now().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	ticker = time.NewTicker(time.Millisecond * 1000)

    l, err := net.Listen("tcp", ":8080")
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    for {
        conn, err := l.Accept()
        if err != nil {
            continue
        }
        go oneConn(conn)
    }
}