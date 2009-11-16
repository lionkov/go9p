package p9srv

import (
	"./p9";
	"bytes";
	"log";
	"net";
	"os";
	"sync";
	"syscall";
)

type reqStatus int;
const (
	reqFlush reqStatus = (1<<iota);/* request is flushed (no response will be sent) */
	reqWork;			/* goroutine is currently working on it */
	reqResponded;			/* response is already produced */
	reqSaved;			/* no response was produced after the request is worked on */
)

var Eunknownfid	*p9.Error = &p9.Error{"unknown fid", syscall.EINVAL};
var Enoauth *p9.Error = &p9.Error{"no authentication required", syscall.EINVAL};
var Einuse *p9.Error = &p9.Error{"fid already in use", syscall.EINVAL};
var Ebaduse *p9.Error = &p9.Error{"bad use of fid", syscall.EINVAL};
var Eopen *p9.Error = &p9.Error{"fid already opened", syscall.EINVAL};
var Enotdir *p9.Error = &p9.Error{"not a directory", syscall.ENOTDIR};
var Eperm *p9.Error = &p9.Error{"permission denied", syscall.EPERM};
var Etoolarge *p9.Error = &p9.Error{"i/o count too large", syscall.EINVAL};
var Ebadoffset *p9.Error = &p9.Error{"bad offset in directory read", syscall.EINVAL};
var Edirchange *p9.Error = &p9.Error{"cannot convert between files and directories", syscall.EINVAL};
var Enouser *p9.Error = &p9.Error{"unknown user", syscall.EINVAL};

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
	Flush(*Req);
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
	sync.Mutex;
	Msize		uint32;
	Dotu		bool;
	Debuglevel	int;
	Upool		*Users;
	Auth		*Auth;
	Maxpend		int;	/* reqin and reqout channel size */
	Ngoroutines	int;	/* 0 -- create a goroutine for each request */
	Impl		SrvImpl;

	Reqin		chan *Req;
};

type Conn struct {
	sync.Mutex;
	Srv		*Srv;
	Msize		uint32;
	Dotu		bool;

	conn		net.Conn;
	fidpool		map[uint32] *Fid;
	reqfirst	*Req;
	reqlast		*Req;

	reqout		chan *Req;
	done		chan bool;
};

type Fid struct {
	sync.Mutex;
	fid		uint32;
	refcount	int;
	Fconn		*Conn;
	opened		bool;
	Omode		uint8;
	Type		uint8;
	Diroffset	uint64;
	User		*User;
}

type Req struct {
	sync.Mutex;
	Tc		*p9.Fcall;
	Rc		*p9.Fcall;
	Fid		*Fid;
	Afid		*Fid;		/* Tauth, Tattach */
	Newfid		*Fid;		/* Twalk */

	status		reqStatus;
	Conn		*Conn;
	flushreq	*Req;
	prev, next	*Req;
}

func (srv *Srv) Start(impl SrvImpl) bool
{
	srv.Impl = impl;

	if srv.Msize<p9.IOHdrSz {
		srv.Msize = p9.MSize;
	}

	srv.Reqin = make(chan *Req, srv.Maxpend);
	n := srv.Ngoroutines;
	if n<= 0 {
		n = 1;
	}

	for i:=0; i<n; i++ {
			go srv.work()
	}

	return true;
}

func (srv *Srv) work()
{
	for req:=<-srv.Reqin; req!=nil; req=<-srv.Reqin {
		req.Lock();
		flushed := (req.status&reqFlush) != 0;
		if !flushed {
			req.status |= reqWork;
		}
		req.Unlock();

		if flushed {
			req.Respond();
		}

		srv.Impl.ReqProcess(req);
		req.Lock();
		req.status &= ^reqWork;
		if !(req.status&reqResponded!=0) {
			req.status |= reqSaved;
		}
		req.Unlock();
	}
}

