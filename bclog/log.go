package bclog

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Llongfile uint32 = 1 << iota
	Lshortfile
)

var defaultFlags uint32 = Lshortfile

type Level uint32

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelOff
)

var levelStrs = [...]string{"TRC", "DBG", "INF", "WRN", "ERR", "FTL", "OFF"}

func (l Level) String() string {
	if l > LevelOff {
		return "OFF"
	}
	return levelStrs[l]
}

func NewBackend(w io.Writer, opts ...BackendOption) *Backend {
	b := &Backend{w: w, flag: defaultFlags}
	for _, o := range opts {
		o(b)
	}
	return b
}

type Backend struct {
	w    io.Writer
	mu   sync.Mutex
	flag uint32
}

type BackendOption func(b *Backend)

func WithFlags(flags uint32) BackendOption {
	return func(b *Backend) {
		b.flag = flags
	}
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 120)
		return &b
	},
}

func buffer() *[]byte {
	return bufferPool.Get().(*[]byte)
}

func recycleBuffer(b *[]byte) {
	*b = (*b)[:0]
	bufferPool.Put(b)
}

func LevelFromString(s string) (l Level, ok bool) {
	switch strings.ToLower(s) {
	case "trace", "trc":
		return LevelTrace, true
	case "debug", "dbg":
		return LevelDebug, true
	case "info", "inf":
		return LevelInfo, true
	case "warn", "wrn":
		return LevelWarn, true
	case "error", "err":
		return LevelError, true
	case "fatal", "ftl":
		return LevelFatal, true
	case "off":
		return LevelOff, true
	default:
		return LevelInfo, false
	}
}

func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
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
	*buf = append(*buf, b[bp:]...)
}

func formatHeader(buf *[]byte, t time.Time, lvl, tag string, file string, line int) {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	ms := t.Nanosecond() / 1e6

	itoa(buf, year, 4)
	*buf = append(*buf, '-')
	itoa(buf, int(month), 2)
	*buf = append(*buf, '-')
	itoa(buf, day, 2)
	*buf = append(*buf, ' ')
	itoa(buf, hour, 2)
	*buf = append(*buf, ':')
	itoa(buf, min, 2)
	*buf = append(*buf, ':')
	itoa(buf, sec, 2)
	*buf = append(*buf, '.')
	itoa(buf, ms, 3)
	*buf = append(*buf, " ["...)
	*buf = append(*buf, lvl...)
	*buf = append(*buf, "] "...)
	*buf = append(*buf, tag...)
	if file != "" {
		*buf = append(*buf, ' ')
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
	}
	*buf = append(*buf, ": "...)
}

const calldepth = 3

func callsite(flag uint32) (string, int) {
	_, file, line, ok := runtime.Caller(calldepth)
	if !ok {
		return "caller fail", 0
	}
	if flag&Lshortfile != 0 {
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if os.IsPathSeparator(file[i]) {
				short = file[i+1:]
				break
			}
		}
		file = short
	}
	return file, line
}

func (b *Backend) print(lvl, tag string, args ...interface{}) {
	t := time.Now() // get as early as possible

	bytebuf := buffer()

	var file string
	var line int
	if b.flag&(Lshortfile|Llongfile) != 0 {
		file, line = callsite(b.flag)
	}

	formatHeader(bytebuf, t, lvl, tag, file, line)
	buf := bytes.NewBuffer(*bytebuf)
	fmt.Fprintln(buf, args...)
	*bytebuf = buf.Bytes()

	b.mu.Lock()
	b.w.Write(*bytebuf)
	b.mu.Unlock()

	recycleBuffer(bytebuf)
}

func (b *Backend) printf(lvl, tag string, format string, args ...interface{}) {
	t := time.Now() // get as early as possible

	bytebuf := buffer()

	var file string
	var line int
	if b.flag&(Lshortfile|Llongfile) != 0 {
		file, line = callsite(b.flag)
	}

	formatHeader(bytebuf, t, lvl, tag, file, line)
	buf := bytes.NewBuffer(*bytebuf)
	fmt.Fprintf(buf, format, args...)
	*bytebuf = append(buf.Bytes(), '\n')

	b.mu.Lock()
	b.w.Write(*bytebuf)
	b.mu.Unlock()

	recycleBuffer(bytebuf)
}

func (b *Backend) Logger(subsystemTag string) Logger {
	return &slog{LevelInfo, subsystemTag, b}
}

type slog struct {
	lvl Level
	tag string
	b   *Backend
}

func (l *slog) Level() Level {
	return Level(atomic.LoadUint32((*uint32)(&l.lvl)))
}

func (l *slog) SetLevel(level Level) {
	atomic.StoreUint32((*uint32)(&l.lvl), uint32(level))
}

func (l *slog) Trace(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelTrace {
		l.b.print("TRC", l.tag, args...)
	}
}

func (l *slog) Tracef(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelTrace {
		l.b.printf("TRC", l.tag, format, args...)
	}
}

func (l *slog) Debug(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelDebug {
		l.b.print("DBG", l.tag, args...)
	}
}

func (l *slog) Debugf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelDebug {
		l.b.printf("DBG", l.tag, format, args...)
	}
}

func (l *slog) Info(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelInfo {
		l.b.print("INF", l.tag, args...)
	}
}

func (l *slog) Infof(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelInfo {
		l.b.printf("INF", l.tag, format, args...)
	}
}

func (l *slog) Warn(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelWarn {
		l.b.print("WRN", l.tag, args...)
	}
}

func (l *slog) Warnf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelWarn {
		l.b.printf("WRN", l.tag, format, args...)
	}
}

func (l *slog) Error(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelError {
		l.b.print("ERR", l.tag, args...)
	}
}

func (l *slog) Errorf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelError {
		l.b.printf("ERR", l.tag, format, args...)
	}
}

func (l *slog) Fatal(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelFatal {
		l.b.print("FTL", l.tag, args...)
	}
}

func (l *slog) Fatalf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelFatal {
		l.b.printf("FTL", l.tag, format, args...)
	}
}

var Disabled Logger

func init() {
	Disabled = &slog{lvl: LevelOff, b: NewBackend(ioutil.Discard)}
}
