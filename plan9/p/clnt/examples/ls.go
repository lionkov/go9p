package main

import "flag"
import "fmt"
import "log"
import "os"
import "plan9/p"
import "plan9/p/clnt"

var addr = flag.String("addr", "127.0.0.1:5640", "network address")

func main() {
	var user p.User;
	var err *p.Error;
	var c *clnt.Clnt;
	var file *clnt.File;
	var st []*p.Stat;

	flag.Parse();
	user = p.OsUsers.Uid2User(os.Geteuid());
	c, err = clnt.Mount("tcp", *addr, "", user);
	if err != nil {
		goto error
	}

	if flag.NArg() != 1 {
		log.Stderr("invalid arguments");
		return;
	}

	file, err = c.FOpen(flag.Arg(0), p.OREAD);
	if err != nil {
		goto error
	}

	for {
		st, err = file.Readdir(0);
		if err != nil {
			goto error
		}

		if st == nil || len(st) == 0 {
			break
		}

		for i := 0; i < len(st); i++ {
			os.Stdout.WriteString(st[i].Name + "\n")
		}
	}

	file.Close();
	return;

error:
	log.Stderr(fmt.Sprintf("Error: %s %d", err.Error, err.Nerror));
}
