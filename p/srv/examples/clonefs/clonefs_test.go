package main

import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"github.com/lionkov/go9p/p/srv"
)

func startClonefsServer(t *testing.T) (addr string, stop func()) {
	t.Helper()

	user := p.OsUsers.Uid2User(os.Geteuid())
	root = new(srv.File)
	if err := root.Add(nil, "/", user, nil, p.DMDIR|0777, nil); err != nil {
		t.Fatalf("root.Add: %v", err)
	}
	cl := new(Clone)
	if err := cl.Add(root, "clone", user, nil, 0444, cl); err != nil {
		t.Fatalf("add clone: %v", err)
	}

	s := srv.NewFileSrv(root)
	s.Dotu = true
	s.Start(s)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.StartListener(ln) }()

	return ln.Addr().String(), func() {
		_ = ln.Close()
		if err := <-errCh; err != nil &&
			!errors.Is(err, net.ErrClosed) &&
			!errors.Is(err, os.ErrClosed) &&
			!strings.Contains(err.Error(), "use of closed network connection") {
			t.Fatalf("server: %v", err)
		}
	}
}

func TestClonefs_CloneCreatesWritableFile(t *testing.T) {
	addr, stop := startClonefsServer(t)
	defer stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	c := clnt.NewClnt(conn, 8192, true)
	defer c.Unmount()

	user := p.OsUsers.Uid2User(os.Geteuid())
	rootfid, err := c.Attach(nil, user, "/")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	// Read /clone to create a new entry and get its name.
	clone := c.FidAlloc()
	if _, err := c.Walk(rootfid, clone, []string{"clone"}); err != nil {
		t.Fatalf("walk clone: %v", err)
	}
	if err := c.Open(clone, p.OREAD); err != nil {
		t.Fatalf("open clone: %v", err)
	}
	b, err := c.Read(clone, 0, 64)
	if err != nil {
		t.Fatalf("read clone: %v", err)
	}
	name := strings.TrimSpace(string(b))
	if name == "" {
		t.Fatalf("clone returned empty name")
	}

	// The created file should exist and be writable.
	fid := c.FidAlloc()
	if _, err := c.Walk(rootfid, fid, []string{name}); err != nil {
		t.Fatalf("walk new file %q: %v", name, err)
	}
	if err := c.Open(fid, p.OWRITE|p.OTRUNC); err != nil {
		t.Fatalf("open write %q: %v", name, err)
	}
	want := []byte("abc\n")
	if n, err := c.Write(fid, want, 0); err != nil {
		t.Fatalf("write %q: %v", name, err)
	} else if n != len(want) {
		t.Fatalf("write n=%d want=%d", n, len(want))
	}

	rfid := c.FidAlloc()
	if _, err := c.Walk(rootfid, rfid, []string{name}); err != nil {
		t.Fatalf("walk read %q: %v", name, err)
	}
	if err := c.Open(rfid, p.OREAD); err != nil {
		t.Fatalf("open read %q: %v", name, err)
	}
	got, err := c.Read(rfid, 0, 64)
	if err != nil {
		t.Fatalf("read %q: %v", name, err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q want %q", string(got), string(want))
	}
}

