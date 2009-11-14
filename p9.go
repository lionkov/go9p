package p9

import "os"
import "syscall"
import "bytes"

const (
	Tfirst, Tversion uint8 = 100+iota, 100+iota;
	Rversion = 100+iota;
	Tauth;
	Rauth;
	Tattach;
	Rattach;
	Terror;
	Rerror;
	Tflush;
	Rflush;
	Twalk;
	Rwalk;
	Topen;
	Ropen;
	Tcreate;
	Rcreate;
	Tread;
	Rread;
	Twrite;
	Rwrite;
	Tclunk;
	Rclunk;
	Tremove;
	Rremove;
	Tstat;
	Rstat;
	Twstat;
	Rwstat;
	Tlast;
);

const(
	IOHdrSz = 24;
	Port = 564;
)

const(
	//Qid.type
	QTFILE = 1<<iota;
	QTSYMLINK;
	QTTMP;
	QTAUTH;
	QTMOUNT;
	QTEXCL;
	QTAPPEND;
	QTDIR
)

const(
	AEXIST = iota;
	AEXEC;
	AWRITE;
	AREAD
)

const(
	// from p9p, included for completeness
	OREAD = iota;	// open for readind
	OWRITE;			// open for writing
	ORDWR;			// read and write
	OEXEC;			// execute (== read but check execute permission)
	OTRUNC = 16;	// or'ed in (except for exec), truncate file first
	OCEXEC = 32;	// or'ed in, close on exec
	ORCLOSE = 64;	// or'ed in, remove on close
	ODIRECT = 128;	// or'ed in, direct access
	ONONBLOCK = 256;// or'ed in, non-blocking call
	OEXCL = 0x1000;	// or'ed in, exclusive use (create only)
	OLOCK = 0x2000; // or'ed in, lock after opening
	OAPPEND = 0x4000;// or'ed in, append only
)

const( 
	// Dir.mode
	DMDIR       =0x80000000; // mode bit for directories 
	DMAPPEND    =0x40000000; // mode bit for append only files 
	DMEXCL      =0x20000000; // mode bit for exclusive use files 
	DMMOUNT     =0x10000000; // mode bit for mounted channel 
	DMAUTH      =0x08000000; // mode bit for authentication file 
	DMTMP       =0x04000000; // mode bit for non-backed-up file 
	DMSYMLINK   =0x02000000; // mode bit for symbolic link (Unix, 9P2000.u) 
	DMDEVICE    =0x00800000; // mode bit for device file (Unix, 9P2000.u) 
	DMNAMEDPIPE =0x00200000; // mode bit for named pipe (Unix, 9P2000.u) 
	DMSOCKET    =0x00100000; // mode bit for socket (Unix, 9P2000.u) 
	DMSETUID    =0x00080000; // mode bit for setuid (Unix, 9P2000.u) 
	DMSETGID    =0x00040000; // mode bit for setgid (Unix, 9P2000.u) 
	DMREAD      =0x4;    // mode bit for read permission 
	DMWRITE     =0x2;    // mode bit for write permission 
	DMEXEC      =0x1;    // mode bit for execute permission 
)

const(
	Notag uint16	= 0xFFFF;
	Nofid uint32	= 0xFFFFFFFF;
	Nouid uint32	= 0xFFFFFFFF;
	Errundef uint32 = 0xFFFFFFFF;
);

type Error struct {
	error	string;
	nerror	os.Errno;
}

type Qid struct {
	qtype	uint8;
	version	uint32;
	path	uint64;
}

//TODO: string implementations for debugging
func (s Stat) String() string {
	return "";
}

type Stat struct {
	size	uint16;
	stype	uint16;
	dev	uint32;
	qid	Qid;
	mode	uint32;
	atime	uint32;
	mtime	uint32;
	length	uint64;
	name	string;
	uid	string;
	gid	string;
	muid	string;

	/* 9P2000.u extension */
	ext	string;
	nuid	uint32;
	ngid	uint32;
	nmuid	uint32;
};

//TODO: string implementations for debugging
func (c Call) String() string {
	return "";
}