func (req *Req) Process()
{
	conn := req.Conn;
	srv := conn.Srv;
	tc := req.Tc;

	if tc.Fid!=p9.Nofid &&tc.Id!=p9.Tattach {
		srv.Lock();
		req.Fid = conn.fidGet(tc.Fid);
		srv.Unlock();
		if req.Fid==nil {
			req.RespondError(Eunknownfid);
			return;
		}
	}

	switch req.Tc.Id {
	default:
		req.RespondError(&p9.Error{"unknown message type", syscall.ENOSYS});

	case p9.Tversion:
		srv.version(req);

	case p9.Tauth:
		srv.auth(req);

	case p9.Tattach:
		srv.attach(req);

	case p9.Tflush:
		srv.flush(req);

	case p9.Twalk:
		srv.walk(req);

	case p9.Topen:
		srv.open(req);

	case p9.Tcreate:
		srv.create(req);

	case p9.Tread:
		srv.read(req);

	case p9.Twrite:
		srv.write(req);

	case p9.Tclunk:
		srv.clunk(req);

	case p9.Tremove:
		srv.remove(req);

	case p9.Tstat:
		srv.stat(req);

	case p9.Twstat:
		srv.wstat(req);
	}
}

func (req *Req) Respond()
{
	conn := req.Conn;
	srv := conn.Srv;
	req.Lock();
	status := req.status;
	req.status |= reqResponded;
	req.status &= ^reqWork;
	req.Unlock();

	if (status&reqResponded)!=0 {
		return;
	}

	/* remove the request and all requests flushing it */
	conn.Lock();
	if req.prev!=nil {
		req.prev.next = req.next;
	} else {
		conn.reqfirst = req.next;
	}

	if req.next!=nil {
		req.next.prev = req.prev;
	} else {
		conn.reqlast = req.prev;
	}

	for freq:=req.flushreq; freq!=nil; freq=freq.flushreq {
		if freq.prev!=nil {
			freq.prev.next = freq.next;
		} else {
			conn.reqfirst = freq.next;
		}

		if freq.next!=nil {
			freq.next.prev = freq.prev;
		} else {
			conn.reqlast = freq.prev;
		}
	}
	conn.Unlock();

	/* call the post-handlers (if needed) */
	switch req.Tc.Id {
	case p9.Tauth:
		srv.authPost(req);

	case p9.Tattach:
		srv.attachPost(req);

	case p9.Twalk:
		srv.walkPost(req);

	case p9.Topen:
		srv.openPost(req);

	case p9.Tcreate:
		srv.createPost(req);

	case p9.Tread:
		srv.readPost(req);

	case p9.Tclunk:
		srv.clunkPost(req);

	case p9.Tremove:
		srv.removePost(req);
	}

	if req.Fid!=nil {
		req.Fid.DecRef();
		req.Fid = nil;
	}

	if req.Afid!=nil {
		req.Afid.DecRef();
		req.Afid = nil;
	}

	if req.Newfid!=nil {
		req.Newfid.DecRef();
		req.Newfid = nil;
	}

	if (status&reqFlush)==0 {
		conn.reqout <- req;
	}

	for freq:=req.flushreq; freq!=nil; freq=freq.flushreq {
		if (freq.status&reqFlush)==0 {
			conn.reqout <- freq;
		}
	}
}

func (req *Req) RespondError(err *p9.Error)
{
	p9.PackRerror(req.Rc, err.Error, uint32(err.Nerror), req.Conn.Dotu);
	req.Respond();
}

