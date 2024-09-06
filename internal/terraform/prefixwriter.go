package terraform

import (
	"bytes"
	"io"
	"sync"
)

type writer struct {
	writer io.Writer
	prefix []byte
	buf    *bytes.Buffer
	saved  *bytes.Buffer
	mu     sync.Mutex
}

func prefixWriter(w io.Writer, prefix string) *writer {
	return &writer{
		writer: w,
		prefix: []byte(prefix),
		buf:    bytes.NewBuffer(nil),
		saved:  bytes.NewBuffer(nil),
	}
}

func (p *writer) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	totalWritten := 0
	for len(data) > 0 {
		newlineIndex := bytes.IndexByte(data, '\n')
		if newlineIndex == -1 {
			p.buf.Write(data)
			totalWritten += len(data)
			break
		}
		line := data[:newlineIndex+1]
		p.buf.Write(line)
		totalWritten += len(line)
		p.flushBuffer()
		data = data[newlineIndex+1:]
	}
	if p.buf.Len() > 0 {
		err := p.flushBuffer()
		if err != nil {
			return totalWritten, err
		}
	}
	return totalWritten, nil
}

func (p *writer) flushBuffer() error {
	if p.buf.Len() == 0 {
		return nil
	}
	_, err := p.writer.Write(p.prefix)
	if err != nil {
		return err
	}
	_, err = p.writer.Write(p.buf.Bytes())
	if err != nil {
		return err
	}
	p.saved.Write(p.buf.Bytes())
	p.buf.Reset()
	return nil
}

func (p *writer) Output() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.saved.String()
}