type Call struct {
	size	uint32;
	id	uint8;
	tag	uint16;

	fid	uint32;
	msize	uint32;			/* Tversion, Rversion */
	version	string;			/* Tversion, Rversion */
	afid	uint32;			/* Tauth, Tattach */
	uname	string;			/* Tauth, Tattach */
	aname	string;			/* Tauth, Tattach */
	qid	Qid;			/* Rauth, Rattach, Ropen, Rcreate */
	error	string;			/* Rerror */
	oldtag	uint16;			/* Tflush */
	newfid	uint32;			/* Twalk */
	wnames	[]string;		/* Twalk */
	wqids	[]Qid;			/* Rwalk */
	mode	uint8;			/* Topen, Tcreate */
	iounit	uint32;			/* Ropen, Rcreate */
	name	string;			/* Tcreate */
	perm	uint32;			/* Tcreate */
	offset	uint64;			/* Tread, Twrite */
	count	uint32;			/* Tread, Rread, Twrite, Rwrite */
	stat	Stat;			/* Rstat, Twstat */
	data	[]uint8;		/* Rread, Twrite */

	/* 9P2000.u extensions */
	nerror	uint32;			/* Rerror */
	ext	string;			/* Tcreate */
	nuname	uint32;			/* Tauth, Tattach */

	pkt	[]uint8;		/* raw packet data */
}

var minFcsize = [...]uint32 {
	6,	/* Tversion msize[4] version[s] */
	6,	/* Rversion msize[4] version[s] */
	8,	/* Tauth fid[4] uname[s] aname[s] */
	13,	/* Rauth aqid[13] */
	12,	/* Tattach fid[4] afid[4] uname[s] aname[s] */
	13,	/* Rattach qid[13] */
	0,	/* Terror */
	2,	/* Rerror ename[s] (ecode[4]) */
	2,	/* Tflush oldtag[2] */
	0,	/* Rflush */
	10,	/* Twalk fid[4] newfid[4] nwname[2] */
	2,	/* Rwalk nwqid[2] */
	5,	/* Topen fid[4] mode[1] */
	17,	/* Ropen qid[13] iounit[4] */
	11,	/* Tcreate fid[4] name[s] perm[4] mode[1] */
	17,	/* Rcreate qid[13] iounit[4] */
	16,	/* Tread fid[4] offset[8] count[4] */
	4,	/* Rread count[4] */
	16,	/* Twrite fid[4] offset[8] count[4] */
	4,	/* Rwrite count[4] */
	4,	/* Tclunk fid[4] */
	0,	/* Rclunk */
	4,	/* Tremove fid[4] */
	0,	/* Rremove */
	4,	/* Tstat fid[4] */
	4,	/* Rstat stat[n] */
	8,	/* Twstat fid[4] stat[n] */
	0,	/* Rwstat */
};

var minFcusize = [...]uint32 {
	6,	/* Tversion msize[4] version[s] */
	6,	/* Rversion msize[4] version[s] */
	12,	/* Tauth fid[4] uname[s] aname[s] */
	13,	/* Rauth aqid[13] */
	16,	/* Tattach fid[4] afid[4] uname[s] aname[s] */
	13,	/* Rattach qid[13] */
	0,	/* Terror */
	6,	/* Rerror ename[s] (ecode[4]) */
	2,	/* Tflush oldtag[2] */
	0,	/* Rflush */
	10,	/* Twalk fid[4] newfid[4] nwname[2] */
	2,	/* Rwalk nwqid[2] */
	5,	/* Topen fid[4] mode[1] */
	17,	/* Ropen qid[13] iounit[4] */
	13,	/* Tcreate fid[4] name[s] perm[4] mode[1] */
	17,	/* Rcreate qid[13] iounit[4] */
	16,	/* Tread fid[4] offset[8] count[4] */
	4,	/* Rread count[4] */
	16,	/* Twrite fid[4] offset[8] count[4] */
	4,	/* Rwrite count[4] */
	4,	/* Tclunk fid[4] */
	0,	/* Rclunk */
	4,	/* Tremove fid[4] */
	0,	/* Rremove */
	4,	/* Tstat fid[4] */
	4,	/* Rstat stat[n] */
	8,	/* Twstat fid[4] stat[n] */
	0,	/* Rwstat */
};

func gint8(buf []byte) (uint8, []byte)
{
	return buf[0], buf[1:len(buf)];
}

