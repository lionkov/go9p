package main

import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"github.com/lionkov/go9p/p/srv"
)

func startTimefsServer(t *testing.T) (addr string, stop func()) {
	t.Helper()

	user := p.OsUsers.Uid2User(os.Geteuid())
	root := new(srv.File)
	if err := root.Add(nil, "/", user, nil, p.DMDIR|0555, nil); err != nil {
		t.Fatalf("root.Add: %v", err)
	}

	tm := new(Time)
	if err := tm.Add(root, "time", user, nil, 0444, tm); err != nil {
		t.Fatalf("add time: %v", err)
	}
	ntm := new(InfTime)
	if err := ntm.Add(root, "inftime", user, nil, 0444, ntm); err != nil {
		t.Fatalf("add inftime: %v", err)
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

func TestTimefs_ReadsWork(t *testing.T) {
	addr, stop := startTimefsServer(t)
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

	readFile := func(name string) string {
		fid := c.FidAlloc()
		if _, err := c.Walk(root, fid, []string{name}); err != nil {
			t.Fatalf("walk %s: %v", name, err)
		}
		if err := c.Open(fid, p.OREAD); err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		b, err := c.Read(fid, 0, 4096)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(b)
	}

	a := readFile("time")
	if a == "" {
		t.Fatalf("time read empty")
	}
	time.Sleep(10 * time.Millisecond)
	b := readFile("time")
	if b == "" {
		t.Fatalf("time read empty (second)")
	}

	inf := readFile("inftime")
	if inf == "" {
		t.Fatalf("inftime read empty")
	}
}

