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

func startRamfsServer(t *testing.T) (addr string, stop func()) {
	t.Helper()

	// Minimal server init, matching the example's semantics.
	rsrv.user = p.OsUsers.Uid2User(os.Geteuid())
	rsrv.group = p.OsUsers.Gid2Group(os.Getegid())
	rsrv.blksz = 8192
	rsrv.blkchan = make(chan []byte, 16)
	rsrv.zero = make([]byte, rsrv.blksz)

	root := new(RFile)
	if err := root.Add(nil, "/", rsrv.user, nil, p.DMDIR|0777, root); err != nil {
		t.Fatalf("root.Add: %v", err)
	}

	rsrv.srv = srv.NewFileSrv(&root.File)
	rsrv.srv.Dotu = true
	rsrv.srv.Start(rsrv.srv)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- rsrv.srv.StartListener(ln) }()

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

func TestRamfs_E2E_CRUD(t *testing.T) {
	addr, stop := startRamfsServer(t)
	defer stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	c := clnt.NewClnt(conn, 8192, true)
	defer c.Unmount()

	user := p.OsUsers.Uid2User(os.Geteuid())
	root, err := c.Attach(nil, user, "/")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	fid := c.FidAlloc()
	if _, err := c.Walk(root, fid, []string{}); err != nil {
		t.Fatalf("walk root: %v", err)
	}
	if err := c.Create(fid, "hello.txt", 0o666, p.OWRITE|p.OTRUNC, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	want := []byte("hello ramfs\n")
	if n, err := c.Write(fid, want, 0); err != nil {
		t.Fatalf("write: %v", err)
	} else if n != len(want) {
		t.Fatalf("write n=%d want=%d", n, len(want))
	}

	rfid := c.FidAlloc()
	if _, err := c.Walk(root, rfid, []string{"hello.txt"}); err != nil {
		t.Fatalf("walk file: %v", err)
	}
	if err := c.Open(rfid, p.OREAD); err != nil {
		t.Fatalf("open: %v", err)
	}
	got, err := c.Read(rfid, 0, 64*1024)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q want %q", string(got), string(want))
	}

	if err := c.Remove(rfid); err != nil {
		t.Fatalf("remove: %v", err)
	}
}