func gint16(buf []byte) (uint16, []byte)
{
	return uint16(buf[0])|(uint16(buf[1])<<8), buf[2:len(buf)];
}

func gint32(buf []byte) (uint32, []byte)
{
	return uint32(buf[0])|(uint32(buf[1])<<8)|(uint32(buf[2])<<16)|
		(uint32(buf[3])<<24), buf[4:len(buf)];
}

func gint64(buf []byte) (uint64, []byte)
{
	return uint64(buf[0])|(uint64(buf[1])<<8)|(uint64(buf[2])<<16)|
		(uint64(buf[3])<<24)|(uint64(buf[4])<<32)|(uint64(buf[5])<<40)|
		(uint64(buf[6])<<48)|(uint64(buf[7])<<56), buf[8:len(buf)];
}

func gstr(buf []byte) (string, []byte)
{
	var n uint16;

	if buf==nil {
		return "", nil;
	}

	n, buf = gint16(buf);
	if int(n)>len(buf) {
		return "", nil;
	}

	return string(buf[0:n]), buf[n:len(buf)];
}

func gqid(buf []byte, qid *Qid) ([]byte)
{
	qid.qtype, buf = gint8(buf);
	qid.version, buf = gint32(buf);
	qid.path, buf = gint64(buf);

	return buf;
}

func gstat(buf []byte, st *Stat, dotu bool) ([]byte)
{
	st.size, buf = gint16(buf);
	st.stype, buf = gint16(buf);
	st.dev, buf = gint32(buf);
	buf = gqid(buf, &st.qid);
	st.mode, buf = gint32(buf);
	st.atime, buf = gint32(buf);
	st.mtime, buf = gint32(buf);
	st.length, buf = gint64(buf);
	st.name, buf = gstr(buf);
	if buf==nil {
		return nil;
	}

	st.uid, buf = gstr(buf);
	if buf==nil {
		return nil;
	}
	st.gid, buf = gstr(buf);
	if buf==nil {
		return nil;
	}

	st.muid, buf = gstr(buf);
	if buf==nil {
		return nil;
	}

	if dotu {
		st.ext, buf = gstr(buf);
		if buf==nil {
			return nil;
		}

		st.nuid, buf = gint32(buf);
		st.ngid, buf = gint32(buf);
		st.nmuid, buf = gint32(buf);
	} else {
		st.nuid = 0xFFFFFFFF;
		st.ngid = 0xFFFFFFFF;
		st.nmuid = 0xFFFFFFFF;
	}

	return buf;
}

func pint8(val uint8, buf []byte) []byte
{
	buf[0] = val;
	return buf[1:len(buf)];
}

func ppint8(val uint8, buf []byte, pval *uint8) []byte
{
	*pval = val;
	return pint8(val, buf);
}

func pint16(val uint16, buf []byte) []byte
{
	buf[0] = uint8(val);
	buf[1] = uint8(val>>8);
	return buf[2:len(buf)];
}

func ppint16(val uint16, buf []byte, pval *uint16) []byte
{
	*pval = val;
	return pint16(val, buf);
}

func pint32(val uint32, buf []byte) []byte
{
	buf[0] = uint8(val);
	buf[1] = uint8(val>>8);
	buf[2] = uint8(val>>16);
	buf[3] = uint8(val>>24);
	return buf[4:len(buf)];
}

func ppint32(val uint32, buf []byte, pval *uint32) []byte
{
	*pval = val;
	return pint32(val, buf);
}

func pint64(val uint64, buf []byte) []byte
{
	buf[0] = uint8(val);
	buf[1] = uint8(val>>8);
	buf[2] = uint8(val>>16);
	buf[3] = uint8(val>>24);
	buf[4] = uint8(val>>32);
	buf[5] = uint8(val>>40);
	buf[6] = uint8(val>>48);
	buf[7] = uint8(val>>58);
	return buf[8:len(buf)];
}

func ppint64(val uint64, buf []byte, pval *uint64) []byte
{
	*pval = val;
	return pint64(val, buf);
}

func pstr(val string, buf []byte) []byte
{
	n := uint16(len(val));
	buf = pint16(n, buf);
	return buf[n:len(buf)];
}

func ppstr(val string, buf []byte, pval *string) []byte
{
	*pval = val;
	return pstr(val, buf);
}

