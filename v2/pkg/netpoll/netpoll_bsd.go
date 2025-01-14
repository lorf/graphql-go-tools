//go:build darwin || netbsd || freebsd || openbsd || dragonfly
// +build darwin netbsd freebsd openbsd dragonfly

package netpoll

import (
	"errors"
	"net"
	"sync"
	"syscall"
	"time"
)

var _ Poller = (*KQueue)(nil)

// KQueue is a poll based connection implementation.
type KQueue struct {
	fd int
	ts syscall.Timespec

	connBufferSize int
	mu             *sync.RWMutex
	changes        []syscall.Kevent_t
	conns          map[int]net.Conn
	connbuf        []net.Conn

	events []syscall.Kevent_t
}

// NewPoller creates a new poller instance.
func NewPoller(connBufferSize int, pollTimeout time.Duration) (*KQueue, error) {
	return newPollerWithBuffer(connBufferSize, pollTimeout)
}

// newPollerWithBuffer creates a new poller instance with buffer size.
func newPollerWithBuffer(count int, pollTimeout time.Duration) (*KQueue, error) {
	p, err := syscall.Kqueue()
	if err != nil {
		panic(err)
	}
	_, err = syscall.Kevent(p, []syscall.Kevent_t{{
		Ident:  0,
		Filter: syscall.EVFILT_USER,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
	}}, nil, nil)
	if err != nil {
		panic(err)
	}

	return &KQueue{
		fd:             p,
		ts:             syscall.NsecToTimespec(pollTimeout.Nanoseconds()),
		connBufferSize: count,
		mu:             &sync.RWMutex{},
		conns:          make(map[int]net.Conn),
		connbuf:        make([]net.Conn, count),
	}, nil
}

// Close closes the poller.
func (e *KQueue) Close(closeConns bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if closeConns {
		for _, conn := range e.conns {
			conn.Close()
		}
	}

	e.conns = nil
	e.changes = nil
	e.connbuf = e.connbuf[:0]

	return syscall.Close(e.fd)
}

// Add adds a network connection to the poller.
func (e *KQueue) Add(conn net.Conn) error {
	conn = newConnImpl(conn)
	fd := SocketFD(conn)
	if e := syscall.SetNonblock(int(fd), true); e != nil {
		return errors.New("udev: unix.SetNonblock failed")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.changes = append(e.changes,
		syscall.Kevent_t{
			Ident: uint64(fd), Flags: syscall.EV_ADD | syscall.EV_EOF, Filter: syscall.EVFILT_READ,
		},
	)

	e.conns[fd] = conn

	return nil
}

// Remove removes a connection from the poller.
// If close is true, the connection will be closed.
func (e *KQueue) Remove(conn net.Conn) error {
	fd := SocketFD(conn)

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.changes) <= 1 {
		e.changes = nil
	} else {
		changes := make([]syscall.Kevent_t, 0, len(e.changes)-1)
		ident := uint64(fd)
		for _, ke := range e.changes {
			if ke.Ident != ident {
				changes = append(changes, ke)
			}
		}
		e.changes = changes
	}

	delete(e.conns, fd)

	return nil
}

// Wait waits for events and returns the connections.
func (e *KQueue) Wait(count int) ([]net.Conn, error) {
	if len(e.events) != count {
		e.events = make([]syscall.Kevent_t, count)
	}

	e.mu.RLock()
	changes := e.changes
	e.mu.RUnlock()

retry:
	n, err := syscall.Kevent(e.fd, changes, e.events, &e.ts)
	if err != nil {
		if err == syscall.EINTR {
			goto retry
		}
		return nil, err
	}

	var conns []net.Conn
	if e.connBufferSize == 0 {
		conns = make([]net.Conn, 0, n)
	} else {
		conns = e.connbuf[:0]
	}

	e.mu.RLock()
	for i := 0; i < n; i++ {
		conn := e.conns[int(e.events[i].Ident)]
		if conn != nil {
			conns = append(conns, conn)
		}
	}
	e.mu.RUnlock()

	return conns, nil
}
