// Copyright 2009 The go9p Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"net"
	"testing"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"github.com/lionkov/go9p/p/srv/ufs"
)

var addr = flag.String("addr", ":5640", "network address")
var pipefsaddr = flag.String("pipefsaddr", ":5641", "pipefs network address")
var attachaddr = flag.String("attachaddr", ":5642", "attach test network address")
var debug = flag.Int("debug", 0, "print debug messages")

func TestAttach(t *testing.T) {
	var err error
	flag.Parse()
	ufs := new(ufs.Ufs)
	ufs.Dotu = false
	ufs.Id = "ufs"
	ufs.Debuglevel = *debug
	ufs.Start(ufs)

	t.Log("ufs starting\n")
	// determined by build tags
	//extraFuncs()
	go func() {
		if err = ufs.StartNetListener("tcp", *attachaddr); err != nil {
			t.Fatalf("Can not start listener: %v", err)
		}
	}()
	/* this may take a few tries ... */
	var conn net.Conn
	for i := 0; i < 16; i++ {
		if conn, err = net.Dial("tcp", *attachaddr); err != nil {
			t.Logf("Try go connect, %d'th try, %v", i, err)
		} else {
			t.Logf("Got a conn, %v\n", conn)
			break
		}
	}
	if err != nil {
		t.Fatalf("Connect failed after many tries ...")
	}

	root := p.OsUsers.Uid2User(0)
	clnt := clnt.NewClnt(conn, 8192, false)
	// run enough attaches to maybe let the race detector trip.
	for i := 0; i < 65536; i++ {
		_, err := clnt.Attach(nil, root, "/tmp")

		if err != nil {
			t.Fatalf("Connect failed: %v\n", err)
		}
		defer clnt.Unmount()

	}
}

func TestAttachOpenReaddir(t *testing.T) {
	var err error
	flag.Parse()
	ufs := new(ufs.Ufs)
	ufs.Dotu = false
	ufs.Id = "ufs"
	ufs.Debuglevel = *debug
	ufs.Start(ufs)
	var offset uint64

	t.Log("ufs starting\n")
	// determined by build tags
	//extraFuncs()
	go func() {
		if err = ufs.StartNetListener("tcp", *addr); err != nil {
			t.Fatalf("Can not start listener: %v", err)
		}
	}()
	/* this may take a few tries ... */
	var conn net.Conn
	for i := 0; i < 16; i++ {
		if conn, err = net.Dial("tcp", *addr); err != nil {
			t.Logf("%v", err)
		} else {
			t.Logf("Got a conn, %v\n", conn)
			break
		}
	}
	if err != nil {
		t.Fatalf("Connect failed after many tries ...")
	}

	clnt := clnt.NewClnt(conn, 8192, false)
	root := p.OsUsers.Uid2User(0)
	rootfid, err := clnt.Attach(nil, root, "/tmp")
	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("attached, rootfid %v\n", rootfid)
	dirfid := clnt.FidAlloc()
	if _, err = clnt.Walk(rootfid, dirfid, []string{"."}); err != nil {
		t.Fatalf("%v", err)
	}
	if err = clnt.Open(dirfid, 0); err != nil {
		t.Fatalf("%v", err)
	}
	var b []byte
	if b, err = clnt.Read(dirfid, 0, 64*1024); err != nil {
		t.Fatalf("%v", err)
	}
	var amt int
	for b != nil && len(b) > 0 {
		t.Logf("len(b) %v\n", len(b))
		if _, b, amt, err = p.UnpackDir(b, ufs.Dotu); err != nil {
			break
		} else {
			offset += uint64(amt)
		}
	}
	// now test partial reads.
	// Read 128 bytes at a time. Remember the last successful offset.
	// if UnpackDir fails, read again from that offset
	t.Logf("NOW TRY PARTIAL")

	for {
		var b []byte
		if b, err = clnt.Read(dirfid, offset, 128); err != nil {
			t.Fatalf("%v", err)
		}
		if len(b) == 0 {
			break
		}
		t.Logf("b %v\n", b)
		for b != nil && len(b) > 0 {
			t.Logf("len(b) %v\n", len(b))
			if d, _, amt, err := p.UnpackDir(b, ufs.Dotu); err != nil {
				// this error is expected ...
				t.Logf("unpack failed (it's ok!). retry at offset %v\n",
					offset)
				break
			} else {
				t.Logf("d %v\n", d)
				offset += uint64(amt)
			}
		}
	}
}
