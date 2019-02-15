package adb

import (
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type asyncWriter struct {
	Done           chan struct{}
	DoneCopy       chan struct{} // for debug
	C              chan struct{}
	err            error
	dst            io.WriteCloser
	dstPath        string
	TotalSize      int64
	dev            *Device
	bytesCompleted int64
	copyErrC       chan error
	wg             sync.WaitGroup
}

func newAsyncWriter(dev *Device, dst io.WriteCloser, dstPath string, totalSize int64) *asyncWriter {
	return &asyncWriter{
		Done:      make(chan struct{}),
		DoneCopy:  make(chan struct{}, 1),
		C:         make(chan struct{}),
		dst:       dst,
		dstPath:   dstPath,
		dev:       dev,
		TotalSize: totalSize,
		copyErrC:  make(chan error, 1),
	}
}

// BytesCompleted returns the total number of bytes which have been copied to the destination
func (aw *asyncWriter) BytesCompleted() int64 {
	return aw.bytesCompleted
}

func (aw *asyncWriter) Progress() float64 {
	if (aw.TotalSize) == 0 {
		return 0
	}
	return float64(aw.bytesCompleted) / float64(aw.TotalSize)
}

// Err return error immediately
func (aw *asyncWriter) Err() error {
	return aw.err
}

func (aw *asyncWriter) Cancel() error {
	return aw.dst.Close()
}

// Wait blocks until sync is completed
func (aw *asyncWriter) Wait() {
	<-aw.Done
}

func (aw *asyncWriter) doCopy(reader io.Reader) {
	aw.wg.Add(1)
	defer aw.wg.Done()

	go aw.darinProgress()
	written, err := io.Copy(aw.dst, reader)
	if err != nil {
		aw.err = err
		aw.copyErrC <- err
	}
	aw.TotalSize = written
	defer aw.dst.Close()
	aw.DoneCopy <- struct{}{}
}

func (a *asyncWriter) darinProgress() {
	t := time.NewTicker(time.Millisecond * 500)
	defer func() {
		t.Stop()
		a.wg.Wait()
		a.Done <- struct{}{}
	}()
	var lastSize int32
	for {
		select {
		case <-t.C:
			finfo, err := a.dev.Stat(a.dstPath)
			if err != nil && errors.Cause(err) != ErrFileNotExist {
				a.err = err
				return
			}
			if finfo == nil {
				continue
			}
			if lastSize != finfo.Size {
				lastSize = finfo.Size
				select {
				case a.C <- struct{}{}:
				default:
				}
			}
			a.bytesCompleted = int64(finfo.Size)
			if a.TotalSize != 0 && a.bytesCompleted >= a.TotalSize {
				return
			}
		case <-a.copyErrC:
			return
		}
	}
}
