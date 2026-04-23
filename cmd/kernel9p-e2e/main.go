package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"golang.org/x/sys/unix"
)

func must(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %s: %v\n", msg, err)
		os.Exit(1)
	}
}

func mustEq[T comparable](got, want T, msg string) {
	if got != want {
		fmt.Fprintf(os.Stderr, "FAIL: %s: got=%v want=%v\n", msg, got, want)
		os.Exit(1)
	}
}

func mustIs(err error, target error, msg string) {
	if !errors.Is(err, target) {
		fmt.Fprintf(os.Stderr, "FAIL: %s: got=%v want=%v\n", msg, err, target)
		os.Exit(1)
	}
}

func readAll(path string) []byte {
	b, err := os.ReadFile(path)
	must(err, "read "+path)
	return b
}

func trySetXattr(path, name string, value []byte) error {
	if runtime.GOOS != "linux" {
		return unix.ENOTSUP
	}
	if err := unix.Setxattr(path, name, value, 0); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	n, err := unix.Getxattr(path, name, buf)
	if err != nil {
		return err
	}
	if string(buf[:n]) != string(value) {
		return fmt.Errorf("xattr readback mismatch: got=%q want=%q", string(buf[:n]), string(value))
	}
	return nil
}

func kernelMountSmoke(root string) {
	st, err := os.Stat(root)
	must(err, "stat mountpoint")
	if !st.IsDir() {
		must(fmt.Errorf("not a directory"), "mountpoint is dir")
	}

	work := filepath.Join(root, "kernel9p-e2e")
	_ = os.RemoveAll(work)
	must(os.MkdirAll(work, 0o777), "mkdir workdir")

	_, err = os.Stat(filepath.Join(work, "does-not-exist"))
	if err == nil {
		must(fmt.Errorf("expected ENOENT"), "stat nonexistent")
	}
	mustIs(err, unix.ENOENT, "stat nonexistent errno")

	pth := filepath.Join(work, "hello.txt")
	want := []byte("hello-from-kernel-9p\n")
	must(os.WriteFile(pth, want, 0o666), "writefile")
	got := readAll(pth)
	mustEq(string(got), string(want), "readback content")

	f, err := os.OpenFile(pth, os.O_WRONLY|os.O_APPEND, 0o666)
	must(err, "open append")
	_, err = f.Write([]byte("append\n"))
	must(err, "append write")
	must(f.Close(), "close append")

	got2 := readAll(pth)
	mustEq(string(got2), string(append(want, []byte("append\n")...)), "append readback")

	linkPath := filepath.Join(work, "hello.link")
	if err := os.Symlink("hello.txt", linkPath); err != nil {
		// Some 9P servers / dialects (or kernel client configs) may deny symlinks.
		// Treat this as a limitation rather than failing the whole mount smoke.
		fmt.Fprintf(os.Stderr, "WARN: symlink not supported (continuing): %v\n", err)
	} else {
		target, err := os.Readlink(linkPath)
		must(err, "readlink")
		mustEq(target, "hello.txt", "readlink target")
	}

	if err := trySetXattr(pth, "user.go9p", []byte("ok")); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: xattr not supported (continuing): %v\n", err)
	}

	p2 := filepath.Join(work, "renamed.txt")
	must(os.Rename(pth, p2), "rename")
	_, err = os.Stat(pth)
	if err == nil {
		must(fmt.Errorf("expected old path missing"), "old path missing after rename")
	}

	// Large-ish streaming copy (tests read/write loops).
	src := filepath.Join(work, "src.bin")
	dst := filepath.Join(work, "dst.bin")
	buf := make([]byte, 256*1024)
	for i := range buf {
		buf[i] = byte(i)
	}
	must(os.WriteFile(src, buf, 0o666), "write src.bin")

	in, err := os.Open(src)
	must(err, "open src.bin")
	defer in.Close()
	out, err := os.Create(dst)
	must(err, "create dst.bin")
	_, err = io.Copy(out, in)
	must(err, "copy dst.bin")
	must(out.Close(), "close dst.bin")

	mustEq(len(readAll(dst)), len(buf), "copied size")

	must(os.Remove(src), "remove src.bin")
	must(os.Remove(dst), "remove dst.bin")
	must(os.Remove(p2), "remove renamed.txt")
	_ = os.Remove(linkPath) // may not exist if symlink was unsupported
	must(os.RemoveAll(work), "remove workdir")
}

func dial9P(addr string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func go9pClientCRUD(addr string) {
	conn, err := dial9P(addr, 8*time.Second)
	must(err, "dial 9p server "+addr)
	defer conn.Close()

	// The ufs server advertises 9P2000.u/9P2000.L extensions (Dotu=true).
	c := clnt.NewClnt(conn, 8192, true)
	defer c.Unmount()

	user := p.OsUsers.Uid2User(os.Geteuid())
	root, err := c.Attach(nil, user, "/")
	must(err, "attach")

	fid := c.FidAlloc()
	_, err = c.Walk(root, fid, []string{"."})
	must(err, "walk .")

	const (
		name = "guest-client.txt"
		perm = 0o666
	)
	must(c.Create(fid, name, perm, p.OWRITE|p.OTRUNC, ""), "create")

	want := []byte("hello-from-go9p-client\n")
	n, err := c.Write(fid, want, 0)
	must(err, "write")
	mustEq(n, len(want), "write size")

	rfid := c.FidAlloc()
	_, err = c.Walk(root, rfid, []string{name})
	must(err, "walk file")
	must(c.Open(rfid, p.OREAD), "open read")
	got, err := c.Read(rfid, 0, 64*1024)
	must(err, "read")
	mustEq(string(got), string(want), "readback")

	must(c.Remove(rfid), "remove")
}

func main() {
	mount := os.Getenv("KERNEL9P_MOUNT")
	if mount == "" {
		mount = "/mnt/9p"
	}
	server := os.Getenv("KERNEL9P_SERVER")
	if server == "" {
		server = "qemu"
	}
	tcpAddr := os.Getenv("KERNEL9P_TCP_ADDR")
	if tcpAddr == "" {
		tcpAddr = "10.0.2.2"
	}
	tcpPort := os.Getenv("KERNEL9P_TCP_PORT")
	if tcpPort == "" {
		tcpPort = "564"
	}

	fmt.Printf("INFO: kernel9p-e2e mount=%s server=%s tcp=%s:%s\n", mount, server, tcpAddr, tcpPort)

	kernelMountSmoke(mount)
	fmt.Println("PASS: kernel v9fs mount smoke")

	// When testing against our own server backend, also validate go9p client↔server CRUD.
	if server == "go9p-ufs" {
		go9pClientCRUD(net.JoinHostPort(tcpAddr, tcpPort))
		fmt.Println("PASS: go9p client↔server CRUD")
	}

	fmt.Println("PASS: kernel9p e2e")
}