func pqid(val *Qid, buf []byte) []byte
{
	buf = pint8(val.qtype, buf);
	buf = pint32(val.version, buf);
	buf = pint64(val.path, buf);

	return buf;
}

func ppqid(val *Qid, buf []byte, pval *Qid) []byte
{
	*pval = *val;
	return pqid(val, buf);
}

func statsz(st *Stat, dotu bool) int
{
	sz := 2+2+4+13+4+4+4+8+2+2+2+2+len(st.name)+len(st.uid)+len(st.gid)+len(st.muid);
	if dotu {
		sz += 2+4+4+4+len(st.ext);
	}

	return sz;
}

func pstat(st *Stat, buf []byte, dotu bool) []byte
{
	buf = pint16(uint16(statsz(st, dotu)), buf);
	buf = pint16(st.stype, buf);
	buf = pint32(st.dev, buf);
	buf = pqid(&st.qid, buf);
	buf = pint32(st.mode, buf);
	buf = pint32(st.atime, buf);
	buf = pint32(st.mtime, buf);
	buf = pint64(st.length, buf);
	buf = pstr(st.name, buf);
	buf = pstr(st.uid, buf);
	buf = pstr(st.gid, buf);
	buf = pstr(st.muid, buf);
	if dotu {
		buf = pstr(st.ext, buf);
		buf = pint32(st.nuid, buf);
		buf = pint32(st.ngid, buf);
		buf = pint32(st.nmuid, buf);
	}

	return buf;
}

func ppstat(st *Stat, buf []byte, dotu bool, pval *Stat) []byte
{
	*pval = *st;
	return pstat(st, buf, dotu);
}

func PackStat(st *Stat, buf []byte, dotu bool) ([]byte, *Error)
{
	sz := statsz(st, dotu);
	if sz>len(buf) {
		return buf, &Error{"invalid size", syscall.EINVAL};
	}

	buf = pstat(st, buf, dotu);
	return buf, nil;
}

func UnpackStat(buf []byte, dotu bool) (st *Stat, err *Error, rest []byte)
{
	sz := 2 + 2 + 4 + 13 + 4 + /* size[2] type[2] dev[4] qid[13] mode[4] */
		4 + 4 + 8 +	   /* atime[4] mtime[4] length[8] */
		8;		   /* name[s] uid[s] gid[s] muid[s] */

	if dotu {
		sz += 4 + 4 + 4 + 2; /* n_uid[4] n_gid[4] n_muid[4] extension[s] */
	}

	if len(buf)<sz {
szerror:
		return nil, &Error{"invalid size", syscall.EINVAL}, buf;
	}

	st = new(Stat);
	buf = gstat(buf, st, dotu);
	if buf==nil {
		goto szerror;
	}

	return st, nil, buf;
}

func packCommon(fc *Call, size int, id uint8) ([]byte, *Error)
{
	size += 4+1+2; /* size[4] id[1] tag[2] */
	if len(fc.pkt)<int(size) {
		return nil, &Error{"buffer too small", syscall.EINVAL};
	}

	p := fc.pkt;
	p = ppint32(uint32(size), p, &fc.size);
	p = ppint8(id, p, &fc.id);
	p = ppint16(Notag, p, &fc.tag);

	return p, nil;
}

func PackTversion(fc *Call, msize uint32, version string) *Error
{
	size := 4 + 2 + len(version);	/* msize[4] version[s] */
	p, err := packCommon(fc, size, Tversion);
	if err!=nil {
		return err;
	}

	p = ppint32(msize, p, &fc.msize);
	p = ppstr(version, p, &fc.version);

	return nil;
}

func PackRversion(fc *Call, msize uint32, version string) *Error
{
	size := 4 + 2 + len(version);	/* msize[4] version[s] */
	p, err := packCommon(fc, size, Rversion);
	if err!=nil {
		return err;
	}

	p = ppint32(msize, p, &fc.msize);
	p = ppstr(version, p, &fc.version);

	return nil;
}

