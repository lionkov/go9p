package p9cl

import (
	"./p9";
	"bytes";
//	"log";
	"net";
	"os";
	"strings";
	"sync";
	"syscall";
)

type Clnt struct {
	sync.Mutex;
	DebugLevel	int;
	Msize		uint32;
	Dotu		bool;
	Root		*Fid;

	conn		net.Conn;
	tagPool		*pool;
	fidPool		*pool;
	reqOut		chan *req;
	done		chan bool;
	reqFirst	*req;
	reqLast		*req;
	err		*p9.Error;
}

type Fid struct {
	sync.Mutex;
	refcount	int;
	Clnt		*Clnt;
	Iounit		uint32;
	Fqid		p9.Qid;
	Mode		uint8;
	Fid		uint32;
	offset		uint64;
	User		*p9.User;
}

type pool struct {
	sync.Mutex;
	need		int;
	nchan		chan uint32;
	maxid		uint32;
	imap		[]byte;
}

type req struct {
	sync.Mutex;
	clnt		*Clnt;
	tc		*p9.Fcall;
	rc		*p9.Fcall;
	err		*p9.Error;
	done		chan *req;
	prev, next	*req;
};

func (clnt *Clnt) rpcnb(r *req) *p9.Error
{
	tag := uint16(clnt.tagPool.getId());
	p9.SetTag(r.rc, tag);
	clnt.Lock();
	if clnt.err!=nil {
		clnt.Unlock();
		return clnt.err;
	}

	if clnt.reqLast!=nil {
		clnt.reqLast.next = r;
	} else {
		clnt.reqFirst = r;
	}

	r.prev = clnt.reqLast;
	clnt.reqLast = r;
	clnt.Unlock();

	clnt.reqOut <- r;
	return nil;
}

func (clnt *Clnt) rpc(tc *p9.Fcall) (*p9.Fcall, *p9.Error)
{
	r := new(req);
	r.tc = tc;
	r.done = make(chan *req);
	err := clnt.rpcnb(r);
	if err!=nil {
		return nil, err;
	}

	<-r.done;
	return r.rc, r.err;
}

func (clnt *Clnt) recv()
{
	var err *p9.Error;

	buf := make([]byte, clnt.Msize);
	pos := 0;
	for {
		if len(buf)<int(clnt.Msize) {
			b := make([]byte, clnt.Msize);
			bytes.Copy(b, buf[0:pos]);
			buf = b;
		}

		n, oerr := clnt.conn.Read(buf[pos:len(buf)]);
		if oerr!=nil || n==0 {
			err = &p9.Error{oerr.String(), syscall.EIO};
			goto closed;
		}

		pos += n;
		for pos>4 {
			sz, _ := p9.Gint32(buf);
			if pos<int(sz) {
				break;
			}

			fc, err, fcsize := p9.Unpack(buf, clnt.Dotu);
			clnt.Lock();
			if err!=nil {
				clnt.err = err;
				clnt.conn.Close();
				clnt.Unlock();
				goto closed;
			}

			var r *req = nil;
			for r=clnt.reqFirst; r!=nil; r=r.next {
				if r.tc.Tag==fc.Tag {
					break;
				}
			}

			if (r==nil) {
				clnt.err = &p9.Error{"unexpected response", syscall.EINVAL};
				clnt.conn.Close();
				clnt.Unlock();
				goto closed;
			}

			r.rc = fc;
			if r.prev!=nil {
				r.prev.next = r.next;
			} else {
				clnt.reqFirst = r.next;
			}

			if r.next!=nil {
				r.next.prev = r.prev;
			} else {
				clnt.reqLast = r.prev;
			}
			clnt.Unlock();

			if r.tc.Id!=r.rc.Id-1 {
				if r.rc.Id!=p9.Rerror {
					r.err = &p9.Error{"invalid response id", syscall.EINVAL};
				} else {
					if r.err!=nil {
						r.err = &p9.Error{r.rc.Error, os.Errno(r.rc.Nerror)};
					}
				}
			}

			clnt.fidPool.putId(uint32(r.tc.Tag));
			if r.done!=nil {
				r.done <- r;
			}

			pos -= fcsize;
			buf = buf[0:fcsize];
		}
	}

closed:
	clnt.done <- true;

	/* send error to all pending requests */
	clnt.Lock();
	r := clnt.reqFirst;
	clnt.reqFirst = nil;
	clnt.reqLast = nil;
	err = clnt.err;
	clnt.Unlock();
	for ;r!=nil; r=r.next {
		r.err = err;
		if r.done!=nil {
			r.done <- r;
		}
	}
}

