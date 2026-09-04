package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type BackendDialer interface {
	DialBackend(ctx context.Context) (net.Conn, error)
}

type Forwarder struct {
	address string
	dialer  BackendDialer
	logger  *slog.Logger

	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	active   sync.Map
	count    atomic.Int64
}

var bufferPool = sync.Pool{
	New: func() any {
		buffer := make([]byte, 32*1024)
		return &buffer
	},
}

func NewForwarder(address string, dialer BackendDialer, logger *slog.Logger) *Forwarder {
	return &Forwarder{address: address, dialer: dialer, logger: logger}
}

func (f *Forwarder) Start(parent context.Context) error {
	listener, err := net.Listen("tcp4", f.address)
	if err != nil {
		return err
	}
	f.listener = listener
	f.ctx, f.cancel = context.WithCancel(parent)
	f.wg.Add(1)
	go f.acceptLoop()
	return nil
}

func (f *Forwarder) Stop() {
	if f.cancel != nil {
		f.cancel()
	}
	if f.listener != nil {
		_ = f.listener.Close()
	}
	f.active.Range(func(key, _ any) bool {
		_ = key.(net.Conn).Close()
		return true
	})
	f.wg.Wait()
}

func (f *Forwarder) ActiveConnections() int64 {
	return f.count.Load()
}

func (f *Forwarder) acceptLoop() {
	defer f.wg.Done()
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			if f.ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			if f.logger != nil {
				f.logger.Warn("accept failed", "error", err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetKeepAlive(true)
			_ = tcp.SetKeepAlivePeriod(30 * time.Second)
		}
		f.active.Store(conn, struct{}{})
		f.count.Add(1)
		f.wg.Add(1)
		go f.handle(conn)
	}
}

func (f *Forwarder) handle(client net.Conn) {
	defer f.wg.Done()
	defer func() {
		f.active.Delete(client)
		f.count.Add(-1)
		_ = client.Close()
	}()

	ctx, cancel := context.WithTimeout(f.ctx, 12*time.Second)
	backend, err := f.dialer.DialBackend(ctx)
	cancel()
	if err != nil {
		if f.logger != nil && f.ctx.Err() == nil {
			f.logger.Warn("backend connection failed", "error", err)
		}
		return
	}
	defer backend.Close()
	f.active.Store(backend, struct{}{})
	defer f.active.Delete(backend)
	if tcp, ok := backend.(*net.TCPConn); ok {
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(30 * time.Second)
	}

	type copyResult struct{ err error }
	results := make(chan copyResult, 2)
	go func() { results <- copyResult{err: copyHalfClose(backend, client)} }()
	go func() { results <- copyResult{err: copyHalfClose(client, backend)} }()

	first := <-results
	if first.err != nil && !isClosedError(first.err) {
		_ = client.SetDeadline(time.Now())
		_ = backend.SetDeadline(time.Now())
	}
	<-results
}

func copyHalfClose(dst, src net.Conn) error {
	buffer := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(buffer)
	_, err := io.CopyBuffer(dst, src, *buffer)
	if tcp, ok := dst.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
	return err
}

func isClosedError(err error) bool {
	return err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF)
}