func PackTauth(fc *Call, fid uint32, uname string, aname string, nuname uint32, dotu bool) *Error
{
	size := 4+2+2+len(uname)+len(aname); /* fid[4] uname[s] aname[s] */
	if dotu {
		size += 4; /* n_uname[4] */
	}

	p, err := packCommon(fc, size, Tauth);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	p = ppstr(uname, p, &fc.uname);
	p = ppstr(aname, p, &fc.aname);
	if dotu {
		p = ppint32(nuname, p, &fc.nuname);
	}

	return nil;
}

func PackRauth(fc *Call, aqid *Qid) *Error {
	size := 13;	/* aqid[13] */
	p, err := packCommon(fc, size, Rauth);
	if err!=nil {
		return err;
	}

	p = ppqid(aqid, p, &fc.qid);
	return nil;
}

func PackRerror(fc *Call, error string, nerror uint32, dotu bool) *Error
{
	size := 2+len(error);	/* ename[s] */
	if dotu {
		size += 4;	/* ecode[4] */
	}

	p, err := packCommon(fc, size, Rerror);
	if err!=nil {
		return err;
	}

	p = ppstr(error, p, &fc.error);
	if dotu {
		p = ppint32(nerror, p, &fc.nerror);
	}

	return nil;
}

func PackTflush(fc *Call, oldtag uint16) *Error
{
	p, err := packCommon(fc, 2, Tflush);
	if err!=nil {
		return err;
	}

	p = ppint16(oldtag, p, &fc.oldtag);
	return nil;
}

func PackRflush(fc *Call) *Error
{
	_, err := packCommon(fc, 0, Rflush);

	return err;
}

func PackTattach(fc *Call, fid uint32, afid uint32, uname string, aname string, nuname uint32, dotu bool) *Error
{
	size := 4+4+2+len(uname)+2+len(aname); /* fid[4] afid[4] uname[s] aname[s] */
	if dotu {
		size += 4;
	}

	p, err := packCommon(fc, size, Tattach);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	p = ppint32(afid, p, &fc.afid);
	p = ppstr(uname, p, &fc.uname);
	p = ppstr(aname, p, &fc.aname);
	if dotu {
		p = ppint32(nuname, p, &fc.nuname);
	}

	return nil;
}

func PackRattach(fc *Call, aqid *Qid) *Error {
	size := 13;	/* aqid[13] */
	p, err := packCommon(fc, size, Rattach);
	if err!=nil {
		return err;
	}

	p = ppqid(aqid, p, &fc.qid);
	return nil;
}

func PackTwalk(fc *Call, fid uint32, newfid uint32, wnames []string) *Error
{
	nwname := len(wnames);
	size := 4+4+2+nwname*2; /* fid[4] newfid[4] nwname[2] nwname*wname[s] */
	for i:= 0; i<nwname; i++ {
		size += len(wnames[i]);
	}

	p, err := packCommon(fc, size, Twalk);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	p = ppint32(newfid, p, &fc.newfid);
	p = pint16(uint16(nwname), p);
	fc.wnames = make([]string, nwname);
	for i:=0; i<nwname; i++ {
		p = ppstr(wnames[i], p, &fc.wnames[i]);
	}

	return nil;
}

func PackRwalk(fc *Call, wqids []Qid) *Error
{
	nwqid := len(wqids);
	size := 2+nwqid*13; /* nwqid[2] nwname*wqid[13] */
	p, err := packCommon(fc, size, Rwalk);
	if err!=nil {
		return err;
	}

	p = pint16(uint16(nwqid), p);
	fc.wqids = make([]Qid, nwqid);
	for i:=0; i<nwqid; i++ {
		p = ppqid(&wqids[i], p, &fc.wqids[i]);
	}

	return nil;
}

func PackTopen(fc *Call, fid uint32, mode uint8) *Error
{
	size := 4+1;	/* fid[4] mode[1] */
	p, err := packCommon(fc, size, Topen);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	p = ppint8(mode, p, &fc.mode);
	return nil;
}

func PackRopen(fc *Call, qid *Qid, iounit uint32) *Error
{
	size := 13+4;	/* qid[13] iounit[4] */
	p, err := packCommon(fc, size, Ropen);
	if err!=nil {
		return err;
	}

	p = ppqid(qid, p, &fc.qid);
	p = ppint32(iounit, p, &fc.iounit);
	return nil;
}

