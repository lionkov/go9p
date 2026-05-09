package clnt

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"strings"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/srv/ufs"
)

func startUFSServer(t *testing.T, root string) (addr string, stop func()) {
	t.Helper()

	s := new(ufs.Ufs)
	s.Dotu = false
	s.Id = "ufs"
	s.Msize = 8192
	s.Root = root
	if ok := s.Start(s); !ok {
		t.Fatalf("ufs.Start returned false")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.StartListener(ln) }()

	stop = func() {
		_ = ln.Close()
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			// macOS/Linux may surface different close errors; only fail if it's not a close.
			if !errors.Is(err, os.ErrClosed) && !strings.Contains(err.Error(), "use of closed network connection") {
				t.Fatalf("server error: %v", err)
			}
		}
	}
	return ln.Addr().String(), stop
}

func dialClient(t *testing.T, addr string) (*Clnt, func()) {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c := NewClnt(conn, 8192, false)
	return c, func() {
		c.Unmount()
		_ = conn.Close()
	}
}

func TestE2E_UFS_ClientServer_CRUD(t *testing.T) {
	tmp := t.TempDir()
	addr, stop := startUFSServer(t, tmp)
	defer stop()

	c, cleanup := dialClient(t, addr)
	defer cleanup()

	user := p.OsUsers.Uid2User(os.Geteuid())
	root, err := c.Attach(nil, user, "/")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	// Create a file.
	fid := c.FidAlloc()
	if _, err := c.Walk(root, fid, []string{"."}); err != nil {
		t.Fatalf("walk .: %v", err)
	}

	const (
		name = "hello.txt"
		perm = 0666
	)
	if err := c.Create(fid, name, perm, p.OWRITE|p.OTRUNC, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	want := []byte("hello 9p\n")
	if n, err := c.Write(fid, want, 0); err != nil {
		t.Fatalf("write: %v", err)
	} else if n != len(want) {
		t.Fatalf("write: wrote %d bytes, want %d", n, len(want))
	}

	// Stat should reflect size.
	d, err := c.Stat(fid)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if d.Length != uint64(len(want)) {
		t.Fatalf("stat length: got %d want %d", d.Length, len(want))
	}

	// Read back using a fresh fid.
	rfid := c.FidAlloc()
	if _, err := c.Walk(root, rfid, []string{name}); err != nil {
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
		t.Fatalf("read content mismatch: got %q want %q", string(got), string(want))
	}

	// Rename and verify on disk (ufs is a real FS export).
	if err := c.Rename(rfid, "renamed.txt"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "renamed.txt")); err != nil {
		t.Fatalf("os.Stat renamed: %v", err)
	}

	// Remove via 9p.
	if err := c.Remove(rfid); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "renamed.txt")); err == nil {
		t.Fatalf("expected removed file to be gone")
	}
}

