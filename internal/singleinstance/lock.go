package singleinstance

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	controlAddress  = "127.0.0.1:42391"
	activateRequest = "NBLINK/1 ACTIVATE\n"
	activateReply   = "NBLINK/1 OK\n"
)

var ErrActivated = errors.New("existing instance activated")

type Lock struct {
	listener net.Listener
	activate chan struct{}
	done     chan struct{}
}

func Acquire() (*Lock, error) {
	return acquireAt(controlAddress)
}

func acquireAt(address string) (*Lock, error) {
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		if activateExisting(address) == nil {
			return nil, ErrActivated
		}
		return nil, fmt.Errorf("single-instance control address unavailable: %w", err)
	}
	lock := &Lock{
		listener: listener,
		activate: make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	go lock.acceptLoop()
	return lock, nil
}

func activateExisting(address string) error {
	conn, err := net.DialTimeout("tcp4", address, 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte(activateRequest)); err != nil {
		return err
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if reply != activateReply {
		return errors.New("unexpected activation reply")
	}
	return nil
}

func (l *Lock) Activations() <-chan struct{} {
	return l.activate
}

func (l *Lock) acceptLoop() {
	defer close(l.done)
	defer close(l.activate)
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			return
		}
		go l.handle(conn)
	}
}

func (l *Lock) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	request, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || request != activateRequest {
		return
	}
	if _, err := conn.Write([]byte(activateReply)); err != nil {
		return
	}
	select {
	case l.activate <- struct{}{}:
	default:
	}
}

func (l *Lock) Close() error {
	if l == nil || l.listener == nil {
		return nil
	}
	err := l.listener.Close()
	<-l.done
	return err
}