func PackTcreate(fc *Call, fid uint32, name string, perm uint32, mode uint8, ext string, dotu bool) *Error
{
	size := 4+2+len(name)+4+1;	/* fid[4] name[s] perm[4] mode[1] */

	if dotu {
		size += 2+len(ext);
	}

	p, err := packCommon(fc, size, Tcreate);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	p = ppstr(name, p, &fc.name);
	p = ppint32(perm, p, &fc.perm);
	p = ppint8(mode, p, &fc.mode);

	if dotu {
		p = ppstr(ext, p, &fc.ext);
	}

	return nil;
}

func PackRcreate(fc *Call, qid *Qid, iounit uint32) *Error
{
	size := 13+4;	/* qid[13] iounit[4] */
	p, err := packCommon(fc, size, Rcreate);
	if err!=nil {
		return err;
	}

	p = ppqid(qid, p, &fc.qid);
	p = ppint32(iounit, p, &fc.iounit);
	return nil;
}

func PackTread(fc *Call, fid uint32, offset uint64, count uint32) *Error
{
	size := 4+8+4; /* fid[4] offset[8] count[4] */
	p, err := packCommon(fc, size, Tread);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	p = ppint64(offset, p, &fc.offset);
	p = ppint32(count, p, &fc.count);
	return nil;
}

func InitRread(fc *Call, count uint32) *Error
{
	size := int(4+count); /* count[4] data[count] */
	p, err := packCommon(fc, size, Rread);
	if err!=nil {
		return err;
	}

	p = ppint32(count, p, &fc.count);
	fc.data = p;
	return nil;
}

func SetRreadCount(fc *Call, count uint32)
{
	/* we need to update both the packet size as well as the data count */
	size := 4+1+2+4+count;	/* size[4] id[1] tag[2] count[4] data[count] */
	ppint32(size, fc.pkt, &fc.size);
	ppint32(count, fc.pkt[7:11], &fc.count);
}

func PackRread(fc *Call, data []byte) *Error
{
	count := uint32(len(data));
	err := InitRread(fc, count);
	if err!=nil {
		return err;
	}

	bytes.Copy(fc.data, data);
	return nil;
}

func PackTwrite(fc *Call, fid uint32, offset uint64, data []byte) *Error
{
	count := len(data);
	size := 4+8+4+count;	/* fid[4] offset[8] count[4] data[count] */
	p, err := packCommon(fc, size, Twrite);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	p = ppint64(offset, p, &fc.offset);
	p = ppint32(uint32(count), p, &fc.count);
	fc.data = p;
	bytes.Copy(fc.data, data);
	return nil;
}

func PackRwrite(fc *Call, count uint32) *Error
{
	p, err := packCommon(fc, 4, Rwrite);	/* count[4] */
	if err!=nil {
		return err;
	}

	p = ppint32(count, p, &fc.count);
	return nil;
}

func PackTclunk(fc *Call, fid uint32) *Error
{
	p, err := packCommon(fc, 4, Tclunk);	/* fid[4] */
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	return nil;
}

func PackRclunk(fc *Call) *Error
{
	_, err := packCommon(fc, 0, Rclunk);
	return err;
}

func PackTremove(fc *Call, fid uint32) *Error
{
	p, err := packCommon(fc, 4, Tremove);	/* fid[4] */
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	return nil;
}

func PackRremove(fc *Call) *Error
{
	_, err := packCommon(fc, 0, Rremove);
	return err;
}

func PackTstat(fc *Call, fid uint32) *Error
{
	p, err := packCommon(fc, 4, Tstat);	/* fid[4] */
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	return nil;
}

func PackRstat(fc *Call, st *Stat, dotu bool) *Error
{
	stsz := statsz(st, dotu);
	size := 2+stsz;	/* stat[n] */
	p, err := packCommon(fc, size, Rstat);
	if err!=nil {
		return err;
	}

	p = pint16(uint16(stsz), p);
	p = ppstat(st, p, dotu, &fc.stat);
	return nil;
}

func PackTwstat(fc *Call, fid uint32, st *Stat, dotu bool) *Error
{
	stsz := statsz(st, dotu);
	size := 4+2+stsz;	/* fid[4] stat[n] */
	p, err := packCommon(fc, size, Twstat);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.fid);
	p = ppstat(st, p, dotu, &fc.stat);
	return nil;
}

