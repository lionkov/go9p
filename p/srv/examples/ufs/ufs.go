// Copyright 2009 The go9p Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"

	"github.com/lionkov/go9p/p/srv/ufs"
)

var (
	debug = flag.Int("d", 0, "print debug messages")
	addr = flag.String("addr", ":5640", "network address")
	user = flag.String("user", "", "user name")
)

func main() {
	flag.Parse()
	ufs := ufs.New()
	ufs.Dotu = true
	ufs.Id = "ufs"
	ufs.Debuglevel = *debug
	ufs.Start(ufs)
	if *user != "" {
		u := ufs.Upool.Uname2User(*user)
		if u == nil {
			log.Printf("Warning: Adding %v failed", *user)
		}
	}

	err := ufs.StartNetListener("tcp", *addr)
	if err != nil {
		log.Println(err)
	}
}
