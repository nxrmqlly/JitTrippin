package logs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

type Ref struct {
	RunID string
	Job   string
}

// JobLog holds a Stderr and a Stdout io.WriteCloser meant for a single Job.
type JobLog struct {
	Stderr io.WriteCloser
	Stdout io.WriteCloser

	Close func() error
}

type Line struct {
	Time   time.Time `json:"time"`
	Stream string    `json:"stream"`
	Data   string    `json:"data"`
}

type Store interface {
	Open(ctx context.Context, ref Ref) (*JobLog, error)
	Read(ctx context.Context, ref Ref) (io.ReadCloser, error)
}

type LineWriter struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	stream string
	emit   func(Line) error
}

func NewLineWriter(stream string, emit func(Line) error) *LineWriter {
	return &LineWriter{
		stream: stream,
		emit:   emit,
	}
}

func (w *LineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.buf.Write(p); err != nil {
		return 0, err
	}

	for {
		data := w.buf.Bytes()
		// try to find if a newline was written
		i := bytes.IndexByte(data, '\n')
		if i == -1 {
			break
		}
		line := Line{
			Time:   time.Now(),
			Stream: w.stream,
			Data:   string(data[:i+1]), // only upto the newline.
		}
		if err := w.emit(line); err != nil {
			return 0, err
		}

		w.buf.Next(i + 1)
	}
	return len(p), nil
}

func (w *LineWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf.Len() == 0 {
		return nil
	}

	if err := w.emit(Line{
		Time:   time.Now(),
		Stream: w.stream,
		Data:   w.buf.String(),
	}); err != nil {
		return err
	}

	w.buf.Reset()
	return nil
}

// Close is an alias to Flush
func (w *LineWriter) Close() error {
	return w.Flush()
}

type Broadcaster struct {
	mu   sync.RWMutex
	subs map[Ref]map[chan Line]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subs: make(map[Ref]map[chan Line]struct{}),
	}
}

func (b *Broadcaster) Publish(ref Ref, line Line) {
	b.mu.RLock()
	subs := b.subs[ref]
	b.mu.RUnlock()

	for ch := range subs {
		select {
		case ch <- line:
		default:
			// if subscriber is too slow, drop it
		}
	}
}

func (b *Broadcaster) Subscribe(ref Ref) (schan <-chan Line, unsubscribe func()) {
	ch := make(chan Line, 256)
	// Subscribe
	fmt.Printf("SUBSCRIBE: %+v\n", ref)
	fmt.Printf("Subscribe broadcaster=%p\n", b)
	fmt.Printf("subs map=%p\n", b.subs)

	b.mu.Lock()
	if b.subs[ref] == nil {
		b.subs[ref] = make(map[chan Line]struct{})
	}
	b.subs[ref][ch] = struct{}{}
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		delete(b.subs[ref], ch)

		if len(b.subs[ref]) == 0 {
			delete(b.subs, ref)
		}

		close(ch)

		b.mu.Unlock()
	}

	return ch, unsub
}
