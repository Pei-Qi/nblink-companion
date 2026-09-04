package proxy

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func BenchmarkThroughput(b *testing.B) {
	backend := benchmarkEchoServer(b)
	defer backend.Close()
	forwarder, forwardedAddress := benchmarkForwarder(b, backend.Addr().String())
	defer forwarder.Stop()

	payload := bytes.Repeat([]byte("nblink-forwarder-"), 2048)
	for _, target := range []struct {
		name    string
		address string
	}{
		{name: "Direct", address: backend.Addr().String()},
		{name: "Forwarded", address: forwardedAddress},
	} {
		b.Run(target.name, func(b *testing.B) {
			conn := benchmarkDial(b, target.address)
			defer conn.Close()
			response := make([]byte, len(payload))
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := conn.Write(payload); err != nil {
					b.Fatal(err)
				}
				if _, err := io.ReadFull(conn, response); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkConnectionLatency(b *testing.B) {
	backend := benchmarkEchoServer(b)
	defer backend.Close()
	forwarder, forwardedAddress := benchmarkForwarder(b, backend.Addr().String())
	defer forwarder.Stop()

	for _, target := range []struct {
		name    string
		address string
	}{
		{name: "Direct", address: backend.Addr().String()},
		{name: "Forwarded", address: forwardedAddress},
	} {
		b.Run(target.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				conn := benchmarkDial(b, target.address)
				if err := benchmarkClose(conn); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkConcurrentConnections(b *testing.B) {
	backend := benchmarkEchoServer(b)
	defer backend.Close()
	forwarder, forwardedAddress := benchmarkForwarder(b, backend.Addr().String())
	defer forwarder.Stop()
	payload := bytes.Repeat([]byte("c"), 1024)

	b.SetBytes(int64(len(payload)))
	b.RunParallel(func(pb *testing.PB) {
		response := make([]byte, len(payload))
		for pb.Next() {
			conn := benchmarkDial(b, forwardedAddress)
			if _, err := conn.Write(payload); err != nil {
				_ = conn.Close()
				b.Error(err)
				return
			}
			if _, err := io.ReadFull(conn, response); err != nil {
				_ = conn.Close()
				b.Error(err)
				return
			}
			if err := benchmarkClose(conn); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

func benchmarkEchoServer(b *testing.B) net.Listener {
	b.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				if tcp, ok := conn.(*net.TCPConn); ok {
					_ = tcp.SetLinger(0)
				}
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener
}

func benchmarkClose(conn net.Conn) error {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	return conn.Close()
}

func benchmarkForwarder(b *testing.B, backendAddress string) (*Forwarder, string) {
	b.Helper()
	forwarder := NewForwarder("127.0.0.1:0", staticDialer{backendAddress}, nil)
	if err := forwarder.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	return forwarder, forwarder.listener.Addr().String()
}

func benchmarkDial(b *testing.B, address string) net.Conn {
	b.Helper()
	conn, err := net.DialTimeout("tcp4", address, 3*time.Second)
	if err != nil {
		b.Fatal(err)
	}
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		_ = conn.Close()
		b.Fatal(err)
	}
	return conn
}
