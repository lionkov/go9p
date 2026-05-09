package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

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

func readAll(path string) []byte {
	b, err := os.ReadFile(path)
	must(err, "read "+path)
	return b
}

func mustIs(err error, target error, msg string) {
	if !errors.Is(err, target) {
		fmt.Fprintf(os.Stderr, "FAIL: %s: got=%v want=%v\n", msg, err, target)
		os.Exit(1)
	}
}

func trySetXattr(path, name string, value []byte) error {
	// Use syscall where available without adding external dependencies.
	// On Linux, xattr syscalls exist; on other platforms this test harness isn't used.
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

func main() {
	// The initramfs mounts the host-exported 9p tag at /mnt/9p.
	root := os.Getenv("KERNEL9P_MOUNT")
	if root == "" {
		root = "/mnt/9p"
	}

	st, err := os.Stat(root)
	must(err, "stat mountpoint")
	if !st.IsDir() {
		must(fmt.Errorf("not a directory"), "mountpoint is dir")
	}

	work := filepath.Join(root, "kernel9p-smoke")
	_ = os.RemoveAll(work)
	must(os.MkdirAll(work, 0o777), "mkdir workdir")

	// Non-existent path should yield ENOENT.
	_, err = os.Stat(filepath.Join(work, "does-not-exist"))
	if err == nil {
		must(fmt.Errorf("expected ENOENT"), "stat nonexistent")
	}
	mustIs(err, unix.ENOENT, "stat nonexistent errno")

	// Basic create/write/read.
	p := filepath.Join(work, "hello.txt")
	want := []byte("hello-from-kernel-9p\n")
	must(os.WriteFile(p, want, 0o666), "writefile")
	got := readAll(p)
	mustEq(string(got), string(want), "readback content")

	// Append semantics (open + write).
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0o666)
	must(err, "open append")
	_, err = f.Write([]byte("append\n"))
	must(err, "append write")
	must(f.Close(), "close append")

	got2 := readAll(p)
	mustEq(string(got2), string(append(want, []byte("append\n")...)), "append readback")

	// Symlink + readlink.
	linkPath := filepath.Join(work, "hello.link")
	must(os.Symlink("hello.txt", linkPath), "symlink")
	target, err := os.Readlink(linkPath)
	must(err, "readlink")
	mustEq(target, "hello.txt", "readlink target")

	// Xattrs (best-effort; may not be supported depending on server/kernel opts).
	if err := trySetXattr(p, "user.go9p", []byte("ok")); err != nil {
		// Many 9p setups don't support xattrs; treat as skip-but-log.
		fmt.Fprintf(os.Stderr, "WARN: xattr not supported (continuing): %v\n", err)
	}

	// Permissions (best-effort): chmod 000 and attempt open for read should fail.
	// Depending on mount/security model, permission enforcement may vary.
	must(os.Chmod(p, 0o000), "chmod 000")
	_, err = os.Open(p)
	if err == nil {
		fmt.Fprintf(os.Stderr, "WARN: open succeeded on chmod 000 (permission enforcement varies on 9p)\n")
	} else if !errors.Is(err, unix.EACCES) && !errors.Is(err, unix.EPERM) {
		fmt.Fprintf(os.Stderr, "WARN: open failed with unexpected error (continuing): %v\n", err)
	}
	must(os.Chmod(p, 0o666), "chmod restore")

	// Rename.
	p2 := filepath.Join(work, "renamed.txt")
	must(os.Rename(p, p2), "rename")
	_, err = os.Stat(p)
	if err == nil {
		must(fmt.Errorf("expected old path missing"), "old path missing after rename")
	}

	// Readdir.
	ents, err := os.ReadDir(work)
	must(err, "readdir")
	found := false
	for _, e := range ents {
		if e.Name() == "renamed.txt" {
			found = true
		}
	}
	if !found {
		must(fmt.Errorf("renamed.txt not found in readdir"), "readdir contains renamed.txt")
	}

	// Truncate via open.
	f2, err := os.OpenFile(p2, os.O_WRONLY|os.O_TRUNC, 0o666)
	must(err, "open trunc")
	_, err = f2.Write([]byte("x"))
	must(err, "write trunc")
	must(f2.Close(), "close trunc")
	got3 := readAll(p2)
	mustEq(string(got3), "x", "truncate result")

	// Rename across directories.
	dirA := filepath.Join(work, "a")
	dirB := filepath.Join(work, "b")
	must(os.MkdirAll(dirA, 0o777), "mkdir a")
	must(os.MkdirAll(dirB, 0o777), "mkdir b")
	x := filepath.Join(dirA, "x.txt")
	must(os.WriteFile(x, []byte("x"), 0o666), "write a/x.txt")
	y := filepath.Join(dirB, "y.txt")
	must(os.Rename(x, y), "rename a/x.txt -> b/y.txt")
	mustEq(string(readAll(y)), "x", "rename across dirs content")

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

	// Cleanup (remove + rmdir).
	must(os.Remove(src), "remove src.bin")
	must(os.Remove(dst), "remove dst.bin")
	must(os.Remove(p2), "remove renamed.txt")
	must(os.Remove(linkPath), "remove symlink")
	must(os.Remove(y), "remove b/y.txt")
	must(os.RemoveAll(dirA), "remove dir a")
	must(os.RemoveAll(dirB), "remove dir b")
	must(os.RemoveAll(work), "remove workdir")

	fmt.Println("PASS: kernel 9p client smoke test")
}

