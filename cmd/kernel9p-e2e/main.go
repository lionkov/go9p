package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
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

func dirEntNames(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
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

	// Directory CRUD + readdir.
	d1 := filepath.Join(work, "d1")
	d2 := filepath.Join(work, "d2")
	must(os.Mkdir(d1, 0o777), "mkdir d1")
	names, err := dirEntNames(work)
	must(err, "readdir workdir")
	foundD1 := false
	for _, n := range names {
		if n == "d1" {
			foundD1 = true
			break
		}
	}
	mustEq(foundD1, true, "readdir sees d1")

	nested := filepath.Join(d1, "nested.txt")
	must(os.WriteFile(nested, []byte("nested\n"), 0o666), "write nested file")
	must(os.Rename(d1, d2), "rename dir d1->d2")
	_, err = os.Stat(d1)
	if err == nil {
		must(fmt.Errorf("expected old dir missing"), "old dir missing after rename")
	}
	must(os.Remove(filepath.Join(d2, "nested.txt")), "remove nested file")
	must(os.Remove(d2), "remove renamed dir")

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

	// Basic metadata: chmod + chtimes. Some servers/clients may ignore parts; treat
	// mismatches as failures for the kernel-mounted path.
	must(os.Chmod(pth, 0o640), "chmod 0640")
	fi, err := os.Stat(pth)
	must(err, "stat after chmod")
	mustEq(fi.Mode().Perm(), os.FileMode(0o640), "chmod reflected")

	t0 := time.Unix(1700000000, 0) // stable, but not special
	must(os.Chtimes(pth, t0, t0), "chtimes")
	fi, err = os.Stat(pth)
	must(err, "stat after chtimes")
	mt := fi.ModTime()
	if mt.Before(t0.Add(-2*time.Second)) || mt.After(t0.Add(2*time.Second)) {
		must(fmt.Errorf("modtime out of range: got=%v want~=%v", mt, t0), "chtimes reflected (2s tolerance)")
	}

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

func kernelRamfsSmoke(root string) {
	st, err := os.Stat(root)
	must(err, "stat mountpoint")
	if !st.IsDir() {
		must(fmt.Errorf("not a directory"), "mountpoint is dir")
	}

	// Kernel client interop for synthetic servers can be limited; ensure at least
	// mount + readdir works (write tests are covered by userspace unit tests).
	_, err = dirEntNames(root)
	must(err, "readdir mountpoint")
}

func kernelTimeFSSmoke(root string) {
	readSomeTrim := func(name string, max int) string {
		f, err := os.Open(filepath.Join(root, name))
		must(err, "open "+name)
		defer f.Close()
		buf := make([]byte, max)
		n, err := f.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			must(err, "read "+name)
		}
		return strings.TrimSpace(string(buf[:n]))
	}

	a := readSomeTrim("time", 256)
	mustEq(a == "", false, "time non-empty")
	time.Sleep(10 * time.Millisecond)
	b := readSomeTrim("time", 256)
	mustEq(b == "", false, "time non-empty (second)")

	// /inftime is intentionally infinite; just ensure it produces data.
	inf := readSomeTrim("inftime", 256)
	mustEq(inf == "", false, "inftime non-empty")
}

func kernelCloneFSSmoke(root string) {
	clone := strings.TrimSpace(string(readAll(filepath.Join(root, "clone"))))
	if clone == "" {
		must(fmt.Errorf("empty clone result"), "clone returns name")
	}
	if _, err := strconv.Atoi(clone); err != nil {
		must(err, "clone name is integer")
	}

	pth := filepath.Join(root, clone)
	got := strings.TrimSpace(string(readAll(pth)))
	if got == "" {
		must(fmt.Errorf("empty clone file read"), "clone file readable")
	}
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
	fs := os.Getenv("KERNEL9P_FS")
	if fs == "" {
		fs = "ufs"
	}
	tcpAddr := os.Getenv("KERNEL9P_TCP_ADDR")
	if tcpAddr == "" {
		tcpAddr = "10.0.2.2"
	}
	tcpPort := os.Getenv("KERNEL9P_TCP_PORT")
	if tcpPort == "" {
		tcpPort = "564"
	}

	fmt.Printf("INFO: kernel9p-e2e fs=%s mount=%s server=%s tcp=%s:%s\n", fs, mount, server, tcpAddr, tcpPort)

	switch fs {
	case "ufs", "ramfs":
		if fs == "ufs" {
			kernelMountSmoke(mount)
			fmt.Println("PASS: kernel v9fs mount smoke")
		} else {
			kernelRamfsSmoke(mount)
			fmt.Println("PASS: kernel ramfs mount smoke")
		}
	case "timefs":
		kernelTimeFSSmoke(mount)
		fmt.Println("PASS: kernel timefs mount smoke")
	case "clonefs":
		kernelCloneFSSmoke(mount)
		fmt.Println("PASS: kernel clonefs mount smoke")
	default:
		must(fmt.Errorf("unknown KERNEL9P_FS=%q", fs), "select fs mode")
	}

	// When testing against our own server backend, also validate go9p client↔server CRUD.
	if server == "go9p-ufs" {
		go9pClientCRUD(net.JoinHostPort(tcpAddr, tcpPort))
		fmt.Println("PASS: go9p client↔server CRUD")
	}

	fmt.Println("PASS: kernel9p e2e")
}

