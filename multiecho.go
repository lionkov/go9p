// echo server handling simultaneous requests
package main

import (
	"./p9";
	"net";
	"log";
	"os";
)

func runEcho(fd net.Conn) {
	var buf [p9.IOHdrSz]byte;

	for {
		n, err := fd.Read(&buf);
		if err != nil || n == 0 {
			log.Stderr("closing...");
			fd.Close();
			return
		}
		fd.Write(buf[0:n]);
		log.Stderr("got from net: ", fd.RemoteAddr(), buf[0:n])
	}
}

func ListenAndServe(network, listen string) os.Error {
	l, err := net.Listen(network, listen);
	if err != nil {
		log.Stderr("listen fail: ", network, listen, err);
		return err;
	}

	for {
		fd, err := l.Accept();
		if err != nil {
			break
		}
		go runEcho(fd);
	}

	return nil;
}

func main() {
	ListenAndServe("tcp", "0.0.0.0:10000");
}
