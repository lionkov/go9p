package main

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"github.com/lionkov/go9p/p/srv"
)

func startTLSRamfsServer(t *testing.T) (addr string, stop func()) {
	t.Helper()

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

	cert := make([]tls.Certificate, 1)
	cert[0].Certificate = [][]byte{testCertificate}
	cert[0].PrivateKey = testPrivateKey

	ls, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Rand:         rand.Reader,
		Certificates: cert,
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- rsrv.srv.StartListener(ls) }()

	return ls.Addr().String(), func() {
		_ = ls.Close()
		if err := <-errCh; err != nil &&
			!errors.Is(err, net.ErrClosed) &&
			!errors.Is(err, os.ErrClosed) &&
			!strings.Contains(err.Error(), "use of closed network connection") {
			t.Fatalf("server: %v", err)
		}
	}
}

func TestTLSRamfs_TLSClientCRUD(t *testing.T) {
	addr, stop := startTLSRamfsServer(t)
	defer stop()

	c, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	defer c.Close()

	user := p.OsUsers.Uid2User(os.Geteuid())
	m, err := clnt.MountConn(c, "", 8192, user)
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	defer m.Unmount()

	f, err := m.FCreate("hello.txt", 0o666, p.OWRITE|p.OTRUNC)
	if err != nil {
		t.Fatalf("fcreate: %v", err)
	}
	want := []byte("hello tlsramfs\n")
	if _, err := f.Write(want); err != nil {
		t.Fatalf("write: %v", err)
	}
	f.Close()

	r, err := m.FOpen("hello.txt", p.OREAD)
	if err != nil {
		t.Fatalf("fopen: %v", err)
	}
	var got []byte
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}
	if string(got) != string(want) {
		t.Fatalf("got %q want %q", string(got), string(want))
	}
	r.Close()
}

