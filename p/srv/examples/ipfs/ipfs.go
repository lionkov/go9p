// Copyright 2009 The Go9p Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/srv"
)

type fs struct {
	srv   *srv.Fsrv
	user  p.User
	group p.Group
}

type rootOps struct {
	srv.File
}

// Find implements srv.FFindOp. If the child node is not found, we will try to
// resolve the given host and create a synthetic file for it, containing its IP
// addresses. If it is found and is older than one minute, we unlink the child
// from the directory and resolve it again. (This does not respect the caching
// times for DNS, it's just an example.)
func (d *rootOps) Find(host string) (*srv.File, error) {
	now := time.Now()
	f := d.File.Find(host)
	if f != nil {
		if now.Sub(f.Ops.(*fileOps).whenResolved) < time.Minute {
			return f, nil
		}
		f.Remove()
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, err
	}
	ops := new(fileOps)
	if err := ops.File.Add(&d.File, host, ipfs.user, ipfs.group, 0444, ops); err != nil {
		return nil, fmt.Errorf("could not add %q: %v", host, err)
	}
	ops.whenResolved = now
	ops.data = []byte(strings.Join(addrs, "\n") + "\n")
	ops.Length = uint64(len(ops.data))
	return &ops.File, nil
}

type fileOps struct {
	srv.File
	whenResolved time.Time
	data         []byte
}

func (f *fileOps) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	f.Lock()
	defer f.Unlock()
	if offset > f.Length {
		return 0, nil
	}
	return copy(buf, f.data[offset:]), nil
}

var ipfs fs

func main() {
	addr := flag.String("addr", ":5640", "network address")
	debug := flag.Int("d", 0, "debuglevel")
	flag.Parse()

	var err error

	ipfs.user = p.OsUsers.Uid2User(os.Geteuid())
	ipfs.group = p.OsUsers.Gid2Group(os.Getegid())

	root := new(rootOps)
	err = root.Add(nil, "/", ipfs.user, ipfs.group, p.DMDIR|0555, root)
	if err != nil {
		goto error
	}

	ipfs.srv = srv.NewFileSrv(&root.File)
	ipfs.srv.Dotu = true
	ipfs.srv.Debuglevel = *debug
	ipfs.srv.Start(ipfs.srv)
	ipfs.srv.Id = "ipfs"

	err = ipfs.srv.StartNetListener("tcp", *addr)
	if err != nil {
		goto error
	}
	return

error:
	log.Println(fmt.Sprintf("Error: %s", err))
}
