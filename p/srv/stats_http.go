package srv

import (
	"fmt"
	"io"
	"http"
	"go9p.googlecode.com/hg/p"
)

func (srv *Srv) statsRegister() {
	http.Handle("/go9p/srv/"+srv.Id, srv)
}

func (srv *Srv) ServeHTTP(c *http.Conn, r *http.Request) {
	io.WriteString(c, fmt.Sprintf("<html><body><h1>Server %s</h1>", srv.Id))
	defer io.WriteString(c, "</body></html>")

	// connections
	io.WriteString(c, "<h2>Connections</h2><p>")
	srv.Lock()
	if srv.connlist == nil {
		io.WriteString(c, "none")
	}

	for conn := srv.connlist; conn != nil; conn = conn.next {
		io.WriteString(c, fmt.Sprintf("<a href='/go9p/srv/%s/conn/%s'>%s</a><br>", srv.Id, conn.Id, conn.Id))
	}
	srv.Unlock()
}

func (conn *Conn) statsRegister() {
	http.Handle("/go9p/srv/"+conn.Srv.Id+"/conn/"+conn.Id, conn)
}

func (conn *Conn) statsUnregister() {
	http.Handle("/go9p/srv/"+conn.Srv.Id+"/conn/"+conn.Id, nil)
}

func (conn *Conn) ServeHTTP(c *http.Conn, r *http.Request) {
	io.WriteString(c, fmt.Sprintf("<html><body><h1>Connection %s/%s</h1>", conn.Srv.Id, conn.Id))
	defer io.WriteString(c, "</body></html>")

	// statistics
	conn.Lock()
	io.WriteString(c, fmt.Sprintf("<p>Number of processed requests: %d", conn.nreqs))
	io.WriteString(c, fmt.Sprintf("<br>Sent %v bytes", conn.rsz))
	io.WriteString(c, fmt.Sprintf("<br>Received %v bytes", conn.tsz))
	io.WriteString(c, fmt.Sprintf("<br>Pending requests: %d max %d", conn.npend, conn.maxpend))
	conn.Unlock()

	// fcalls
	if conn.Debuglevel&DbgLogFcalls != 0 {
		fs := conn.Srv.Log.Filter(conn, DbgLogFcalls)
		io.WriteString(c, fmt.Sprintf("<h2>Last %d 9P messages</h2>", len(fs)))
		for _, l := range fs {
			fc := l.Data.(*p.Fcall)
			if fc.Type != 0 {
				io.WriteString(c, fmt.Sprintf("<br>%s", fc))
			}
		}
	}
}