func (req *Req) RespondRversion(msize uint32, version string)
{
	err := p9.PackRversion(req.Rc, msize, version);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRauth(aqid *p9.Qid)
{
	err := p9.PackRauth(req.Rc, aqid);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRflush()
{
	err := p9.PackRflush(req.Rc);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRattach(aqid *p9.Qid)
{
	err := p9.PackRattach(req.Rc, aqid);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRwalk(wqids []p9.Qid)
{
	err := p9.PackRwalk(req.Rc, wqids);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRopen(qid *p9.Qid, iounit uint32)
{
	err := p9.PackRopen(req.Rc, qid, iounit);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRcreate(qid *p9.Qid, iounit uint32)
{
	err := p9.PackRcreate(req.Rc, qid, iounit);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRread(data []byte)
{
	err := p9.PackRread(req.Rc, data);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRwrite(count uint32)
{
	err := p9.PackRwrite(req.Rc, count);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRclunk()
{
	err := p9.PackRclunk(req.Rc);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRremove()
{
	err := p9.PackRremove(req.Rc);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRstat(st *p9.Stat)
{
	err := p9.PackRstat(req.Rc, st, req.Conn.Dotu);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (req *Req) RespondRwstat()
{
	err := p9.PackRwstat(req.Rc);
	if err!=nil {
		req.RespondError(err);
	} else {
		req.Respond();
	}
}

func (conn *Conn) fidGet(fidno uint32) *Fid
{
	conn.Lock();
	fid, present := conn.fidpool[fidno];
	if present {
		fid.IncRef();
	}
	conn.Unlock();

	return fid;
}

func (conn *Conn) fidNew(fidno uint32) *Fid
{
	conn.Lock();
	_, present := conn.fidpool[fidno];
	if present {
		conn.Unlock();
		return nil;
	}

	fid := new(Fid);
	fid.fid = fidno;
	fid.refcount = 1;
	fid.Fconn = conn;
	conn.fidpool[fidno] = fid;
	conn.Unlock();

	return fid;
}

func (fid *Fid) IncRef()
{
	fid.Lock();
	fid.refcount++;
	fid.Unlock();
}

func (fid *Fid) DecRef()
{
	fid.Lock();
	fid.refcount--;
	n := fid.refcount;
	fid.Unlock();

	if n>0 {
		return;
	}

	conn := fid.Fconn;
	conn.Lock();
	conn.fidpool[fid.fid] = nil, false;
	conn.Unlock();

	conn.Srv.Impl.FidDestroy(fid);
}

func (srv *Srv) version(req *Req)
{
	tc := req.Tc;
	conn := req.Conn;

	if tc.Msize<p9.IOHdrSz {
		req.RespondError(&p9.Error{"msize too small", syscall.EINVAL});
		return;
	}

	if tc.Msize<conn.Msize {
		conn.Msize = tc.Msize;
	}

	conn.Dotu = tc.Version=="9P2000.u" && srv.Dotu;
	ver := "9P2000";
	if conn.Dotu {
		ver = "9P2000.u";
	}

	/* make sure that the responses of all current requests will be ignored */
	conn.Lock();
	for r:=conn.reqfirst; r!=nil; r=r.next {
		if r!=req {
			r.Lock();
			r.status |= reqFlush;
			r.Unlock();
		}
	}
	conn.Unlock();

	req.RespondRversion(conn.Msize, ver);
}

func (srv *Srv) auth(req *Req)
{
	var aqid p9.Qid;

	tc := req.Tc;
	conn := req.Conn;
	if tc.Afid==p9.Nofid {
		req.RespondError(Eunknownfid);
		return;
	}

	req.Afid = conn.fidNew(tc.Afid);
	if req.Afid==nil {
		req.RespondError(Einuse);
		return;
	}

	var user *User = nil;
	if tc.Uname!="" {
		user = srv.Upool.Uname2User(tc.Uname);
	} else if tc.Nuname!=p9.Nouid {
		user = srv.Upool.Uid2User(int(tc.Nuname));
	}

	if user==nil {
		req.RespondError(Enouser);
		return;
	}

	req.Afid.User = user;
	req.Afid.Type = p9.QTAUTH;
	if srv.Auth!=nil {
		err := srv.Auth.Init(req.Afid, tc.Aname, &aqid);
		if err!=nil {
			req.RespondError(err);
			return;
		}
	} else {
		req.RespondError(Enoauth);
		return;
	}

	req.RespondRauth(&aqid);
}

func (srv *Srv) authPost(req *Req)
{
	if req.Rc!=nil && req.Rc.Id==p9.Rattach {
		req.Afid.IncRef();
	}
}

func (srv *Srv) attach(req *Req)
{
	tc := req.Tc;
	conn := req.Conn;
	if tc.Fid==p9.Nofid {
		req.RespondError(Eunknownfid);
		return;
	}

	req.Fid = conn.fidNew(tc.Fid);
	if req.Fid==nil {
		req.RespondError(Einuse);
		return;
	}

	if tc.Afid!=p9.Nofid {
		req.Afid = conn.fidGet(tc.Afid);
		if req.Afid==nil {
			req.RespondError(Eunknownfid);
		}
	}

	var user *User = nil;
	if tc.Uname!="" {
		user = srv.Upool.Uname2User(tc.Uname);
	} else if tc.Nuname!=p9.Nouid {
		user = srv.Upool.Uid2User(int(tc.Nuname));
	}

	if user==nil {
		req.RespondError(Enouser);
		return;
	}

	req.Fid.User = user;
	if srv.Auth!=nil {
		err := srv.Auth.Check(req.Fid, req.Afid, tc.Aname);
		if err!=nil {
			req.RespondError(err);
			return;
		}
	}

	srv.Impl.Attach(req);
}

func (srv *Srv) attachPost(req *Req)
{
	if req.Rc!=nil && req.Rc.Id==p9.Rattach {
		req.Fid.Type = req.Rc.Fqid.Type;
		req.Fid.IncRef();
	}
}

func (srv *Srv) flush(req *Req)
{
	var r *Req;

	conn := req.Conn;
	tag := req.Tc.Oldtag;
	p9.PackRflush(req.Rc);
	conn.Lock();
	for r=conn.reqfirst; r!=nil; r=r.next {
		if r.Tc.Tag==tag {
			break;
		}
	}
	conn.Unlock();

	if r!=nil {
		r.Lock();
		r.flushreq = req.flushreq;
		r.flushreq = req;
		status := r.status;
		r.status |= reqFlush;
		r.Unlock();

		if (status&(reqWork|reqSaved))==0 {
			/* the request is not worked on yet */
			r.Respond();
		} else {
			srv.Impl.Flush(r);
		}
	} else {
		req.Respond();
	}
}

func (srv *Srv) walk(req *Req)
{
	conn := req.Conn;
	tc := req.Tc;
	fid := req.Fid;

	/* we can't walk regular files, only clone them */
	if len(tc.Wnames)>0 && (fid.Type&p9.QTDIR)==0 {
		req.RespondError(Enotdir);
		return;
	}

	/* we can't walk open files */
	if fid.opened {
		req.RespondError(Ebaduse);
		return;
	}

	if tc.Fid!=tc.Newfid {
		req.Newfid = conn.fidNew(tc.Newfid);
		if req.Newfid==nil {
			req.RespondError(Einuse);
			return;
		}
	} else {
		req.Newfid = req.Fid;
	}

	srv.Impl.Walk(req);
}

func (srv *Srv) walkPost(req *Req)
{
	rc := req.Rc;
	if rc==nil || rc.Id==p9.Rwalk || req.Newfid==nil {
		return;
	}

	n := len(rc.Wqids);
	if n>0 {
		req.Newfid.Type = rc.Wqids[n-1].Type;
	}

	if req.Newfid != req.Fid {
		req.Newfid.IncRef();
	}
}

func (srv *Srv) open(req *Req)
{
	fid := req.Fid;
	tc := req.Tc;
	if fid.opened {
		req.RespondError(Eopen);
		return;
	}

	if (fid.Type&p9.QTDIR)!=0 && tc.Mode!=p9.OREAD {
		req.RespondError(Eperm);
		return;
	}

	fid.Omode = tc.Mode;
	srv.Impl.Open(req);
}

func (srv *Srv) openPost(req *Req)
{
	req.Fid.opened = req.Rc!=nil && req.Rc.Id==p9.Ropen && req.Fid!=nil;
}

func (srv *Srv) create(req *Req)
{
	fid := req.Fid;
	tc := req.Tc;
	if fid.opened {
		req.RespondError(Eopen);
		return;
	}

	if (fid.Type&p9.QTDIR)!=0 {
		req.RespondError(Enotdir);
		return;
	}

	/* can't open directories for other than reading */
	if (tc.Perm&p9.DMDIR)!=0 && tc.Mode!=p9.OREAD {
		req.RespondError(Eperm);
		return;
	}

	/* can't create special files if not 9P2000.u */
	if (tc.Perm&(p9.DMNAMEDPIPE|p9.DMSYMLINK|p9.DMLINK|p9.DMDEVICE|p9.DMSOCKET))!=0 && !req.Conn.Dotu {
		req.RespondError(Eperm);
		return;
	}

	fid.Omode = tc.Mode;
	srv.Impl.Create(req);
}

func (srv *Srv) createPost(req *Req)
{
	if req.Rc!=nil && req.Rc.Id==p9.Rcreate && req.Fid!=nil {
		req.Fid.Type = req.Rc.Fqid.Type;
		req.Fid.opened = true;
	}
}

func (srv *Srv) read(req *Req)
{
	tc := req.Tc;
	fid := req.Fid;
	if tc.Count+p9.IOHdrSz>req.Conn.Msize {
		req.RespondError(Etoolarge);
	}

	if (fid.Type&p9.QTAUTH)!=0 {
		var n int;

		rc := req.Tc;
		err := p9.InitRread(rc, tc.Count);
		if err!=nil {
			req.RespondError(err);
			return;
		}

		n, err = srv.Auth.Read(fid, tc.Offset, rc.Data);
		if err!=nil {
			req.RespondError(err);
			return;
		}
		p9.SetRreadCount(rc, uint32(n));
		req.Respond();
		return;
	}

	if (fid.Type&p9.QTDIR)!=0 && tc.Offset>0 && tc.Offset!=fid.Diroffset {
		req.RespondError(Ebadoffset);
		return;
	}

	srv.Impl.Read(req);
}

func (srv *Srv) readPost(req *Req)
{
	if req.Rc!=nil && req.Rc.Id==p9.Rread && (req.Fid.Type&p9.QTDIR)!=0 {
		req.Fid.Diroffset += uint64(req.Rc.Count);
	}
}


func (srv *Srv) write(req *Req)
{
	fid := req.Fid;
	tc := req.Tc;
	if (fid.Type&p9.QTAUTH)!=0 {
		if srv.Auth==nil {
			req.RespondError(Enoauth);
			return;
		}

		tc := req.Tc;
		n, err := srv.Auth.Write(req.Fid, tc.Offset, tc.Data);
		if err!=nil {
			req.RespondError(err);
		} else {
			req.RespondRwrite(uint32(n));
		}

		return;
	}

	if !fid.opened || (fid.Type&p9.QTDIR)!=0 || (fid.Omode&3)==p9.OREAD {
		req.RespondError(Ebaduse);
		return;
	}

	if tc.Count+p9.IOHdrSz>req.Conn.Msize {
		req.RespondError(Etoolarge);
		return;
	}

	srv.Impl.Write(req);
}

func (srv *Srv) clunk(req *Req)
{
	fid := req.Fid;
	if (fid.Type&p9.QTDIR) != 0 {
		if srv.Auth==nil {
			req.RespondError(Enoauth);
			return;
		}

		srv.Auth.Destroy(fid);
		req.RespondRclunk();
		return;
	}

	srv.Impl.Clunk(req);
}

func (srv *Srv) clunkPost(req *Req)
{
	if req.Rc!=nil && req.Rc.Id==p9.Rclunk && req.Fid!=nil {
		req.Fid.DecRef();
	}
}

func (srv *Srv) remove(req *Req)
{
	srv.Impl.Remove(req);
}

func (srv *Srv) removePost(req *Req)
{
	if req.Rc!=nil && req.Rc.Id==p9.Rremove && req.Fid!=nil {
		req.Fid.DecRef();
	}
}

func (srv *Srv) stat(req *Req)
{
	srv.Impl.Stat(req);
}

func (srv *Srv) wstat(req *Req)
{
	fid := req.Fid;
	stat := req.Tc.Fstat;
	if stat.Type!=uint16(0xFFFF) || stat.Dev!=uint32(0xFFFFFFFF) || stat.Sqid.Version!=uint32(0xFFFFFFFF) ||
			stat.Sqid.Path!=uint64(0xFFFFFFFFFFFFFFFF) {
		req.RespondError(Eperm);
		return;
	}

	if ((fid.Type&p9.QTDIR)!=0 && (stat.Mode&p9.DMDIR)==0) ||
			((fid.Type&p9.QTDIR)==0 && (stat.Mode&p9.DMDIR)!=0) {
		req.RespondError(Edirchange);
	}

	srv.Impl.Wstat(req);
}

func newConn(srv *Srv, c net.Conn)
{
	conn := new(Conn);
	conn.Srv = srv;
	conn.Msize = srv.Msize;
	conn.Dotu = srv.Dotu;
	conn.conn = c;
	conn.fidpool = make(map[uint32] *Fid);
	conn.reqout = make(chan *Req, srv.Maxpend);

	go conn.recv();
	go conn.send();
}

func (conn *Conn) recv()
{
	var err os.Error;
	var n int;

	buf := make([]byte, conn.Msize);
	pos := 0;
	for {
		if len(buf)<int(conn.Msize) {
			b := make([]byte, conn.Msize);
			bytes.Copy(b, buf[0:pos]);
			buf = b;
		}

		n, err = conn.conn.Read(buf[pos:len(buf)]);
		if err!=nil || n==0 {
			goto closed;
		}

		pos += n;
		for pos>4 {
			sz, _ := p9.Gint32(buf);
			if pos<int(sz) {
				break;
			}

			fc, err, fcsize := p9.Unpack(buf, conn.Dotu);
			if err!=nil {
				conn.conn.Close();
				goto closed;
			}

			req := new(Req);
			req.Tc = fc;
			req.Rc = new(p9.Fcall);
			req.Rc.Pkt = make([]byte, conn.Msize);
			req.Conn = conn;

			conn.Lock();
			if conn.reqlast!=nil {
				conn.reqlast.next = req;
			} else {
				conn.reqfirst = req;
			}

			req.prev = conn.reqlast;
			conn.reqlast = req;
			conn.Unlock();
			conn.Srv.Reqin <- req;
			pos -= fcsize;
			buf = buf[0:fcsize];
		}
	}

closed:
	conn.done <- true;
	conn.Srv.Impl.ConnClosed(conn);

	/* call FidDestroy for all remaining fids */
	for _, fid := range conn.fidpool {
		conn.Srv.Impl.FidDestroy(fid);
	}
}

func (conn *Conn) send()
{
	for {
		select {
		case <- conn.done:
			return;

		case req := <-conn.reqout:
			for buf:=req.Rc.Pkt; len(buf)>0; {
				n, err := conn.conn.Write(buf);
				if err!=nil {
					/* just close the socket, will get signal on conn.done */
					conn.conn.Close();
					break;
				}

				buf = buf[n:len(buf)];
			}
		}
	}
}


func StartListener(network, laddr string, srv *Srv) os.Error {
	l, err := net.Listen(network, laddr);
	if err != nil {
		log.Stderr("listen fail: ", network, listen, err);
		return err;
	}

	go listen(l, srv);
	return nil;
}

func listen(l net.Listener, srv *Srv)
{
	for {
		c, err := l.Accept();
		if err != nil {
			break
		}

		newConn(srv, c);
	}
}


/* How to create a server:

func test()
{
	srv := new(Srv);
	srv.Start(srvimpl);
	StartListener("tcp", "xxx", srv);
}

*/
