package log

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	logChanBufSize = 1024
)

// Level represents a log level type
type Level int

// ExitHandler is the application exit handler that will be executed on a fatal level logging
// Default. It will exit with error code 1 (unix catch all error code)
var ExitHandler = func() {
	os.Exit(1)
}

const (
	// DebugLevel represents DEBUG log level
	DebugLevel Level = iota

	// InfoLevel represents INFO log level
	InfoLevel

	// WarningLevel represents WARNING log level
	WarningLevel

	// ErrorLevel represents ERROR log level
	ErrorLevel

	// FatalLevel represents FATAL log level
	FatalLevel
)

// Logger represents a common logger interface.
type Logger interface {
	io.Closer

	Level() Level
	Run()
	IsRunning() bool
	Log(level Level, pkg string, file string, line int, format string, args ...interface{})
}

// Debugf writes a 'debug' message to configured logger.
func Debugf(format string, args ...interface{}) {
	if inst := instance(); inst.Level() <= DebugLevel {
		ci := getCallerInfo()
		inst.Log(DebugLevel, ci.pkg, ci.filename, ci.line, format, args...)
	}
}

// Infof writes a 'info' message to configured logger.
func Infof(format string, args ...interface{}) {
	if inst := instance(); inst.Level() <= InfoLevel {
		ci := getCallerInfo()
		inst.Log(InfoLevel, ci.pkg, ci.filename, ci.line, format, args...)
	}
}

// Warnf writes a 'warn' message to configured logger.
func Warnf(format string, args ...interface{}) {
	if inst := instance(); inst.Level() <= WarningLevel {
		ci := getCallerInfo()
		inst.Log(WarningLevel, ci.pkg, ci.filename, ci.line, format, args...)
	}
}

// Errorf writes a 'warn' message to configured logger.
func Errorf(format string, args ...interface{}) {
	if inst := instance(); inst.Level() <= ErrorLevel {
		ci := getCallerInfo()
		inst.Log(ErrorLevel, ci.pkg, ci.filename, ci.line, format, args...)
	}
}

// Fatalf writes a 'fatal' message to configured logger.
func Fatalf(format string, args ...interface{}) {
	if inst := instance(); inst.Level() <= FatalLevel {
		ci := getCallerInfo()
		inst.Log(FatalLevel, ci.pkg, ci.filename, ci.line, format, args...)
	}
}

// Error writes an error value to configured logger.
func Error(err error) {
	if inst := instance(); inst.Level() <= ErrorLevel {
		ci := getCallerInfo()
		inst.Log(ErrorLevel, ci.pkg, ci.filename, ci.line, "%v", err)
	}
}

// Fatal writes an error value to the configure logger.
// Application should terminate after locking.
func Fatal(err error) {
	if inst := instance(); inst.Level() <= FatalLevel {
		ci := getCallerInfo()
		inst.Log(FatalLevel, ci.pkg, ci.filename, ci.line, "%v", err)
	}
}

var (
	instMu sync.RWMutex
	inst   Logger
)

// Set sets the global logger
func Set(logger Logger) {
	instMu.Lock()
	_ = inst.Close()
	inst = logger
	instMu.Unlock()
}

func instance() Logger {
	instMu.RLock()
	l := inst
	if !l.IsRunning() {
		l.Run()
	}
	instMu.RUnlock()
	return l
}

type logger struct {
	level  Level
	out    io.Writer
	files  []io.WriteCloser
	b      strings.Builder
	active bool
	recCh  chan record
}

type record struct {
	level      Level
	pkg        string
	file       string
	line       int
	log        string
	continueCh chan struct{}
}

type callerInfo struct {
	pkg      string
	filename string
	line     int
}

func init() {
	// Set default stdout logger
	inst = &logger{
		level: InfoLevel,
		out:   os.Stdout,
		files: []io.WriteCloser{},
		recCh: make(chan record, logChanBufSize),
	}
}

