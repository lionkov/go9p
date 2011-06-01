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
		log.Println("invalid arguments")
		return
	}

	file, oserr = c.FOpen(flag.Arg(0), p.OWRITE|p.OTRUNC)
	if oserr != nil {
		file, oserr = c.FCreate(flag.Arg(0), 0666, p.OWRITE)
		if oserr != nil {
			goto oerror
		}
	}

	buf := make([]byte, 8192)
	for {
		n, oserr = os.Stdin.Read(buf)
		if oserr != nil && oserr != os.EOF {
			goto oerror
		}

		if n == 0 {
			break
		}

		m, oserr = file.Write(buf[0:n])
		if oserr != nil {
			goto oerror
		}

		if m != n {
			err = &p.Error{"short write", 0}
			goto error
		}
	}

	file.Close()
	return

error:
	log.Println(fmt.Sprintf("Error: %s %d", err.Error, err.Errornum))
	return
oerror:
	log.Println("Error", oserr)
}
