package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"go9p.googlecode.com/hg/p"
	"go9p.googlecode.com/hg/p/clnt"
)

var debuglevel = flag.Int("d", 0, "debuglevel")
var addr = flag.String("addr", "127.0.0.1:5640", "network address")

func main() {
	var n, m int
	var user p.User
	var err *p.Error
	var oserr os.Error
	var c *clnt.Clnt
	var file *clnt.File

	flag.Parse()
	user = p.OsUsers.Uid2User(os.Geteuid())
	clnt.DefaultDebuglevel = *debuglevel
	c, err = clnt.Mount("tcp", *addr, "", user)
	if err != nil {
		goto error
	}

	if flag.NArg() != 1 {
		log.Stderr("invalid arguments")
		return
	}

	file, err = c.FOpen(flag.Arg(0), p.OWRITE|p.OTRUNC)
	if err != nil {
		file, err = c.FCreate(flag.Arg(0), 0666, p.OWRITE)
		if err != nil {
			goto error
		}
	}

	buf := make([]byte, 8192)
	for {
		n, oserr = os.Stdin.Read(buf)
		if oserr != nil && oserr != os.EOF {
			err = &p.Error{oserr.String(), 0}
			goto error
		}

		if n == 0 {
			break
		}

		m, err = file.Write(buf[0:n])
		if err != nil {
			goto error
		}

		if m != n {
			err = &p.Error{"short write", 0}
			goto error
		}
	}

	file.Close()
	return

error:
	log.Stderr(fmt.Sprintf("Error: %s %d", err.Error, err.Errornum))
}