func PackRwstat(fc *Call) *Error
{
	_, err := packCommon(fc, 0, Rwstat);
	return err;
}

func Unpack(buf []byte, dotu bool) (fc *Call, err *Error, rest []byte)
{
	var m uint16;

	fc.fid = Nofid;
	fc.afid = Nofid;
	fc.newfid = Nofid;

	p := buf;
	fc.size, p = gint32(p);
	fc.id, p = gint8(p);
	fc.tag, p = gint16(p);

	p = p[0:fc.size-7];
	fc.pkt = buf[0:fc.size];
	rest = buf[fc.size:len(buf)];
	if fc.id<Tfirst || fc.id>=Tlast {
		return nil, &Error{"invalid id", syscall.EINVAL}, buf;
	}

	var sz uint32;
	if dotu {
		sz = minFcsize[fc.id - Tfirst];
	} else {
		sz = minFcusize[fc.id - Tfirst];
	}

	if fc.size<sz {
szerror:
		return nil, &Error{"invalid size", syscall.EINVAL}, buf;
	}

	err = nil;
	switch fc.id {
	default:
		return nil, &Error{"invalid message id", syscall.EINVAL}, buf;

	case Tversion, Rversion:
		fc.msize, p = gint32(p);
		fc.version, p = gstr(p);
		if p==nil {
			goto szerror;
		}

	case Tauth:
		fc.afid, p = gint32(p);
		fc.uname, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		fc.aname, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		if dotu {
			fc.nuname, p = gint32(p);
		} else {
			fc.nuname = Nouid;
		}

	case Rauth, Rattach:
		p = gqid(p, &fc.qid);

	case Tflush:
		fc.oldtag, p = gint16(p);

	case Tattach:
		fc.fid, p = gint32(p);
		fc.afid, p = gint32(p);
		fc.uname, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		fc.aname, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		if dotu {
			fc.nuname, p = gint32(p);
		}

	case Rerror:
		fc.error, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		if dotu {
			fc.nerror, p = gint32(p);
		} else {
			fc.nerror = 0;
		}

	case Twalk:
		fc.fid, p = gint32(p);
		fc.newfid, p = gint32(p);
		m, p = gint16(p);
		fc.wnames = make([]string, m);
		for i:=0; i<int(m); i++ {
			fc.wnames[i], p = gstr(p);
			if p==nil {
				goto szerror;
			}
		}

	case Rwalk:
		m, p = gint16(p);
		fc.wqids = make([]Qid, m);
		for i:=0; i<int(m); i++ {
			p = gqid(p, &fc.wqids[i]);
		}

	case Topen:
		fc.fid, p = gint32(p);
		fc.mode, p = gint8(p);

	case Ropen, Rcreate:
		p = gqid(p, &fc.qid);
		fc.iounit, p = gint32(p);

	case Tcreate:
		fc.fid, p = gint32(p);
		fc.name, p = gstr(p);
		if p==nil {
			goto szerror;
		}
		fc.perm, p = gint32(p);
		fc.mode, p = gint8(p);
		if dotu {
			fc.ext, p = gstr(p);
			if p==nil {
				goto szerror;
			}
		}

	case Tread:
		fc.fid, p = gint32(p);
		fc.offset, p = gint64(p);
		fc.count, p = gint32(p);

	case Rread:
		fc.count, p = gint32(p);
		if len(p)<int(fc.count) {
			goto szerror;
		}

	case Twrite:
		fc.fid, p = gint32(p);
		fc.offset, p = gint64(p);
		fc.count, p = gint32(p);
		if len(p)<int(fc.count) {
			goto szerror;
		}

	case Rwrite:
		fc.count, p = gint32(p);

	case Tclunk, Tremove, Tstat:
		fc.fid, p = gint32(p);

	case Rstat:
		m, p = gint16(p);
		p = gstat(p, &fc.stat, dotu);
		if p==nil {
			goto szerror;
		}

	case Twstat:
		fc.fid, p = gint32(p);
		m, p = gint16(p);
		p = gstat(p, &fc.stat, dotu);

	case Rflush, Rclunk, Rremove, Rwstat:
	}

	if len(p)>0 {
		goto szerror;
	}

	return;
}
