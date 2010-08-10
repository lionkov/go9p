// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package srv

import (
	"fmt"
	"log"
	"net"
	"os"
	"go9p.googlecode.com/hg/p"
)

func newConn(srv *Srv, c net.Conn) {
	conn := new(Conn)
	conn.Srv = srv
	conn.Msize = srv.Msize
	conn.Dotu = srv.Dotu
	conn.conn = c
	conn.fidpool = make(map[uint32]*Fid)
	conn.reqout = make(chan *Req, srv.Maxpend)
	conn.done = make(chan bool)
	conn.rchan = make(chan *p.Fcall, 64)
	conn.prev = nil

	srv.Lock()
	conn.next = srv.connlist
	srv.connlist = conn
	srv.Unlock()

	if op, ok := (conn.Srv.ops).(ConnOps); ok {
		op.ConnOpened(conn)
	}

	go conn.recv()
	go conn.send()
}

func (conn *Conn) recv() {
	var err os.Error
	var n int

	buf := make([]byte, conn.Msize*8)
	pos := 0
	for {
		if len(buf) < int(conn.Msize) {
		resize:
			b := make([]byte, conn.Msize*8)
			copy(b, buf[0:pos])
			buf = b
			b = nil
		}

		n, err = conn.conn.Read(buf[pos:])
		if err != nil || n == 0 {
			goto closed
		}

		pos += n
		for pos > 4 {
			sz, _ := p.Gint32(buf)
			if sz > conn.Msize {
				log.Stderr("bad client connection: ", conn.conn.RemoteAddr())
				conn.conn.Close()
				goto closed
			}
			if pos < int(sz) {
				if len(buf) < int(sz) {
					goto resize
				}

				break
			}
			fc, err, fcsize := p.Unpack(buf, conn.Dotu)
			if err != nil {
				log.Stderr(fmt.Sprintf("invalid packet : %v %v", err, buf))
				conn.conn.Close()
				goto closed
			}

			req := new(Req)
			rc, ok := <-conn.rchan
			if !ok {
				rc = p.NewFcall(conn.Msize)
			}

			req.Conn = conn
			req.Tc = fc
			req.Rc = rc
			if conn.Srv.Debuglevel > 0 {
				if conn.Srv.Debuglevel > 1 {
					log.Stderr(">->", conn.Id, fmt.Sprint(req.Tc.Pkt))
				}

				log.Stderr(">>>", conn.Id, req.Tc.String())
			}

			conn.Lock()
			if conn.reqlast != nil {
				conn.reqlast.next = req
			} else {
				conn.reqfirst = req
			}
			req.prev = conn.reqlast
			conn.reqlast = req
			conn.Unlock()
			if conn.Srv.Ngoroutines == 0 {
				go req.process()
			} else {
				conn.Srv.Reqin <- req
			}
			buf = buf[fcsize:]
			pos -= fcsize
		}
	}

closed:
	conn.done <- true
	conn.Srv.Lock()
	if conn.prev != nil {
		conn.prev.next = conn.next
	} else {
		conn.Srv.connlist = conn.next
	}

	if conn.next != nil {
		conn.next.prev = conn.prev
	}
	conn.Srv.Unlock()

	if op, ok := (conn.Srv.ops).(ConnOps); ok {
		op.ConnClosed(conn)
	}

	/* call FidDestroy for all remaining fids */
	if op, ok := (conn.Srv.ops).(FidOps); ok {
		for _, fid := range conn.fidpool {
			op.FidDestroy(fid)
		}
	}
}

func (conn *Conn) send() {
	for {
		select {
		case <-conn.done:
			return

		case req := <-conn.reqout:
			p.SetTag(req.Rc, req.Tc.Tag)
			if conn.Srv.Debuglevel > 0 {
				if conn.Srv.Debuglevel > 1 {
					log.Stderr("<-<", conn.Id, fmt.Sprint(req.Rc.Pkt))
				}

				log.Stderr("<<<", conn.Id, req.Rc.String())
			}

			for buf := req.Rc.Pkt; len(buf) > 0; {
				n, err := conn.conn.Write(buf)
				if err != nil {
					/* just close the socket, will get signal on conn.done */
					log.Stderr("error while writing")
					conn.conn.Close()
					break
				}

				buf = buf[n:]
			}

			_ = conn.rchan <- req.Rc
		}
	}
}

func (conn *Conn) RemoteAddr() net.Addr {
	return conn.conn.RemoteAddr()
}

func (conn *Conn) LocalAddr() net.Addr {
	return conn.conn.LocalAddr()
}

// Start listening on the specified network and address for incoming
// connections. Once a connection is established, create a new Conn
// value, read messages from the socket, send them to the specified
// server, and send back responses received from the server.
func StartListener(network, laddr string, srv *Srv) os.Error {
	l, err := net.Listen(network, laddr)
	if err != nil {
		log.Stderr("listen fail: ", network, listen, err)
		return err
	}

	//go listen(l, srv);
	for {
		c, err := l.Accept()
		if err != nil {
			break
		}

		newConn(srv, c)
	}
	return nil
}

func listen(l net.Listener, srv *Srv) {
	for {
		c, err := l.Accept()
		if err != nil {
			break
		}

		newConn(srv, c)
	}
}
