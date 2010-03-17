// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The srv package provides definitions and functions used to implement
// a 9P2000 file client.
package clnt

import (
	"fmt"
	"log"
	"net"
	"sync"
	"syscall"
    "go9p.googlecode.com/hg/p"

)

// The Clnt type represents a 9P2000 client. The client is connected to
// a 9P2000 file server and its methods can be used to access and manipulate
// the files exported by the server.
type Clnt struct {
	sync.Mutex
	Finished   bool   // client is no longer connected to server
	Debuglevel int    // =0 don't print anything, >0 print Fcalls, >1 print raw packets
	Msize      uint32 // Maximum size of the 9P messages
	Dotu       bool   // If true, 9P2000.u protocol is spoken
	Root       *Fid   // Fid that points to the rood directory

	conn     net.Conn
	tagpool  *pool
	fidpool  *pool
	reqout   chan *req
	done     chan bool
	reqfirst *req
	reqlast  *req
	err      *p.Error
}

// A Fid type represents a file on the server. Fids are used for the
// low level methods that correspond directly to the 9P2000 message requests
type Fid struct {
	sync.Mutex
	Clnt   *Clnt // Client the fid belongs to
	Iounit uint32
	p.Qid         // The Qid description for the file
	Mode   uint8  // Open mode (one of p.O* values) (if file is open)
	Fid    uint32 // Fid number
	p.User        // The user the fid belongs to
}

// The file is similar to the Fid, but is used in the high-level client
// interface.
type File struct {
	fid    *Fid
	offset uint64
}

type pool struct {
	sync.Mutex
	need  int
	nchan chan uint32
	maxid uint32
	imap  []byte
}

type req struct {
	sync.Mutex
	clnt       *Clnt
	tc         *p.Fcall
	rc         *p.Fcall
	err        *p.Error
	done       chan *req
	prev, next *req
}

func (clnt *Clnt) rpcnb(r *req) *p.Error {
	var tag uint16

	if clnt.Finished {
		return &p.Error{"Client no longer connected", 0}
	}
	if r.tc.Type == p.Tversion {
		tag = p.NOTAG
	} else {
		tag = uint16(clnt.tagpool.getId())
	}

	p.SetTag(r.tc, tag)
	clnt.Lock()
	if clnt.err != nil {
		clnt.Unlock()
		return clnt.err
	}

	if clnt.reqlast != nil {
		clnt.reqlast.next = r
	} else {
		clnt.reqfirst = r
	}

	r.prev = clnt.reqlast
	clnt.reqlast = r
	clnt.Unlock()

	clnt.reqout <- r
	return nil
}

func (clnt *Clnt) rpc(tc *p.Fcall) (*p.Fcall, *p.Error) {
	r := new(req)
	r.tc = tc
	r.done = make(chan *req)
	err := clnt.rpcnb(r)
	if err != nil {
		return nil, err
	}

	<-r.done
	return r.rc, r.err
}

