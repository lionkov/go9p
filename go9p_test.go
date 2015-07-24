// Copyright 2009 The go9p Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io"
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
var numDir = flag.Int("numdir", 16384, "Number of directory entries for readdir testing")

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
	tmpDir, err := ioutil.TempDir("", "go9")
	if err != nil {
		t.Fatal("Can't create temp directory")
	}
	//defer os.RemoveAll(tmpDir)
	ufs.Root = tmpDir

	t.Logf("ufs starting in %v\n", tmpDir)
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
	// packet debugging on clients is broken.
	clnt.Debuglevel = 0 // *debug
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

	// Now create a whole bunch of files to test readdir
	for i := 0; i < *numDir; i++ {
		f := fmt.Sprintf(path.Join(tmpDir, fmt.Sprintf("%d", i)))
		if err := ioutil.WriteFile(f, []byte(f), 0600); err != nil {
			t.Fatalf("Create %v: got %v, want nil", f, err)
		}
	}

	if err = clnt.Open(dirfid, 0); err != nil {
		t.Fatalf("%v", err)
	}
	var b []byte
	if b, err = clnt.Read(dirfid, 0, 64*1024); err != nil {
		t.Fatalf("%v", err)
	}
	var i, amt int
	var offset uint64
	err = nil

	for err == nil {
		if b, err = clnt.Read(dirfid, offset, 64*1024); err != nil {
			t.Fatalf("%v", err)
		}
		t.Logf("clnt.Read returns [%v,%v]", len(b), err)
		if len(b) == 0 {
			break
		}
		for b != nil && len(b) > 0 {
			var d *p.Dir
			if d, b, amt, err = p.UnpackDir(b, ufs.Dotu); err != nil {
				t.Errorf("UnpackDir returns %v", err)
				break
			} else {
				if *debug > 128 {
					t.Logf("Entry %d: %v", i, d)
				}
				i++
				offset += uint64(amt)
			}
		}
	}
	if i != *numDir {
		t.Fatalf("Reading %v: got %d entries, wanted %d, err %v", tmpDir, i, *numDir, err)
	}

	t.Logf("-----------------------------> Alternate form, using readdir and File")
	// Alternate form, using readdir and File
	dirfile, err := clnt.FOpen(".", p.OREAD)
	if err != nil {
		t.Fatalf("%v", err)
	}
	i, amt, offset = 0, 0, 0
	err = nil
	passes := 0

	for err == nil {
		d, err := dirfile.Readdir(*numDir)
		if err != nil && err != io.EOF {
			t.Errorf("%v", err)
		}

		if len(d) == 0 {
			break
		}
		i += len(d)
		if i >= *numDir {
			break
		}
		if passes > *numDir {
			t.Fatalf("%d iterations, %d read: no progress", passes, i)
		}
		passes++
	}
	if i != *numDir {
		t.Fatalf("Readdir %v: got %d entries, wanted %d", tmpDir, i, *numDir)
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
	ufs.Root = tmpDir
	t.Log("ufs starting in %v", tmpDir)
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
	from = to
	d.Name = "/c"
	if err = clnt.Wstat(f, d); err != nil {
		t.Errorf("%v", err)
	}

	// the old one should be gone, and the new one should be there.
	if _, err = ioutil.ReadFile(from); err == nil {
		t.Errorf("ReadFile(%v): got nil, want err", from)
	}

	to = path.Join(tmpDir, d.Name)
	if _, err = ioutil.ReadFile(to); err != nil {
		t.Errorf("ReadFile(%v): got %v, want nil", to, err)
	}

	// Make sure they can't walk out of the root.

	from = to
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
