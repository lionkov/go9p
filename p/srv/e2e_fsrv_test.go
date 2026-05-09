package srv

import (
	"errors"
	"net"
	"os"
	"testing"
	"strings"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
)

type memFile struct {
	File
	data []byte
}

func (f *memFile) Read(_ *FFid, buf []byte, offset uint64) (int, error) {
	f.Lock()
	defer f.Unlock()
	if offset >= uint64(len(f.data)) {
		return 0, nil
	}
	n := copy(buf, f.data[offset:])
	return n, nil
}

func (f *memFile) Write(_ *FFid, data []byte, offset uint64) (int, error) {
	f.Lock()
	defer f.Unlock()

	end := int(offset) + len(data)
	if end > len(f.data) {
		newb := make([]byte, end)
		copy(newb, f.data)
		f.data = newb
	}
	copy(f.data[offset:], data)
	f.Length = uint64(len(f.data))
	return len(data), nil
}

type memDir struct {
	File
}

func (d *memDir) Create(_ *FFid, name string, perm uint32) (*File, error) {
	m := new(memFile)
	if err := m.Add(&d.File, name, p.OsUsers.Uid2User(os.Geteuid()), nil, perm, m); err != nil {
		return nil, err
	}
	return &m.File, nil
}

func (f *memFile) Remove(_ *FFid) error { return nil }

func startFsrv(t *testing.T, root *File) (addr string, stop func()) {
	t.Helper()

	s := NewFileSrv(root)
	s.Dotu = false
	s.Id = "fsrv"
	s.Msize = 8192
	if ok := s.Start(s); !ok {
		t.Fatalf("fsrv.Start returned false")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.StartListener(ln) }()

	stop = func() {
		_ = ln.Close()
		if err := <-errCh; err != nil &&
			!errors.Is(err, net.ErrClosed) &&
			!errors.Is(err, os.ErrClosed) &&
			!strings.Contains(err.Error(), "use of closed network connection") {
			t.Fatalf("server error: %v", err)
		}
	}

	return ln.Addr().String(), stop
}

func TestE2E_Fsrv_ClientServer_SyntheticTree(t *testing.T) {
	// Build a synthetic in-memory tree.
	rootDir := new(memDir)
	user := p.OsUsers.Uid2User(os.Geteuid())
	if err := rootDir.Add(nil, "/", user, nil, p.DMDIR|0777, rootDir); err != nil {
		t.Fatalf("add root: %v", err)
	}

	addr, stop := startFsrv(t, &rootDir.File)
	defer stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	c := clnt.NewClnt(conn, 8192, false)
	defer c.Unmount()

	r, err := c.Attach(nil, user, "/")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	// Keep an unopened copy of the root fid for future walks.
	rootfid := c.FidAlloc()
	if _, err := c.Walk(r, rootfid, nil); err != nil {
		t.Fatalf("clone root fid: %v", err)
	}

	// Create and write.
	cfid := c.FidAlloc()
	if _, err := c.Walk(r, cfid, nil); err != nil {
		t.Fatalf("clone fid for create: %v", err)
	}
	if err := c.Create(cfid, "a.txt", 0666, p.OWRITE|p.OTRUNC, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	payload := []byte("abc123")
	if n, err := c.Write(cfid, payload, 0); err != nil {
		t.Fatalf("write: %v", err)
	} else if n != len(payload) {
		t.Fatalf("write: wrote %d want %d", n, len(payload))
	}

	// Readdir should include a.txt.
	if err := c.Open(r, p.OREAD); err != nil {
		t.Fatalf("open root: %v", err)
	}
	b, err := c.Read(r, 0, 64*1024)
	if err != nil {
		t.Fatalf("read root dir: %v", err)
	}
	found := false
	for len(b) > 0 {
		d, rest, _, uerr := p.UnpackDir(b, false)
		if uerr != nil {
			t.Fatalf("unpackdir: %v", uerr)
		}
		if d.Name == "a.txt" {
			found = true
			break
		}
		b = rest
	}
	if !found {
		t.Fatalf("expected a.txt in root listing")
	}

	// Read back via a new fid.
	rf := c.FidAlloc()
	if _, err := c.Walk(rootfid, rf, []string{"a.txt"}); err != nil {
		t.Fatalf("walk a.txt: %v", err)
	}
	if err := c.Open(rf, p.OREAD); err != nil {
		t.Fatalf("open a.txt: %v", err)
	}
	got, err := c.Read(rf, 0, 64*1024)
	if err != nil {
		t.Fatalf("read a.txt: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload mismatch: got %q want %q", string(got), string(payload))
	}
}