func (clnt *Clnt) send()
{
	for {
		select {
		case <- clnt.done:
			return;

		case req := <-clnt.reqOut:
			for buf:=req.tc.Pkt; len(buf)>0; {
				n, err := clnt.conn.Write(buf);
				if err!=nil {
					/* jsut close the socket, will get signal on clnt.done */
					clnt.conn.Close();
					break;
				}

				buf = buf[n:len(buf)];
			}
		}
	}
}

func NewClnt(c net.Conn, msize uint32, dotu bool) (*Clnt)
{
	clnt := new(Clnt);
	clnt.conn = c;
	clnt.Msize = msize;
	clnt.Dotu = dotu;
	clnt.tagPool = newPool(uint32(p9.Notag));
	clnt.fidPool = newPool(p9.Nofid);
	clnt.reqOut = make(chan *req);
	clnt.done = make(chan bool);

	go clnt.recv();
	go clnt.send();

	return clnt;
}

func Connect(ntype, addr string, msize uint32, dotu bool) (*Clnt, *p9.Error)
{
	c, e := net.Dial(ntype, "", addr);
	if e!=nil {
		return nil, &p9.Error{e.String(), syscall.EIO};
	}

	clnt := NewClnt(c, msize, dotu);
	ver := "9P2000";
	if clnt.Dotu {
		ver = "9P2000.u";
	}

	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTversion(tc, clnt.Msize, ver);
	if err!=nil {
		return nil, err;
	}

	rc, err := clnt.rpc(tc);
	if err!=nil {
		return nil, err;
	}

	if rc.Msize<clnt.Msize {
		clnt.Msize = rc.Msize;
	}

	clnt.Dotu = rc.Version=="9P2000.u" && clnt.Dotu;
	return clnt, nil;
}

func Mount(net, addr, aname string, user *p9.User) (*Clnt, *p9.Error)
{
	clnt, err := Connect(net, addr, 8192+p9.IOHdrSz, true);
	if err!=nil {
		return nil, err;
	}

	fid := clnt.FidAlloc();
	tc := p9.NewFcall(clnt.Msize);
	err = p9.PackTattach(tc, fid.Fid, p9.Nofid, user.Name(), aname, uint32(user.Id()), clnt.Dotu);
	if err!=nil {
		return nil, err;
	}

	rc, err := clnt.rpc(tc);
	if err!= nil {
		return nil, err;
	}

	fid.User = user;
	fid.Fqid = rc.Fqid;
	clnt.Root = fid;
	return clnt, nil;
}

func (clnt *Clnt) Unmount()
{
	clnt.Lock();
	clnt.err = &p9.Error{"connection closed", syscall.ECONNRESET};
	clnt.conn.Close();
	clnt.Unlock();
}

func (clnt *Clnt) Walk(path string) (*Fid, *p9.Error)
{
	var err *p9.Error = nil;
	wnames := strings.Split(path, "/", 0);
	newfid := clnt.FidAlloc();
	fid := clnt.Root;
	newfid.User = fid.User;
	for len(wnames) > 0 {
		n := len(wnames);
		if n>16 {
			n = 16;
		}

		tc := p9.NewFcall(clnt.Msize);
		err := p9.PackTwalk(tc, fid.Fid, newfid.Fid, wnames[0:n]);
		if err!=nil {
			goto error;
		}

		rc, err := clnt.rpc(tc);
		if err!=nil {
			goto error;
		}

		if len(rc.Wqids)!=n {
			err = &p9.Error{"file not found", syscall.ENOENT};
			goto error;
		}

		newfid.Fqid = rc.Wqids[len(rc.Wqids)-1];
		wnames = wnames[n:len(wnames)];
		fid = newfid;
	}

	return newfid, nil;

error:
	newfid.DecRef();
	return nil, err;
}

