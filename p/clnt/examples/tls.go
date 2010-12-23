// Connects to a server over TLS and lists the specified directory
package main

import (
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"go9p.googlecode.com/hg/p"
	"go9p.googlecode.com/hg/p/clnt"
)

var debuglevel = flag.Int("d", 0, "debuglevel")
var addr = flag.String("addr", "127.0.0.1:5640", "network address")

func main() {
	var user p.User
	var file *clnt.File
	var d []*p.Dir

	flag.Parse()
	user = p.OsUsers.Uid2User(os.Geteuid())
	clnt.DefaultDebuglevel = *debuglevel
	
	c, oerr := tls.Dial("tcp", "", *addr, &tls.Config { Rand: rand.Reader, Time: time.Nanoseconds,})
	if oerr!=nil {
		log.Println("can't dial", oerr)
		return
	}

	clnt, err := clnt.MountConn(c, "", user)
	if err != nil {
		goto error
	}

	if flag.NArg() != 1 {
		log.Println("invalid arguments")
		return
	}

	file, err = clnt.FOpen(flag.Arg(0), p.OREAD)
	if err != nil {
		goto error
	}

	for {
		d, err = file.Readdir(0)
		if err != nil {
			goto error
		}

		if d == nil || len(d) == 0 {
			break
		}

		for i := 0; i < len(d); i++ {
			os.Stdout.WriteString(d[i].Name + "\n")
		}
	}

	file.Close()
	return

error:
	log.Println(fmt.Sprintf("Error: %s %d", err.Error, err.Errornum))
}
