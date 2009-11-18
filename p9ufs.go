package main

import (
	"fmt";
	"log";
	"os";
	"strconv";
	"strings";
	"syscall";
	"./p9";
	"./p9srv";
)

type Fid struct {
	path		string;
	file		*os.File;
	dirs		[]os.Dir;
	diroffset	uint64;
	st		*os.Dir;
}

type Ufs struct {
	p9srv.Srv;
}

var Enoent	*p9.Error = &p9.Error{"file not found", syscall.ENOENT};


func (fid *Fid) stat() *p9.Error {
	var err os.Error;

	fid.st, err = os.Lstat(fid.path);
	if err!=nil {
		return &p9.Error{err.String(), 0};	// TODO: fix error code
	}

	return nil;
}

func omode2uflags(mode uint8) int {
        ret := int(0);
        switch (mode & 3) {
        case p9.OREAD:
                ret = os.O_RDONLY;
                break;

        case p9.ORDWR:
                ret = os.O_RDWR;
                break;

        case p9.OWRITE:
                ret = os.O_WRONLY;
                break;

        case p9.OEXEC:
                ret = os.O_RDONLY;
                break;
        }

        if mode&p9.OTRUNC!=0 {
                ret |= os.O_TRUNC;
	}

        return ret;
}

func dir2Qid(d *os.Dir) *p9.Qid {
	var qid p9.Qid;

	qid.Path = d.Ino;
	qid.Version = uint32(d.Mtime_ns/1000000);
	qid.Type = dir2QidType(d);

	return &qid;
}

func dir2QidType(d *os.Dir) uint8 {
	ret := uint8(0);
	if d.IsDirectory() {
		ret |= p9.QTDIR;
	}

	if d.IsSymlink() {
		ret |= p9.QTSYMLINK;
	}

	return ret;
}

func dir2Npmode(d *os.Dir, dotu bool) uint32 {
	ret := uint32(d.Mode&0777);
	if d.IsDirectory() {
		ret |= p9.DMDIR;
	}

	if dotu {
		if d.IsSymlink() {
			ret |= p9.DMSYMLINK;
		}

		if d.IsSocket() {
			ret |= p9.DMSOCKET;
		}

		if d.IsFifo() {
			ret |= p9.DMNAMEDPIPE;
		}

		if d.IsBlock() || d.IsChar() {
			ret |= p9.DMDEVICE;
		}

		/* TODO: setuid and setgid */
	}

	return ret;		
}

func dir2Stat(path string, d *os.Dir, dotu bool, upool p9.Users) *p9.Stat {
	st := new(p9.Stat);
	st.Sqid = *dir2Qid(d);
	st.Mode = dir2Npmode(d, dotu);
	st.Atime = uint32(d.Atime_ns/1000000000);
	st.Mtime = uint32(d.Mtime_ns/1000000000);
	st.Length = d.Size;

	u := upool.Uid2User(int(d.Uid));
	g := upool.Gid2Group(int(d.Gid));
	st.Uid = u.Name();
	st.Gid = g.Name();
	st.Muid = "";
	st.Ext = "";
	if dotu {
		st.Nuid = uint32(u.Id());
		st.Ngid = uint32(g.Id());
		st.Nmuid = p9.Nouid;
		if d.IsSymlink() {
			var err os.Error;
			st.Ext, err = os.Readlink(path);
			if err!=nil {
				st.Ext = "";
			}
		} else if d.IsBlock() {
			st.Ext = fmt.Sprintf("b %d %d", d.Rdev>>24, d.Rdev&0xFFFFFF);
		} else if d.IsChar() {
			st.Ext = fmt.Sprintf("c %d %d", d.Rdev>>24, d.Rdev&0xFFFFFF);
		}
	}

	st.Name = path[strings.LastIndex(path, "/")+1:len(path)];
	return st;
}

func (*Ufs) ConnOpened(*p9srv.Conn) {
	log.Stderr("connected");
}

func (*Ufs) ConnClosed(*p9srv.Conn) {
	log.Stderr("disconnected");
}

func (*Ufs) FidDestroy(sfid *p9srv.Fid) {
	var fid *Fid;
	fid = sfid.Aux.(*Fid);

	if fid.file!=nil {
		fid.file.Close();
	}
}

func (ufs *Ufs) ReqProcess(req *p9srv.Req) {
	req.Process();
}

