package mc

import (
	"bytes"
	"compress/gzip"
	"io"
	"sync"
)

var (
	bytesBuffPool = sync.Pool{
		New: func() any {
			return &bytes.Buffer{}
		},
	}
	gzipReaderPool = sync.Pool{}
	gzipWriterPool = sync.Pool{
		New: func() any {
			w, err := gzip.NewWriterLevel(nil, gzip.BestSpeed)
			if err != nil {
				panic("gzip pool bug")
			}
			return w
		},
	}
)

func gzipReader(r io.Reader) (*gzip.Reader, error) {
	if v, ok := gzipReaderPool.Get().(*gzip.Reader); ok {
		if err := v.Reset(r); err != nil {
			return nil, err
		}
		return v, nil
	}
	return gzip.NewReader(r)
}

func gzipWriter(w io.Writer) *gzip.Writer {
	v := gzipWriterPool.Get().(*gzip.Writer)
	v.Reset(w)
	return v
}

func (c *Client) compress(v []byte) ([]byte, error) {
	if c.opts.Compression.Compress != nil {
		return c.opts.Compression.Compress(v)
	}
	return compress(v)
}

func (c *Client) decompress(v []byte) ([]byte, error) {
	if c.opts.Compression.Decompress != nil {
		return c.opts.Compression.Decompress(v)
	}
	return decompress(v)
}

func compress(v []byte) ([]byte, error) {
	buff := bytesBuffPool.Get().(*bytes.Buffer)
	buff.Reset()
	w := gzipWriter(buff)
	{
		w.Write(v)
		if err := w.Close(); err != nil {
			return nil, err
		}
	}

	size := buff.Len()

	if n := size - len(v); n > 0 {
		v = append(v, make([]byte, n)...)
	}

	copy(v, buff.Bytes())

	v = v[:size]

	gzipWriterPool.Put(w)
	bytesBuffPool.Put(buff)
	return v, nil
}

func decompress(value []byte) ([]byte, error) {
	buff := bytesBuffPool.Get().(*bytes.Buffer)
	buff.Reset()
	buff.Write(value)

	r, err := gzipReader(buff)
	if err != nil {
		return nil, err
	}
	defer func() {
		bytesBuffPool.Put(buff)
		gzipReaderPool.Put(r)
	}()

	return io.ReadAll(r)
}
