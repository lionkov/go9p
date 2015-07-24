// Copyright 2009 The go9p Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"io/ioutil"
	"net"
	"os"
	"path"
	"testing"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"github.com/lionkov/go9p/p/srv/ufs"
)

var debug = flag.Int("debug", 0, "print debug messages")

// Two files, dotu was true.
var testunpackbytes = []byte{
	79, 0, 0, 0, 0, 0, 0, 0, 0, 228, 193, 233, 248, 44, 145, 3, 0, 0, 0, 0, 0, 164, 1, 0, 0, 0, 0, 0, 0, 47, 117, 180, 83, 102, 3, 0, 0, 0, 0, 0, 0, 6, 0, 112, 97, 115, 115, 119, 100, 4, 0, 110, 111, 110, 101, 4, 0, 110, 111, 110, 101, 4, 0, 110, 111, 110, 101, 0, 0, 232, 3, 0, 0, 232, 3, 0, 0, 255, 255, 255, 255, 78, 0, 0, 0, 0, 0, 0, 0, 0, 123, 171, 233, 248, 42, 145, 3, 0, 0, 0, 0, 0, 164, 1, 0, 0, 0, 0, 0, 0, 41, 117, 180, 83, 195, 0, 0, 0, 0, 0, 0, 0, 5, 0, 104, 111, 115, 116, 115, 4, 0, 110, 111, 110, 101, 4, 0, 110, 111, 110, 101, 4, 0, 110, 111, 110, 101, 0, 0, 232, 3, 0, 0, 232, 3, 0, 0, 255, 255, 255, 255,
}

func TestUnpackDir(t *testing.T) {
	b := testunpackbytes
	for len(b) > 0 {
		var err error
		if _, b, _, err = p.UnpackDir(b, true); err != nil {
			t.Fatalf("Unpackdir: %v", err)
		}
	}
}

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
	l, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatalf("Can not start listener: %v", err)
	}
	srvAddr := l.Addr().String()
	t.Logf("Server is at %v", srvAddr)
	go func() {
		if err = ufs.StartListener(l); err != nil {
			t.Fatalf("Can not start listener: %v", err)
		}
	}()
	var conn net.Conn
	if conn, err = net.Dial("tcp", srvAddr); err != nil {
		t.Fatalf("%v", err)
	} else {
		t.Logf("Got a conn, %v\n", conn)
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
	tmpDir, err := ioutil.TempDir("", "go9")
	if err != nil {
		t.Fatal("Can't create temp directory")
	}
	defer os.RemoveAll(tmpDir)

	t.Log("ufs starting\n")
	// determined by build tags
	//extraFuncs()
	l, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatalf("Can not start listener: %v", err)
	}
	srvAddr := l.Addr().String()
	t.Logf("Server is at %v", srvAddr)
	go func() {
		if err = ufs.StartListener(l); err != nil {
			t.Fatalf("Can not start listener: %v", err)
		}
	}()
	var conn net.Conn
	if conn, err = net.Dial("tcp", srvAddr); err != nil {
		t.Fatalf("%v", err)
	} else {
		t.Logf("Got a conn, %v\n", conn)
	}

	clnt := clnt.NewClnt(conn, 8192, false)
	root := p.OsUsers.Uid2User(0)
	rootfid, err := clnt.Attach(nil, root, tmpDir)
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

func TestRename(t *testing.T) {
	var err error
	flag.Parse()
	ufs := new(ufs.Ufs)
	ufs.Dotu = false
	ufs.Id = "ufs"
	ufs.Debuglevel = *debug
	ufs.Msize = 8192
	ufs.Start(ufs)

	tmpDir, err := ioutil.TempDir("", "go9")
	if err != nil {
		t.Fatal("Can't create temp directory")
	}
	defer os.RemoveAll(tmpDir)
	t.Log("ufs starting\n")
	// determined by build tags
	//extraFuncs()
	l, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatalf("Can not start listener: %v", err)
	}
	srvAddr := l.Addr().String()
	t.Logf("Server is at %v", srvAddr)
	go func() {
		if err = ufs.StartListener(l); err != nil {
			t.Fatalf("Can not start listener: %v", err)
		}
	}()
	var conn net.Conn
	if conn, err = net.Dial("tcp", srvAddr); err != nil {
		t.Fatalf("%v", err)
	} else {
		t.Logf("Got a conn, %v\n", conn)
	}

	clnt := clnt.NewClnt(conn, 8192, false)
	root := p.OsUsers.Uid2User(0)
	rootfid, err := clnt.Attach(nil, root, tmpDir)
	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("attached, rootfid %v\n", rootfid)
	// OK, create a file behind go9ps back and then rename it.
	b := make([]byte, 0)
	from := path.Join(tmpDir, "a")
	to := path.Join(tmpDir, "b")
	if err = ioutil.WriteFile(from, b, 0666); err != nil {
		t.Fatalf("%v", err)
	}

	f := clnt.FidAlloc()
	if _, err = clnt.Walk(rootfid, f, []string{"a"}); err != nil {
		t.Fatalf("%v", err)
	}
	d, err := clnt.Stat(f)
	if err != nil {
		t.Fatalf("%v", err)
	}
	d.Name = "b"
	if err = clnt.Wstat(f, d); err != nil {
		t.Errorf("%v", err)
	}
	// the old one should be gone, and the new one should be there.
	if _, err = ioutil.ReadFile(from); err == nil {
		t.Errorf("ReadFile(%v): got nil, want err", from)
	}

	if _, err = ioutil.ReadFile(to); err != nil {
		t.Errorf("ReadFile(%v): got %v, want nil", from, err)
	}

	// now try with an absolute path, which is supported on ufs servers.
	// It's not guaranteed to work on all servers, but it is hugely useful
	// on those that can do it -- which is almost all of them, save Plan 9
	// of course.
	d.Name = path.Join(tmpDir, "c")
	if err = clnt.Wstat(f, d); err != nil {
		t.Errorf("%v", err)
	}

	// the old one should be gone, and the new one should be there.
	if _, err = ioutil.ReadFile(to); err == nil {
		t.Errorf("ReadFile(%v): got nil, want err", to)
	}

	if _, err = ioutil.ReadFile(d.Name); err != nil {
		t.Errorf("ReadFile(%v): got %v, want nil", d.Name, err)
	}

	// And, finally, make sure they can't walk out of the root.

	from = d.Name
	d.Name = "../../../../d"
	if err = clnt.Wstat(f, d); err != nil {
		t.Errorf("%v", err)
	}

	// the old one should be gone, and the new one should be there.
	if _, err = ioutil.ReadFile(from); err == nil {
		t.Errorf("ReadFile(%v): got nil, want err", from)
	}

	to = path.Join(tmpDir, "d")
	if _, err = ioutil.ReadFile(to); err != nil {
		t.Errorf("ReadFile(%v): got %v, want nil", to, err)
	}

}