func (*Ufs) Attach(req *p9srv.Req) {
	if req.Afid!=nil {
		req.RespondError(p9srv.Enoauth);
		return;
	}

	tc := req.Tc;
	fid := new(Fid);
	if len(tc.Aname)==0 {
		fid.path = "/";
	} else {
		fid.path = tc.Aname;
	}

	req.Fid.Aux = fid;
	err := fid.stat();
	if err!=nil {
		req.RespondError(err);
	}

	qid := dir2Qid(fid.st);
	req.RespondRattach(qid);
}

func (*Ufs) Flush(req *p9srv.Req) {
}

func (*Ufs) Walk(req *p9srv.Req) {
	fid := req.Fid.Aux.(*Fid);
	tc := req.Tc;

	err:=fid.stat();
	if err!=nil {
		req.RespondError(err);
		return;
	}

	if req.Newfid.Aux==nil {
		req.Newfid.Aux = new(Fid);
	}

	nfid := req.Newfid.Aux.(*Fid);
	wqids := make([]p9.Qid, len(tc.Wnames));
	path := fid.path;
	i := 0;
	for ; i<len(tc.Wnames); i++ {
		p := path + "/" + tc.Wnames[i];
		st, err := os.Lstat(p);
		if err!=nil {
			if i==0 {
				req.RespondError(Enoent);
				return;
			}

			break;
		}

		wqids[i] = *dir2Qid(st);
		path = p;
	}

	nfid.path = path;
	req.RespondRwalk(wqids[0:i]);
}

func (*Ufs) Open(req *p9srv.Req){
	fid := req.Fid.Aux.(*Fid);
	tc := req.Tc;
	err:=fid.stat();
	if err!=nil {
		req.RespondError(err);
		return;
	}

	var e os.Error;
	fid.file, e = os.Open(fid.path, omode2uflags(tc.Mode), 0);
	if e!=nil {
		req.RespondError(&p9.Error{e.String(), 0});
		return;
	}

	req.RespondRopen(dir2Qid(fid.st), 0);
}

func (*Ufs) Create(req *p9srv.Req){
	fid := req.Fid.Aux.(*Fid);
	tc := req.Tc;
	err:=fid.stat();
	if err!=nil {
		req.RespondError(err);
	}

	path := fid.path + "/" + tc.Name;
	var e os.Error = nil;
	var file *os.File = nil;
	switch {
	case tc.Perm&p9.DMDIR!=0:
		e = os.Mkdir(path, int(tc.Perm&0777));

	case tc.Perm&p9.DMSYMLINK!=0:
		e = os.Symlink(tc.Ext, path);

	case tc.Perm&p9.DMLINK!=0:
		n, e := strconv.Atoui(tc.Ext);
		if e!=nil {
			break;
		}

		ofid := req.Conn.FidGet(uint32(n));
		if ofid==nil {
			req.RespondError(p9srv.Eunknownfid);
			return;
		}

		e = os.Link(ofid.Aux.(*Fid).path, path);
		ofid.DecRef();

	case tc.Perm&p9.DMNAMEDPIPE!=0:
	case tc.Perm&p9.DMDEVICE!=0:
		req.RespondError(&p9.Error{"not implemented", syscall.EIO});
		return;

	default:
		file, e = os.Open(path, omode2uflags(tc.Mode) | os.O_CREATE, int(tc.Perm&0777));
	}

	if file==nil && e==nil {
		file, e = os.Open(path, omode2uflags(tc.Mode), 0);
	}

	if e!=nil {
		req.RespondError(&p9.Error{e.String(), 0});
		return;
	}

	fid.path = path;
	fid.file = file;
	err=fid.stat();
	if err!=nil {
		req.RespondError(err);
	}

	req.RespondRcreate(dir2Qid(fid.st), 0);
}