func (clnt *Clnt) Create(path string, perm uint32, mode uint8) (*Fid, *p9.Error)
{
	n := strings.LastIndex(path, "/");
	if n<0 {
		n = 0;
	}

	fid, err := clnt.Walk(path[0:n]);
	if err!=nil {
		return nil, err;
	}

	tc := p9.NewFcall(clnt.Msize);
	err = p9.PackTcreate(tc, fid.Fid, path[n:len(path)], perm, mode, "", clnt.Dotu);
	if err!=nil {
		goto error;
	}

	rc, err := clnt.rpc(tc);
	if err!=nil {
		goto error;
	}

	fid.Fqid = rc.Fqid;
	fid.Iounit = rc.Iounit;
	fid.Mode = rc.Mode;
	return fid, nil;

error:
	fid.DecRef();
	return nil, err;
}

func (clnt *Clnt) Open(path string, mode uint8) (*Fid, *p9.Error)
{
	fid, err := clnt.Walk(path);
	if err!=nil {
		return nil, err;
	}

	tc := p9.NewFcall(clnt.Msize);
	err = p9.PackTopen(tc, fid.Fid, mode);
	if err!=nil {
		goto error;
	}

	rc, err := clnt.rpc(tc);
	if err!=nil {
		goto error;
	}

	fid.Fqid = rc.Fqid;
	fid.Iounit = rc.Iounit;
	fid.Mode = rc.Mode;
	return fid, nil;

error:
	fid.DecRef();
	return nil, err;
}

func (clnt *Clnt) Remove(path string) *p9.Error
{
	var err *p9.Error;
	fid, err := clnt.Walk(path);
	if err!=nil {
		return err;
	}

	tc := p9.NewFcall(clnt.Msize);
	err = p9.PackTremove(tc, fid.Fid);
	if err!=nil {
		goto error;
	}

	_, err = clnt.rpc(tc);
	if err != nil {
error:
		fid.DecRef();
		return err;
	}

	return nil;
}

func (clnt *Clnt) Stat(path string) (*p9.Stat, *p9.Error)
{
	fid, err := clnt.Walk(path);
	if err!=nil {
		return nil, err;
	}

	tc := p9.NewFcall(clnt.Msize);
	err = p9.PackTstat(tc, fid.Fid);
	if err!=nil {
		goto error;
	}

	rc, err := clnt.rpc(tc);
	if err != nil {
error:
		fid.DecRef();
		return nil, err;
	}

	return &rc.Fstat, nil;
}

func (fid *Fid) Read(buf []byte, offset uint64) (int, *p9.Error)
{
	tc := p9.NewFcall(fid.Clnt.Msize);
	err := p9.PackTread(tc, fid.Fid, offset, uint32(len(buf)));
	if err!=nil {
		return 0, err;
	}

	rc, err := fid.Clnt.rpc(tc);
	if err != nil {
		return 0, err;
	}

	bytes.Copy(buf, rc.Data);
	return len(rc.Data), nil;
}

func (fid *Fid) Readn(buf []byte, offset uint64) (int, *p9.Error)
{
	ret := 0;
	for len(buf)>0 {
		n, err := fid.Read(buf, offset);
		if err!=nil {
			return 0, err;
		}

		if n==0 {
			break;
		}

		buf = buf[n:len(buf)];
		offset += uint64(n);
		ret += n;
	}

	return ret, nil;
}

func (fid *Fid) Write(buf []byte, offset uint64) (int, *p9.Error)
{
	tc := p9.NewFcall(fid.Clnt.Msize);
	err := p9.PackTwrite(tc, fid.Fid, offset, buf);
	if err!=nil {
		return 0, err;
	}

	rc, err := fid.Clnt.rpc(tc);
	if err != nil {
		return 0, err;
	}

	return int(rc.Count), nil;
}

func (fid *Fid) Writen(buf []byte, offset uint64) (int, *p9.Error)
{
	ret := 0;
	for len(buf)>0 {
		n, err := fid.Write(buf, offset);
		if err!=nil {
			return 0, err;
		}

		if n==0 {
			break;
		}

		buf = buf[n:len(buf)];
		offset += uint64(n);
		ret += n;
	}

	return ret, nil;
}

