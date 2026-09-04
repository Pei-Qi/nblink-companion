package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

type staticDialer struct{ address string }

func (d staticDialer) DialBackend(ctx context.Context) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp4", d.address)
}

func TestForwarderCopiesBothDirections(t *testing.T) {
	backend, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	go func() {
		conn, err := backend.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()

	port := freePort(t)
	forwarder := NewForwarder("127.0.0.1:"+strconv.Itoa(port), staticDialer{backend.Addr().String()}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := forwarder.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer forwarder.Stop()

	conn, err := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "hello\n" {
		t.Fatalf("unexpected response %q", line)
	}
}

func TestForwarderPreservesHalfClose(t *testing.T) {
	backend, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	go func() {
		conn, err := backend.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		data, _ := io.ReadAll(conn)
		_, _ = conn.Write(append([]byte("received:"), data...))
	}()

	port := freePort(t)
	forwarder := NewForwarder("127.0.0.1:"+strconv.Itoa(port), staticDialer{backend.Addr().String()}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := forwarder.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer forwarder.Stop()

	raw, err := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn := raw.(*net.TCPConn)
	defer conn.Close()
	_, _ = conn.Write([]byte("payload"))
	if err := conn.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != "received:payload" {
		t.Fatalf("unexpected response %q", response)
	}
}

func TestForwarderPreservesBinaryDataAcrossConcurrentConnections(t *testing.T) {
	backend, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	port := freePort(t)
	forwarder := NewForwarder("127.0.0.1:"+strconv.Itoa(port), staticDialer{backend.Addr().String()}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := forwarder.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer forwarder.Stop()

	payload := make([]byte, 2<<20)
	for i := range payload {
		payload[i] = byte((i * 31) % 251)
	}
	const clients = 12
	var wg sync.WaitGroup
	errors := make(chan error, clients)
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
			if err != nil {
				errors <- err
				return
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				errors <- err
				return
			}
			writeDone := make(chan error, 1)
			go func() {
				_, err := io.Copy(conn, bytes.NewReader(payload))
				writeDone <- err
			}()
			response := make([]byte, len(payload))
			if _, err := io.ReadFull(conn, response); err != nil {
				errors <- err
				return
			}
			if err := <-writeDone; err != nil {
				errors <- err
				return
			}
			if !bytes.Equal(response, payload) {
				errors <- fmt.Errorf("binary payload changed")
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
