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
	lastLog.debugL.output(format, v...)
}
func Info(format string, v ...any) {
	lastLog.infoL.output(format, v...)
}
func Warn(format string, v ...any) {
	lastLog.warnL.output(format, v...)
}
func Error(format string, v ...any) {
	lastLog.errorL.output(format, v...)
}
func Fatal(format string, v ...any) {
	lastLog.fatalL.output(format, v...)
}

type Log struct {
	noCopy

	debugL log
	infoL  log
	errorL log
	fatalL log
	warnL  log
}

func init() {
	lastLog, _ = NewLog("")
}

// output to stdout if dir == ""
func NewLog(dir string) (*Log, error) {
	l := &Log{
		debugL: log{dir: dir, name: "debug", fd: -1},
		infoL:  log{dir: dir, name: "info", fd: -1},
		errorL: log{dir: dir, name: "error", fd: -1},
		fatalL: log{dir: dir, name: "fatal", fd: -1},
		warnL:  log{dir: dir, name: "warn", fd: -1},
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
	l.debugL.output(format, v...)
}
func (l *Log) Info(format string, v ...any) {
	l.infoL.output(format, v...)
}
func (l *Log) Warn(format string, v ...any) {
	l.warnL.output(format, v...)
}
func (l *Log) Error(format string, v ...any) {
	l.errorL.output(format, v...)
}
func (l *Log) Fatal(format string, v ...any) {
	l.fatalL.output(format, v...)
}

// implement
type log struct {
	newFileYear  int
	newFileMonth int
	newFileDay   int
	fd           int
	buff         []byte
	dir          string
	name         string

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
		for {
			l.fd, err = syscall.Open(logFile, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_APPEND, 0644)
			if err != nil {
				if err == syscall.EINTR {
					continue
				}
				return err
			}
			break
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
func (l *log) output(format string, v ...any) {
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
	l.buff = l.buff[:11] // 11 is len("2023-07-05 ")
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
