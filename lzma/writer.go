package lzma

import (
	"bufio"
	"errors"
	"io"
)

// MinDictCap and MaxDictCap provide the range of supported dictionary
// capacities.
const (
	MinDictCap = 1 << 12
	MaxDictCap = 1<<32 - 1
)

// Writer compresses data in the classic LZMA format.
type Writer struct {
	Properties Properties
	DictCap    int
	Size       int64
	BufSize    int
	EOSMarker  bool
	bw         io.ByteWriter
	buf        *bufio.Writer
	e          *Encoder
}

// NewWriter creates a new writer for the classic LZMA format.
func NewWriter(lzma io.Writer) *Writer {
	w := &Writer{
		Properties: Properties{LC: 3, LP: 0, PB: 2},
		DictCap:    8 * 1024 * 1024,
		Size:       -1,
		BufSize:    4096,
		EOSMarker:  true,
	}

	var ok bool
	w.bw, ok = lzma.(io.ByteWriter)
	if !ok {
		w.buf = bufio.NewWriter(lzma)
		w.bw = w.buf
	}

	return w
}

func (w *Writer) writeHeader() error {
	p := make([]byte, 13)
	p[0] = w.Properties.Code()
	putUint32LE(p[1:5], uint32(w.DictCap))
	var l uint64
	if w.Size >= 0 {
		l = uint64(w.Size)
	} else {
		l = noHeaderLen
	}
	putUint64LE(p[5:], l)
	_, err := w.bw.(io.Writer).Write(p)
	return err
}

func (w *Writer) init() error {
	if w.e != nil {
		panic("w.e expected to be nil")
	}
	var err error
	if err = w.Properties.Verify(); err != nil {
		return err
	}
	if !(MinDictCap <= w.DictCap && int64(w.DictCap) <= MaxDictCap) {
		return errors.New("lzma.Writer: DictCap out of range")
	}
	if w.Size < 0 {
		w.EOSMarker = true
	}
	if !(maxMatchLen <= w.BufSize) {
		return errors.New(
			"lzma.Writer: lookahead buffer size too small")
	}

	state := NewState(w.Properties)
	dict, err := NewEncoderDict(w.DictCap, w.DictCap+w.BufSize)
	if err != nil {
		return err
	}
	var flags EncoderFlags
	if w.EOSMarker {
		flags = EOSMarker
	}
	if w.e, err = NewEncoder(w.bw, state, dict, flags); err != nil {
		return err
	}

	err = w.writeHeader()
	return err
}

// Write puts data into the Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.e == nil {
		if err = w.init(); err != nil {
			return 0, err
		}
	}
	if w.Size >= 0 {
		m := w.Size
		m -= w.e.Compressed() + int64(w.e.Dict.Buffered())
		if m < 0 {
			m = 0
		}
		if m < int64(len(p)) {
			p = p[:m]
			err = ErrNoSpace
		}
	}
	var werr error
	if n, werr = w.e.Write(p); werr != nil {
		err = werr
	}
	return n, err
}

// Close closes the writer stream. It ensures that all data from the
// buffer will be compressed and the LZMA stream will be finished.
func (w *Writer) Close() error {
	if w.e == nil {
		if err := w.init(); err != nil {
			return err
		}
	}
	if w.Size >= 0 {
		n := w.e.Compressed() + int64(w.e.Dict.Buffered())
		if n != w.Size {
			return errSize
		}
	}
	err := w.e.Close()
	if w.buf != nil {
		ferr := w.buf.Flush()
		if err == nil {
			err = ferr
		}
	}
	return err
}
