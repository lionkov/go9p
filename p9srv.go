package p9srv

import (
	"./p9";
	"net";
	"log";
	"os";
	"sync";
)

type Auth interface {
	Init(afid *Fid, aname string, aqid *p9.Qid) *p9.Error;
	Destroy(afid *Fid);
	Check(fid *Fid, afid *Fid, aname string) *p9.Error;
	Read(afid *Fid, offset uint64, data []byte) (count int, err *p9.Error);
	Write(afid *Fid, offset uint64, data []byte) (count int, err *p9.Error);
}

type Users interface {
	Uid2User(uid int) *User;
	Uname2User(uname string) *User;
	Gid2Group(gid int) *Group;
	Gname2Group(gname string) *Group;

type User interface {
	Name() string;
	Id() int;
	Groups() []*Group;
}

type Group interface {
	Name() string;
	Id() int;
	Members() []*User;
}

type SrvImpl interface {
	Start(*Srv, SrvImpl);
	ConnOpened(*Conn);
	ConnClosed(*Conn);
	FidDestroy(*Fid);
	ReqProcess(*Req);
	ReqDestroy(*Req);

	Attach(*Req);
	Flush(*Req) bool;
	Walk(*Req);
	Open(*Req);
	Create(*Req);
	Read(*Req);
	Write(*Req);
	Clunk(*Req);
	Remove(*Req);
	Stat(*Req);
	Wstat(*Req);
}

type Srv struct {
	msize		uint32;
	dotu		bool;
	debuglevel	int;
	upool		Users;
	auth		Auth;
	maxpend		int;	/* reqin and reqout channel size */
	ngoroutines	int;	/* 0 -- create a goroutine for each request */
	fd			net.Conn;
	impl		SrvImpl;

	reqin		chan *Req;
	reqout		chan *Req;
};

type Conn struct {
	srv			*Srv;
	msize		uint32;
	dotu		bool;

	lock		sync.Mutex;
	refcount	int;
	conn		net.Conn;
	fidpool		map[uint32] *Fid;
	reqfirst	*Req;
	reqlast		*Req;
};

type Fid struct {
	fid		uint32;
	lock		sync.Mutex;
	refcount	int;
	conn		*Conn;
	omode		uint16;
	ftype		uint8;
	diroffset	uint32;
	user		*User;
}

type Req struct {
	tc		*p9.Call;
	rc		*p9.Call;
	fid		*Fid;
	afid		*Fid;		/* Tauth, Tattach */
	newfid		*Fid;		/* Twalk */

	conn		*Conn;
	flushreq	*Req;
	prev, next	*Req;
}

func (srv *Srv) Start(impl SrvImpl, fd net.Conn) bool
{
	srv.impl = impl;
	srv.fd = fd

	if srv.msize<p9.IOHdrSz {
		srv.msize = p9.MSz
	}

	srv.reqin = make(chan *Req, srv.maxpend);
	srv.reqout = make(chan *Req, srv.maxpend);
	done = make(chan bool, 0);

	// XXX: handle ngoroutines==0?
	//for i:=0; i<srv.ngoroutines; i++ {
	//		go srv.work()
	//}

	// let's leave it simple for now: one receiver, one sender
	go srv.recv()
	go srv.send()

	for {
		select {
		case req := srv.reqin:
			// add req to list (for flush)
			// send to SrvImpl for handling, if missing use defaults
			// send to workers for dispatch; XXX: use separate Writer workers?
			srv.impl.ReqProcess(req)
			// now, what happens if impl can't process the request? we must handle
			// it gracefully for the most common types or return error... 
			// what's the best way to do this?

		case <-done:
			// Flush outstanding requests; (tell all workers to leave?); exit

			// lastly, close connection
			srv.fd.Close();
			return;
		}
	}

	return true;
}

func (srv *Srv) recv()
{
	var buf [MSz]uint8;	// start with a default buffer size but will adjust after negotiation with the client if necessary
	for {
		n, err := fd.Read(buf[0:4]);
		if err != nil {
			log.Stderr("got error, exiting", fd.RemoteAddr(), err.Error);
			srv.done<-true;
			return;
		}
		if n == 0 {
			log.Stderr("eof from client: ", fd.RemoteAddr())
			srv.done<-true;
			return;
		}
		p := buf
		sz, p = gint32(p)
		n, err := fd.Read(buf[4:sz]);
		if err != nil {
			log.Stderr("error reading rest of packet: ", fd.RemoteAddr(), err.Error);
			srv.done<-true;
		}
		if n < sz - 4 {
			log.Stderr("short read from net, closing: ", fd.RemoteAddr());
			srv.done<-true;
		}
		call, err, rest := p9.Unpack(buf, srv.dotu)
		if rest {
			log.Stderr("procotol botch", fd.RemoteAddr());
			//XXX: send error to client before closing on them
		}
		req := new(Req);
		req.fid = new(Fid);
		req.tc = call;
		srv.reqin <- req;
	}
}

func (srv *Srv) send()
{
	for {
		call := <-srv.reqout
		switch call.rc.id {
		// I don't see a buffer to send to the client here :(
		// Perhaps a common Pack() routine would be better?
		case p9.Rversion:
			err := p9.PackRversion(req.rc, srv.msize, srv.version);
		case p9.Rauth:
			err := p9.PackRauth(req.rc, srv.auth.aqid);
		case p9.Rerror:
			err := p9.PackRerror(req.rc, /*XXX*/error, nerror, dotu);
		case p9.Rattach:
			p9.PackRattach();
		case p9.Rwalk:
			p9.PackRwalk();
		case p9.Ropen:
			p9.PackRopen();
		case p9.Rcreate:
			p9.PackRcreate();
		case p9.Rread:
			p9.PackRread();
		case p9.Rwrite:
			p9.PackRwrite();
		case p9.Rclunk:
			p9.PackRclunk();
		case p9.Rremove:
			p9.PackRremove();
		case p9.Rstat:
			p9.PackRstat();
		case p9.Rwstat:
			p9.PackRwstat();
		}
	}
}

func ListenAndServe(network, listen string, impl SrvImpl, debug bool) os.Error {
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
		// how do we handle debugging outside of the Srv?
		if debug {
			log.Stderr("accepted connection from", fd.RemoteAddr());
		}
		// do we keep track of servers we create so we can kill then upon exit?
		srv := new(Srv);
		go srv.start(impl, fd);
	}
	return nil;
}
