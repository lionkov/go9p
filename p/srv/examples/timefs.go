// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"go9p.googlecode.com/hg/p"
	"go9p.googlecode.com/hg/p/srv"
)

type Time struct {
	srv.File
}
type InfTime struct {
	srv.File
}

var addr = flag.String("addr", ":5640", "network address")
var debug = flag.Bool("d", false, "print debug messages")
var debugall = flag.Bool("D", false, "print packets as well as debug messages")

func (*InfTime) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	// push out time ignoring offset (infinite read)
	t := time.LocalTime().String() + "\n"
	b := []byte(t)
	ml := len(b)
	if ml > len(buf) {
		ml = len(buf)
	}

	copy(buf, b[0:ml])
	return ml, nil
}

func (*Time) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	t := time.LocalTime().String()
	b := []byte(t)
	n := len(b)
	if offset >= uint64(n) {
		return 0, nil
	}

	b = b[int(offset):n]
	n -= int(offset)
	if len(buf) < n {
		n = len(buf)
	}

	copy(buf[offset:int(offset)+n], b[offset:])
	return n, nil
}

func main() {
	var err error
	var tm *Time
	var ntm *InfTime
	var s *srv.Fsrv

	flag.Parse()
	user := p.OsUsers.Uid2User(os.Geteuid())
	root := new(srv.File)
	err = root.Add(nil, "/", user, nil, p.DMDIR|0555, nil)
	if err != nil {
		goto error
	}

	tm = new(Time)
	err = tm.Add(root, "time", p.OsUsers.Uid2User(os.Geteuid()), nil, 0444, tm)
	if err != nil {
		goto error
	}
	ntm = new(InfTime)
	err = ntm.Add(root, "inftime", p.OsUsers.Uid2User(os.Geteuid()), nil, 0444, ntm)
	if err != nil {
		goto error
	}

	s = srv.NewFileSrv(root)
	s.Dotu = true

	if *debug {
		s.Debuglevel = 1
	}
	if *debugall {
		s.Debuglevel = 2
	}

	s.Start(s)
	err = s.StartNetListener("tcp", *addr)
	if err != nil {
		goto error
	}

	return

error:
	log.Println(fmt.Sprintf("Error: %s", err))
}
