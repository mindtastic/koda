package log

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type writerBuffer struct {
	mu      sync.RWMutex
	buf     *bytes.Buffer
	closeCh chan bool
}

func newWritterBuffer() *writerBuffer {
	return &writerBuffer{
		buf:     bytes.NewBuffer(nil),
		closeCh: make(chan bool),
	}
}

func (wb *writerBuffer) Write(p []byte) (int, error) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	return wb.buf.Write(p)
}

func (wb *writerBuffer) String() string {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	return wb.buf.String()
}

func (wb *writerBuffer) Reset() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.buf.Reset()
}

func (wb *writerBuffer) Close() error {
	wb.closeCh <- true
	return nil
}

func (wb *writerBuffer) Done() <-chan bool {
	return wb.closeCh
}

func setupTest(level string) (*writerBuffer, *writerBuffer) {
	out := newWritterBuffer()
	logfile := newWritterBuffer()
	l, _ := New(level, out, logfile)
	Set(l)
	return out, logfile
}

func TestDebugLog(t *testing.T) {
	bw, _ := setupTest("debug")

	Debugf("test debug log!")
	time.Sleep(time.Millisecond * 50)

	l := bw.String()

	if !strings.Contains(l, "[DBG]") {
		t.FailNow()
	}

	if !strings.Contains(l, "🔍") {
		t.FailNow()
	}

	if !strings.Contains(l, "test debug log!") {
		t.FailNow()
	}
}

func TestInfoLog(t *testing.T) {
	bw, _ := setupTest("info")

	Infof("test info log!")
	time.Sleep(time.Millisecond * 50)

	l := bw.String()

	if !strings.Contains(l, "[INF]") {
		t.FailNow()
	}

	if !strings.Contains(l, "ℹ️") {
		t.FailNow()
	}

	if !strings.Contains(l, "test info log!") {
		t.FailNow()
	}
}

func TestWarningLog(t *testing.T) {
	bw, _ := setupTest("warning")

	Warnf("test warning log!")
	time.Sleep(time.Millisecond * 50)

	l := bw.String()

	if !strings.Contains(l, "[WRN]") {
		t.FailNow()
	}

	if !strings.Contains(l, "⚠️") {
		t.FailNow()
	}

	if !strings.Contains(l, "test warning log!") {
		t.FailNow()
	}
}

func TestErrorLog(t *testing.T) {
	bw, _ := setupTest("error")

	Errorf("test error log!")
	time.Sleep(time.Millisecond * 50)

	l := bw.String()

	if !strings.Contains(l, "[ERR]") {
		t.FailNow()
	}

	if !strings.Contains(l, "💥") {
		t.FailNow()
	}

	if !strings.Contains(l, "test error log!") {
		t.FailNow()
	}

	bw.Reset()

	Error(errors.New("some error string"))
	time.Sleep(time.Millisecond * 50)

	l = bw.String()
	if !strings.Contains(l, "some error string") {
		t.FailNow()
	}
}

func TestFatalLog(t *testing.T) {
	var exited bool
	ExitHandler = func() {
		exited = true
	}
	bw, _ := setupTest("fatal")

	Fatalf("test fatal log!")
	time.Sleep(time.Millisecond * 50)

	if !exited {
		t.Fatal("no exit handler call on log.Fatalf call")
	}

	l := bw.String()

	if !strings.Contains(l, "[FTL]") {
		t.FailNow()
	}

	if !strings.Contains(l, "💀") {
		t.FailNow()
	}

	if !strings.Contains(l, "test fatal log!") {
		t.FailNow()
	}

	bw.Reset()
	exited = false

	Fatal(errors.New("some error string"))
	time.Sleep(time.Millisecond * 50)

	if !exited {
		t.Fatalf("no exit handler call on log.Fatal call")
	}

	l = bw.String()
	if !strings.Contains(l, "some error string") {
		t.FailNow()
	}
}

func TestLogFile(t *testing.T) {
	bw, lf := setupTest("debug")

	Debugf("test debug log!")
	time.Sleep(time.Millisecond * 50)

	if bw.String() != lf.String() {
		t.Fatal("out and file log differs")
	}

	// test that file is closed
	inst.Close()

	select {
	case <-lf.Done():
		// Pass test
	case <-time.After(time.Second):
		t.Fatal("log file has not been closed")
	}
}

func TestUndefinedLevel(t *testing.T) {
	_, err := New("Undefined logging level", os.Stdout)
	if !strings.Contains(err.Error(), "unrecognized log level") {
		t.FailNow()
	}
}
