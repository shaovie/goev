package goev

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"syscall"
	"time"
)

// Point to the last allocated object
var lastLog *Log

func Debug(format string, v ...any) {
	lastLog.debugL.write(format, v...)
}
func Rinfo(format string, v ...any) {
	lastLog.rinfoL.write(format, v...)
}
func Error(format string, v ...any) {
	lastLog.errorL.write(format, v...)
}
func Fatal(format string, v ...any) {
	lastLog.fatalL.write(format, v...)
}
func Warning(format string, v ...any) {
	lastLog.warningL.write(format, v...)
}

type Log struct {
	noCopy

	debugL   log
	rinfoL   log
	errorL   log
	fatalL   log
	warningL log
}

// output to stdout if dir == ""
func NewLog(dir string) (*Log, error) {
	l := &Log{
		debugL:   log{dir: dir, name: "debug", fd: -1},
		rinfoL:   log{dir: dir, name: "rinfo", fd: -1},
		errorL:   log{dir: dir, name: "error", fd: -1},
		fatalL:   log{dir: dir, name: "fatal", fd: -1},
		warningL: log{dir: dir, name: "warng", fd: -1},
	}
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, errors.New("NewLog mkdir fail! " + err.Error())
		}
	}
	lastLog = l
	return l, nil
}
func (l *Log) Debug(format string, v ...any) {
	l.debugL.write(format, v...)
}
func (l *Log) Rinfo(format string, v ...any) {
	l.rinfoL.write(format, v...)
}
func (l *Log) Error(format string, v ...any) {
	l.errorL.write(format, v...)
}
func (l *Log) Fatal(format string, v ...any) {
	l.fatalL.write(format, v...)
}
func (l *Log) Warning(format string, v ...any) {
	l.warningL.write(format, v...)
}

// implement
type log struct {
	newFileYear  int
	newFileMonth int
	newFileDay   int
	fd           int
	dir          string
	name         string
	buff         []byte

	mtx sync.Mutex
}

func (l *log) newFile(year, month, day int) error {
	if l.newFileYear != year || l.newFileMonth != month || l.newFileDay != day {
		l.close()
		if err := l.open(year, month, day); err != nil {
			return err
		}
	}
	return nil
}
func (l *log) open(year, month, day int) (err error) {
	if l.dir == "" {
		l.fd = 1
	} else {
		fname := fmt.Sprintf("%s-%d-%02d-%02d.log", l.name, year, month, day)
		logFile := path.Join(l.dir, fname)
		l.fd, err = syscall.Open(logFile, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_APPEND, 0644)
		if err != nil {
			return err
		}
	}
	l.newFileYear, l.newFileMonth, l.newFileDay = year, month, day
	l.buff = make([]byte, 0, 512)
	l.itoa(year, 4)
	l.buff = append(l.buff, '-')
	l.itoa(int(month), 2)
	l.buff = append(l.buff, '-')
	l.itoa(day, 2)
	l.buff = append(l.buff, ' ')
	return nil
}
func (l *log) close() {
	if l.dir != "" && l.fd != -1 {
		syscall.Close(l.fd)
		l.fd = -1
	}
}
func (l *log) write(format string, v ...any) {
	now := time.Now()
	year, month, day := now.Date()

	l.mtx.Lock()
	defer l.mtx.Unlock()

	if err := l.newFile(year, int(month), day); err != nil {
		return
	}

	if l.fd == -1 {
		return
	}
	hour, min, sec := now.Clock()
	l.itoa(hour, 2)
	l.buff = append(l.buff, ':')
	l.itoa(int(min), 2)
	l.buff = append(l.buff, ':')
	l.itoa(sec, 2)
	l.buff = append(l.buff, '.')
	l.itoa(now.Nanosecond()/1e6, 3)
	if l.dir != "" {
		l.buff = append(l.buff, []byte{' ', '>', ' '}...)
	} else {
		l.buff = append(l.buff, ' ')
		l.buff = append(l.buff, []byte(l.name+" > ")...)
	}

	l.buff = fmt.Appendf(l.buff, format, v...)
	l.buff = append(l.buff, '\n')
	for {
		_, err := syscall.Write(l.fd, l.buff)
		if err != nil && err == syscall.EINTR {
			continue
		}
		break
	}
	l.buff = l.buff[:11 /*len("2023-07-05 ")*/]
}
func (l *log) itoa(i int, wid int) {
	// Assemble decimal in reverse order.
	var b [8]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	l.buff = append(l.buff, b[bp:]...)
}
