package adb

import (
	"io"
	"sync/atomic"
)

// AsyncWriter does copying in the background.
type AsyncWriter struct {
	// channel for cancelation. Maybe convert back to (chan chan struct{})
	cancel    chan struct{}
	err       atomic.Value
	dst       io.WriteCloser
	totalSize int64
	dev       *Device
	// atomic completion count
	bytesCompleted int64
}

func newAsyncWriter(dev *Device, dst io.WriteCloser, totalSize int64) *AsyncWriter {
	return &AsyncWriter{
		cancel:    make(chan struct{}),
		dst:       dst,
		dev:       dev,
		totalSize: totalSize,
	}
}

// BytesCompleted returns the total number of bytes which have been copied to the destination
func (aw *AsyncWriter) BytesCompleted() int64 {
	return atomic.LoadInt64(&aw.bytesCompleted)
}

// Progress returns the progress between 0 and 1.
func (aw *AsyncWriter) Progress() float64 {
	if aw.totalSize == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&aw.bytesCompleted)) / float64(aw.totalSize)
}

// Err return a potential error immediately.
func (aw *AsyncWriter) Err() error {
	v := aw.err.Load()
	if err, ok := v.(error); ok {
		return err
	}
	return nil
}

// Cancel cancels the asyncronus write. Blocks until cancel was confirmed.
// Data written will not be undone.
func (aw *AsyncWriter) Cancel() {
	select {
	case <-aw.cancel:
		return
	case aw.cancel <- struct{}{}:
		close(aw.cancel)
	}
}

// Wait blocks until the operation is completed or an error ocurred.
// Check for an error afterwards.
func (aw *AsyncWriter) Wait() {
	<-aw.cancel
}

// call this from a goroutine
// redFrom closes aw.cancel on complete but not if canceled.
func (aw *AsyncWriter) readFrom(r io.Reader) {
	defer func() {
		err := aw.dst.Close()
		if err != nil && aw.err.Load() == nil {
			aw.err.Store(err)
		}
	}()
	buf := make([]byte, 16*1024)
outer:
	for {
		select {
		case <-aw.cancel:
			return
		default:
			nr, er := r.Read(buf)
			if nr > 0 {
				nw, ew := aw.dst.Write(buf[0:nr])
				if nw > 0 {
					atomic.AddInt64(&aw.bytesCompleted, int64(nw))
				}
				if ew != nil {
					aw.err.Store(ew)
					break outer
				}
				if nr != nw {
					aw.err.Store(io.ErrShortWrite)
					break outer
				}
			}
			if er != nil {
				if er != io.EOF {
					aw.err.Store(er)
				}
				break outer
			}
		}
	}
	select {
	case <-aw.cancel:
	default:
		close(aw.cancel)
	}
}