// New returns a default logger instance.
func New(level string, out io.Writer, files ...io.WriteCloser) (Logger, error) {
	lvl, err := levelFromString(level)
	if err != nil {
		return nil, err
	}

	l := &logger{
		level: lvl,
		out:   out,
		files: files,
		recCh: make(chan record, logChanBufSize),
	}

	// Start logger main loop
	go l.mainLoop()
	return l, nil
}

func (l *logger) Level() Level {
	return l.level
}

func (l *logger) IsRunning() bool {
	return l.active
}

func (l *logger) Run() {
	go l.mainLoop()
}

func (l *logger) Log(level Level, pkg string, file string, line int, format string, args ...interface{}) {
	entity := record{
		level:      level,
		pkg:        pkg,
		file:       file,
		line:       line,
		log:        fmt.Sprintf(format, args...),
		continueCh: make(chan struct{}),
	}

	select {
	case l.recCh <- entity:
		if level == FatalLevel {
			<-entity.continueCh // wait until done
		}
	default:
		break // avoid blocking -->
	}
}

func (l *logger) Close() error {
	close(l.recCh)
	return nil
}

func (l *logger) mainLoop() {
	for {
		l.active = true

		select {
		case rec, ok := <-l.recCh:
			if !ok {
				// close log files
				for _, w := range l.files {
					_ = w.Close()
				}

				// Abort main loop execution
				l.active = false
				return
			}

			// Build string from record using strings.Builder
			l.b.Reset()
			l.b.WriteString(time.Now().Format("2006-01-02 15:04:05"))
			l.b.WriteString(" ")
			l.b.WriteString(logLevelGlyph(rec.level))
			l.b.WriteString(" [")
			l.b.WriteString(logLevelAbbreviation(rec.level))
			l.b.WriteString("] ")

			l.b.WriteString(rec.pkg)
			if len(rec.pkg) > 0 {
				l.b.WriteString("/")
			}
			l.b.WriteString(rec.file)
			l.b.WriteString(":")
			l.b.WriteString(strconv.Itoa(rec.line))
			l.b.WriteString(" - ")
			l.b.WriteString(rec.log)
			l.b.WriteString("\n")

			line := l.b.String()
			fmt.Fprint(l.out, line)
			for _, w := range l.files {
				fmt.Fprint(w, line)
			}

			if rec.level == FatalLevel {
				ExitHandler()
			}

			close(rec.continueCh)
		}
	}
}

func getCallerInfo() callerInfo {
	c := callerInfo{}
	_, file, line, ok := runtime.Caller(2)
	if ok {
		c.pkg = filepath.Base(path.Dir(file))
		filename := filepath.Base(file)
		c.filename = strings.TrimSuffix(filename, filepath.Ext(filename))
		c.line = line
	} else {
		c.pkg = "???"
		c.filename = "???"
	}

	return c
}

func logLevelAbbreviation(level Level) string {
	switch level {
	case DebugLevel:
		return "DBG"
	case InfoLevel:
		return "INF"
	case WarningLevel:
		return "WRN"
	case ErrorLevel:
		return "ERR"
	case FatalLevel:
		return "FTL"
	default:
		return ""
	}
}

func logLevelGlyph(level Level) string {
	switch level {
	case DebugLevel:
		return "\U0001f50D"
	case InfoLevel:
		return "\u2139\ufe0f"
	case WarningLevel:
		return "\u26a0\ufe0f"
	case ErrorLevel:
		return "\U0001f4a5"
	case FatalLevel:
		return "\U0001f480"
	default:
		return ""
	}
}

func levelFromString(level string) (Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel, nil
	case "", "info":
		return InfoLevel, nil
	case "warning":
		return WarningLevel, nil
	case "error":
		return ErrorLevel, nil
	case "fatal":
		return FatalLevel, nil
	}

	return Level(-1), fmt.Errorf("log: unrecognized log level: %s", level)
}