func (*Ufs) Read(req *p9srv.Req){
	fid := req.Fid.Aux.(*Fid);
	tc := req.Tc;
	rc := req.Rc;
	err:=fid.stat();
	if err!=nil {
		req.RespondError(err);
	}

	p9.InitRread(rc, tc.Count);
	var count int;
	var e os.Error;
	if fid.st.IsDirectory() {
		p := rc.Data;
		for len(p) > 0 {
			if fid.dirs == nil {
				fid.dirs, e = fid.file.Readdir(16);
				if e!=nil {
					req.RespondError(&p9.Error{e.String(), 0});
					return;
				}

				if len(fid.dirs)==0 {
					break;
				}
			}

			var i int;
			for i=0; i<len(fid.dirs); i++ {
				path := fid.path + "/" + fid.dirs[i].Name;
				st := dir2Stat(path, &fid.dirs[i], req.Conn.Dotu, req.Conn.Srv.Upool);
				sz := p9.PackStat(st, p, req.Conn.Dotu);
				if sz==0 {
					break;
				}

				p = p[sz:len(p)];
				count += sz;
			}

			if i<len(fid.dirs) {
				fid.dirs = fid.dirs[i:len(fid.dirs)];
				break;
			} else {
				fid.dirs = nil;
			}
		}
	} else {
		count, e = fid.file.Read(rc.Data);
		if e!=nil && e!=os.EOF {
			req.RespondError(&p9.Error{e.String(), 0});
			return;
		}
	}

	p9.SetRreadCount(rc, uint32(count));
	req.Respond();
}

func (*Ufs) Write(req *p9srv.Req){
	fid := req.Fid.Aux.(*Fid);
	tc := req.Tc;
	err:=fid.stat();
	if err!=nil {
		req.RespondError(err);
		return;
	}

	n, e := fid.file.Write(tc.Data);
	if e!=nil {
		req.RespondError(&p9.Error{e.String(), 0});
		return;
	}

	req.RespondRwrite(uint32(n));
}

func (*Ufs) Clunk(req *p9srv.Req){
	req.RespondRclunk();
}

func (*Ufs) Remove(req *p9srv.Req){
	fid := req.Fid.Aux.(*Fid);
	err:=fid.stat();
	if err!=nil {
		req.RespondError(err);
		return;
	}

	e := os.Remove(fid.path);
	if e!=nil {
		req.RespondError(&p9.Error{e.String(), 0});
		return;
	}

	req.RespondRremove();
}

func (*Ufs) Stat(req *p9srv.Req){
	fid := req.Fid.Aux.(*Fid);
	err:=fid.stat();
	if err!=nil {
		req.RespondError(err);
		return;
	}

	st := dir2Stat(fid.path, fid.st, req.Conn.Dotu, req.Conn.Srv.Upool);
	req.RespondRstat(st);
}

func (*Ufs) Wstat(req *p9srv.Req){
	var uid, gid uint32;

	fid := req.Fid.Aux.(*Fid);
	err:=fid.stat();
	if err!=nil {
		req.RespondError(err);
		return;
	}

	st := &req.Tc.Fstat;
	up := req.Conn.Srv.Upool;
	if req.Conn.Dotu {
		uid = st.Nuid;
		gid = st.Ngid;
	} else {
		uid = p9.Nouid;
		gid = p9.Nouid;
	}

	if uid==p9.Nouid && st.Uid!="" {
		user := up.Uname2User(st.Uid);
		if user==nil {
			req.RespondError(p9srv.Enouser);
			return;
		}

		uid = uint32(user.Id());
	}

	if gid==p9.Nouid && st.Gid!="" {
		group := up.Gname2Group(st.Gid);
		if group==nil {
			req.RespondError(p9srv.Enouser);
			return;
		}

		gid = uint32(group.Id());
	}

	if st.Mode!=0xFFFFFFFF {
		e := os.Chmod(fid.path, int(st.Mode&0777));
		if e!=nil {
			req.RespondError(&p9.Error{e.String(), 0});
			return;
		}
	}

	if gid!=0xFFFFFFFF || uid!=0xFFFFFFFF {
		e := os.Chown(fid.path, int(uid), int(gid));
		if e!=nil {
			req.RespondError(&p9.Error{e.String(), 0});
			return;
		}
	}

	if st.Name!="" {
		/* no os.Rename */
		req.RespondError(&p9.Error{"not implemented", syscall.EINVAL});
		return;
	}

	if st.Length!=0xFFFFFFFFFFFFFFFF {
		e := os.Truncate(fid.path, int64(st.Length));
		if e!=nil {
			req.RespondError(&p9.Error{e.String(), 0});
			return;
		}
	}

	req.RespondRwstat();
}

func main() {
	ufs := new(Ufs);
	ufs.Dotu = true;
	ufs.Debuglevel = 2;
	ufs.Start(ufs);
	p9srv.StartListener("tcp", ":5640", &ufs.Srv);
}