func (fid *Fid) DirRead() ([]*p9.Stat, *p9.Error)
{
	buf := make([]byte, fid.Clnt.Msize-p9.IOHdrSz);
	stats := make([]*p9.Stat, 32);
	pos := 0;
	for {
		n, err := fid.Read(buf, fid.offset);
		if err!=nil {
			return nil, err;
		}

		if n==0 {
			break;
		}

		for p:=buf; len(p)>0; {
			var st *p9.Stat;
			st, err, p = p9.UnpackStat(p, fid.Clnt.Dotu);
			if err!=nil {
				return nil, err;
			}

			if pos>=len(stats) {
				s := make([]*p9.Stat, len(stats)+32);
				for i:=0; i<len(stats); i++ {
					s[i] = stats[i];
				}

				stats = s;
			}

			stats[pos] = st;
			pos++;
		}
	}

	return stats[0:pos], nil;
}

func (fid *Fid) Clunk() *p9.Error
{
	tc := p9.NewFcall(fid.Clnt.Msize);
	err := p9.PackTclunk(tc, fid.Fid);
	if err!=nil {
		return err;
	}

	_, err = fid.Clnt.rpc(tc);
	if err != nil {
		return err;
	}

	return nil;
}

/* the C* methods correspond directly to the 9P calls */
func (clnt *Clnt) Cauth(user *p9.User, aname string) (*Fid, *p9.Error)
{
	fid := clnt.FidAlloc();
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTauth(tc, fid.Fid, user.Name(), aname, uint32(user.Id()), clnt.Dotu);
	if err!=nil {
		return nil, err;
	}

	_, err = clnt.rpc(tc);
	if err!= nil {
		return nil, err;
	}

	fid.User = user;
	return fid, nil;
}

func (clnt *Clnt) Cattach(afid *Fid, user *p9.User, aname string) (*Fid, *p9.Error)
{
	fid := clnt.FidAlloc();
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTattach(tc, fid.Fid, afid.Fid, user.Name(), aname, uint32(user.Id()), clnt.Dotu);
	if err!=nil {
		return nil, err;
	}

	rc, err := clnt.rpc(tc);
	if err!= nil {
		return nil, err;
	}

	fid.Fqid = rc.Fqid;
	fid.User = user;
	return fid, nil;
}

func (clnt *Clnt) Cwalk(fid *Fid, newfid *Fid, wnames []string) ([]p9.Qid, *p9.Error)
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTwalk(tc, fid.Fid, newfid.Fid, wnames);
	if err!=nil {
		return nil, err;
	}

	rc, err := clnt.rpc(tc);
	if err!= nil {
		return nil, err;
	}

	return rc.Wqids, nil;
}

func (clnt *Clnt) Copen(fid *Fid, mode uint8) (uint32, *p9.Qid, *p9.Error)
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTopen(tc, fid.Fid, mode);
	if err!=nil {
		return 0, nil, err;
	}

	rc, err := clnt.rpc(tc);
	if err!= nil {
		return 0, nil, err;
	}

	fid.Fqid = rc.Fqid;
	fid.Iounit = rc.Iounit;
	if fid.Iounit==0 || fid.Iounit>clnt.Msize-p9.IOHdrSz {
		fid.Iounit = clnt.Msize - p9.IOHdrSz;
	}
	fid.Mode = mode;

	return fid.Iounit, &fid.Fqid, nil;
}

func (clnt *Clnt) Ccreate(fid *Fid, name string, perm uint32, mode uint8,
					ext string) (uint32, *p9.Qid, *p9.Error)
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTcreate(tc, fid.Fid, name, perm, mode, ext, clnt.Dotu);
	if err!=nil {
		return 0, nil, err;
	}

	rc, err := clnt.rpc(tc);
	if err!= nil {
		return 0, nil, err;
	}

	fid.Fqid = rc.Fqid;
	fid.Iounit = rc.Iounit;
	if fid.Iounit==0 || fid.Iounit>clnt.Msize-p9.IOHdrSz {
		fid.Iounit = clnt.Msize - p9.IOHdrSz;
	}
	fid.Mode = mode;

	return fid.Iounit, &fid.Fqid, nil;
}