func (clnt *Clnt) recv() {
	var err *p.Error

	buf := make([]byte, clnt.Msize)
	pos := 0
	for {
		if len(buf) < int(clnt.Msize) {
			b := make([]byte, clnt.Msize)
			copy(b, buf[0:pos])
			buf = b
		}

		n, oerr := clnt.conn.Read(buf[pos:len(buf)])
		if oerr != nil || n == 0 {
			err = &p.Error{oerr.String(), syscall.EIO}
			goto closed
		}

		pos += n
		for pos > 4 {
			sz, _ := p.Gint32(buf)
			if pos < int(sz) {
				break
			}

			fc, err, fcsize := p.Unpack(buf, clnt.Dotu)
			clnt.Lock()
			if err != nil {
				clnt.err = err
				clnt.conn.Close()
				clnt.Unlock()
				goto closed
			}

			if clnt.Debuglevel > 0 {
				if clnt.Debuglevel > 1 {
					log.Stderr("}-} " + fmt.Sprint(fc.Pkt))
				}

				log.Stderr("}}} " + fc.String())
			}

			var r *req = nil
			for r = clnt.reqfirst; r != nil; r = r.next {
				if r.tc.Tag == fc.Tag {
					break
				}
			}

			if r == nil {
				clnt.err = &p.Error{"unexpected response", syscall.EINVAL}
				clnt.conn.Close()
				clnt.Unlock()
				goto closed
			}

			r.rc = fc
			if r.prev != nil {
				r.prev.next = r.next
			} else {
				clnt.reqfirst = r.next
			}

			if r.next != nil {
				r.next.prev = r.prev
			} else {
				clnt.reqlast = r.prev
			}
			clnt.Unlock()

			if r.tc.Type != r.rc.Type-1 {
				if r.rc.Type != p.Rerror {
					r.err = &p.Error{"invalid response id", syscall.EINVAL}
				} else {
					if r.err != nil {
						r.err = &p.Error{r.rc.Error, int(r.rc.Errornum)}
					}
				}
			}

			if r.tc.Tag != p.NOTAG {
				clnt.tagpool.putId(uint32(r.tc.Tag))
			}

			if r.done != nil {
				r.done <- r
			}

			pos -= fcsize
			buf = buf[0:fcsize]
		}
	}

closed:
	clnt.done <- true

	/* send error to all pending requests */
	clnt.Lock()
	r := clnt.reqfirst
	clnt.reqfirst = nil
	clnt.reqlast = nil
	err = clnt.err
	clnt.Unlock()
	for ; r != nil; r = r.next {
		r.err = err
		if r.done != nil {
			r.done <- r
		}
	}
}

func (clnt *Clnt) send() {
	for {
		select {
		case <-clnt.done:
			clnt.Finished = true
			return

		case req := <-clnt.reqout:
			if clnt.Debuglevel > 0 {
				if clnt.Debuglevel > 1 {
					log.Stderr("{-{ " + fmt.Sprint(req.tc.Pkt))
				}

				log.Stderr("{{{ " + req.tc.String())
			}

			for buf := req.tc.Pkt; len(buf) > 0; {
				n, err := clnt.conn.Write(buf)
				if err != nil {
					/* just close the socket, will get signal on clnt.done */
					clnt.conn.Close()
					break
				}

				buf = buf[n:len(buf)]
			}
		}
	}
}

// Creates and initializes a new Clnt object. Doesn't send any data
// on the wire.
func NewClnt(c net.Conn, msize uint32, dotu bool) *Clnt {
	clnt := new(Clnt)
	clnt.conn = c
	clnt.Msize = msize
	clnt.Dotu = dotu
	clnt.tagpool = newPool(uint32(p.NOTAG))
	clnt.fidpool = newPool(p.NOFID)
	clnt.reqout = make(chan *req)
	clnt.done = make(chan bool)

	go clnt.recv()
	go clnt.send()

	return clnt
}

// Establishes a new socket connection to the 9P server and creates
// a client object for it. Negotiates the dialect and msize for the
// connection. Returns a Clnt object, or Error.
func Connect(ntype, addr string, msize uint32, dotu bool) (*Clnt, *p.Error) {
	c, e := net.Dial(ntype, "", addr)
	if e != nil {
		return nil, &p.Error{e.String(), syscall.EIO}
	}

	clnt := NewClnt(c, msize, dotu)
	ver := "9P2000"
	if clnt.Dotu {
		ver = "9P2000.u"
	}

	tc := p.NewFcall(clnt.Msize)
	err := p.PackTversion(tc, clnt.Msize, ver)
	if err != nil {
		return nil, err
	}

	rc, err := clnt.rpc(tc)
	if err != nil {
		return nil, err
	}

	if rc.Msize < clnt.Msize {
		clnt.Msize = rc.Msize
	}

	clnt.Dotu = rc.Version == "9P2000.u" && clnt.Dotu
	return clnt, nil
}

// Creates a new Fid object for the client
func (clnt *Clnt) FidAlloc() *Fid {
	fid := new(Fid)
	fid.Fid = clnt.fidpool.getId()
	fid.Clnt = clnt

	return fid
}
