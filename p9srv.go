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
}

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
	impl		SrvImpl;

	reqin		chan *Req;
	reqout		chan *Req;
};

type Conn struct {
	srv		*Srv;
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

func (srv *Srv) Start(impl SrvImpl) bool
{
	srv.impl = impl;

	if srv.msize<p9.IOHdrSz {
		srv.msize = 8192+p9.IOHdrSz;
	}

	srv.reqin = make(chan *Req, srv.maxpend);
	srv.reqout = make(chan *Req, srv.maxpend);
	for i:=0; i<srv.ngoroutines; i++ {
		go srv.work();
	}

	return true;
}

func (srv *Srv) work()
{
	// TODO
}

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