func (clnt *Clnt) Cread(fid *Fid, offset uint64, count uint32) ([]byte, *p9.Error)
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTread(tc, fid.Fid, offset, count);
	if err!=nil {
		return nil, err;
	}

	rc, err := clnt.rpc(tc);
	if err!= nil {
		return nil, err;
	}

	return rc.Data, nil;
}

func (clnt *Clnt) Cwrite(fid *Fid, data []byte, offset uint64) (int, *p9.Error)
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTwrite(tc, fid.Fid, offset, data);
	if err!=nil {
		return 0, err;
	}

	rc, err := clnt.rpc(tc);
	if err!= nil {
		return 0, err;
	}

	return int(rc.Count), nil;
}

func (clnt *Clnt) Cstat(fid *Fid) (*p9.Stat, *p9.Error)
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTstat(tc, fid.Fid);
	if err!=nil {
		return nil, err;
	}

	rc, err := clnt.rpc(tc);
	if err!= nil {
		return nil, err;
	}

	return &rc.Fstat, nil;
}

func (clnt *Clnt) Cclunk(fid *Fid) *p9.Error
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTclunk(tc, fid.Fid);
	if err!=nil {
		return err;
	}

	_, err = clnt.rpc(tc);
	fid.DecRef();
	return err;
}

func (clnt *Clnt) Cremove(fid *Fid) *p9.Error
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTremove(tc, fid.Fid);
	if err!=nil {
		return err;
	}

	_, err = clnt.rpc(tc);
	fid.DecRef();
	return err;
}

func (clnt *Clnt) Cwstat(fid *Fid, st *p9.Stat) *p9.Error
{
	tc := p9.NewFcall(clnt.Msize);
	err := p9.PackTwstat(tc, fid.Fid, st, clnt.Dotu);
	if err!=nil {
		return err;
	}

	_, err = clnt.rpc(tc);
	return err;
}

func (clnt *Clnt) FidAlloc() *Fid
{
	fid := new(Fid);
	fid.Fid = clnt.fidPool.getId();
	fid.Clnt = clnt;
	fid.refcount = 1;

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
	if fid.refcount>0 {
		fid.Unlock();
		return;
	}

	n := fid.Fid;
	fid.Fid = p9.Nofid;
	fid.Unlock();

	fid.Clnt.fidPool.putId(n);
}

var m2id = [...] uint8 {
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 4, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 5, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 4, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 6, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 4, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 5, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 4, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 7, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 4, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 5, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 4, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 6, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 4, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 5, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 4, 
0, 1, 0, 2, 0, 1, 0, 3, 
0, 1, 0, 2, 0, 1, 0, 0,
};

func newPool(maxid uint32) *pool
{
	p := new(pool);
	p.maxid = maxid;
	p.nchan = make(chan uint32);

	return p;	
}

func (p *pool) getId() uint32
{
	var n uint32 = 0;
	var ret uint32;

	p.Lock();
	for n=0; n<uint32(len(p.imap)); n++ {
		if p.imap[n]!=0xFF {
			break;
		}
	}

	if int(n)>=len(p.imap) {
		m := uint32(len(p.imap) + 32);
		if uint32(m*8)>p.maxid {
			m = p.maxid/8 + 1;
		}

		b := make([]byte, m);
		bytes.Copy(b, p.imap);
		p.imap = b;
	}

	if n>=uint32(len(p.imap)) {
		p.need++;
		p.Unlock();
		ret = <-p.nchan;
	} else {
		ret = uint32(m2id[p.imap[n]]);
		p.imap[n] |= 1<<ret;
		ret += n*8;
		p.Unlock();
	}

	return ret;
}

func (p *pool) putId(id uint32)
{
	p.Lock();
	if p.need>0 {
		p.nchan <- id;
		p.need--;
		p.Unlock();
	}

	p.imap[id/8] &= ^(1<<(id%8));
	p.Unlock();
}
