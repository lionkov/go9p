package p9

import "os"
import "syscall"
import "bytes"
import "fmt"
import "strings"
import "sync"
//import "log" //debugging

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
	MaxMsgSz = 128*1024;	// should probably equal msize, for now leave it a bit larger;
							// it's used to filter non-9p clients as any ascii string will overflow
							// the message size buffer (first four bytes)
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
	DMLINK	    =0x01000000; // mode bit for hard link (Unix, 9P2000.u)
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
	MSize uint32	= 8192+IOHdrSz;
);

type Error struct {
	Error	string;
	Nerror	os.Errno;
}

type Qid struct {
	Type	uint8;
	Version	uint32;
	Path	uint64;
}

//TODO: string implementations for debugging
func (s Stat) String() string {
	return "";
}

type Stat struct {
	size	uint16;
	Type	uint16;
	Dev	uint32;
	Sqid	Qid;
	Mode	uint32;
	Atime	uint32;
	Mtime	uint32;
	Length	uint64;
	Name	string;
	Uid	string;
	Gid	string;
	Muid	string;

	/* 9P2000.u extension */
	Ext	string;
	Nuid	uint32;
	Ngid	uint32;
	Nmuid	uint32;
};

type Fcall struct {
	size	uint32;
	Id	uint8;
	Tag	uint16;

	Fid	uint32;
	Msize	uint32;			/* Tversion, Rversion */
	Version	string;			/* Tversion, Rversion */
	Afid	uint32;			/* Tauth, Tattach */
	Uname	string;			/* Tauth, Tattach */
	Aname	string;			/* Tauth, Tattach */
	Fqid	Qid;			/* Rauth, Rattach, Ropen, Rcreate */
	Error	string;			/* Rerror */
	Oldtag	uint16;			/* Tflush */
	Newfid	uint32;			/* Twalk */
	Wnames	[]string;		/* Twalk */
	Wqids	[]Qid;			/* Rwalk */
	Mode	uint8;			/* Topen, Tcreate */
	Iounit	uint32;			/* Ropen, Rcreate */
	Name	string;			/* Tcreate */
	Perm	uint32;			/* Tcreate */
	Offset	uint64;			/* Tread, Twrite */
	Count	uint32;			/* Tread, Rread, Twrite, Rwrite */
	Fstat	Stat;			/* Rstat, Twstat */
	Data	[]uint8;		/* Rread, Twrite */

	/* 9P2000.u extensions */
	Nerror	uint32;			/* Rerror */
	Ext	string;			/* Tcreate */
	Nuname	uint32;			/* Tauth, Tattach */

	Pkt	[]uint8;		/* raw packet data */
}

type Users interface {
	Uid2User(uid int) User;
	Uname2User(uname string) User;
	Gid2Group(gid int) Group;
	Gname2Group(gname string) Group;
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

func (fc *Fcall) String() string {
	ret := "";
	fid := fmt.Sprintf("%X", fc.Fid);
	switch fc.Id {
	case Tversion:
		ret = "-"+fid+"-> Tversion: " + fc.Version + " msize: " + fmt.Sprint(fc.Msize)
	case Rversion:
		ret = "<-"+fid+"- Rversion: " + fc.Version + " msize: " + fmt.Sprint(fc.Msize)
	case Tauth:
		ret += "--> Tauth"
	case Rauth:
		ret += "<-- Rauth"
	case Rattach:
		ret += "<-- Rattach"
	case Tattach:
		ret += "--> Tattach"
	case Tflush:
		ret += "--> Tflush"
	case Rerror:
		ret += "<-- Rerror"
	case Twalk:
		ret += "--> Twalk"
	case Rwalk:
		ret += "<-- Rwalk"
	case Topen:
		ret += "--> Topen"
	case Ropen:
		ret += "<-- Ropen"
	case Rcreate:
		ret += "<-- Rcreate"
	case Tcreate:
		ret += "--> Tcreate"
	case Tread:
		ret += "--> Tread"
	case Rread:
		ret += "<-- Rread"
	case Twrite:
		ret += "--> Twrite"
	case Rwrite:
		ret += "<-- Rwrite"
	case Tclunk:
		ret += "--> Tclunk"
	case Rclunk:
		ret += "<-- Rclunk"
	case Tremove:
		ret += "--> Tremove"
	case Tstat:
		ret += "--> Tstat"
	case Rstat:
		ret += "<-- Rstat"
	case Twstat:
		ret += "--> Twstat"
	case Rflush:
		ret += "<-- Rflush"
	case Rremove:
		ret += "<-- Rremove"
	case Rwstat:
		ret += "<-- Rwstat"
	}

	return ret;
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

func Gint32(buf []byte) (uint32, []byte)
{
	return gint32(buf);
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
	qid.Type, buf = gint8(buf);
	qid.Version, buf = gint32(buf);
	qid.Path, buf = gint64(buf);

	return buf;
}

func gstat(buf []byte, st *Stat, dotu bool) ([]byte)
{
	st.size, buf = gint16(buf);
	st.Type, buf = gint16(buf);
	st.Dev, buf = gint32(buf);
	buf = gqid(buf, &st.Sqid);
	st.Mode, buf = gint32(buf);
	st.Atime, buf = gint32(buf);
	st.Mtime, buf = gint32(buf);
	st.Length, buf = gint64(buf);
	st.Name, buf = gstr(buf);
	if buf==nil {
		return nil;
	}

	st.Uid, buf = gstr(buf);
	if buf==nil {
		return nil;
	}
	st.Gid, buf = gstr(buf);
	if buf==nil {
		return nil;
	}

	st.Muid, buf = gstr(buf);
	if buf==nil {
		return nil;
	}

	if dotu {
		st.Ext, buf = gstr(buf);
		if buf==nil {
			return nil;
		}

		st.Nuid, buf = gint32(buf);
		st.Ngid, buf = gint32(buf);
		st.Nmuid, buf = gint32(buf);
	} else {
		st.Nuid = Nouid;
		st.Ngid = Nouid;
		st.Nmuid = Nouid;
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
	bytes := strings.Bytes(val);
	for i := 0; i < len(bytes); i++ {
		buf = pint8(bytes[i], buf);
	}
	return buf[n:len(buf)];
}

func ppstr(val string, buf []byte, pval *string) []byte
{
	*pval = val;
	return pstr(val, buf);
}

func pqid(val *Qid, buf []byte) []byte
{
	buf = pint8(val.Type, buf);
	buf = pint32(val.Version, buf);
	buf = pint64(val.Path, buf);

	return buf;
}

func ppqid(val *Qid, buf []byte, pval *Qid) []byte
{
	*pval = *val;
	return pqid(val, buf);
}

func statsz(st *Stat, dotu bool) int
{
	sz := 2+2+4+13+4+4+4+8+2+2+2+2+len(st.Name)+len(st.Uid)+len(st.Gid)+len(st.Muid);
	if dotu {
		sz += 2+4+4+4+len(st.Ext);
	}

	return sz;
}

func pstat(st *Stat, buf []byte, dotu bool) []byte
{
	buf = pint16(uint16(statsz(st, dotu)), buf);
	buf = pint16(st.Type, buf);
	buf = pint32(st.Dev, buf);
	buf = pqid(&st.Sqid, buf);
	buf = pint32(st.Mode, buf);
	buf = pint32(st.Atime, buf);
	buf = pint32(st.Mtime, buf);
	buf = pint64(st.Length, buf);
	buf = pstr(st.Name, buf);
	buf = pstr(st.Uid, buf);
	buf = pstr(st.Gid, buf);
	buf = pstr(st.Muid, buf);
	if dotu {
		buf = pstr(st.Ext, buf);
		buf = pint32(st.Nuid, buf);
		buf = pint32(st.Ngid, buf);
		buf = pint32(st.Nmuid, buf);
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

func NewFcall(sz uint32) *Fcall
{
	fc := new(Fcall);
	fc.Pkt = make([]byte, sz);

	return fc;
}

func SetTag(fc *Fcall, tag uint16)
{
	ppint16(tag, fc.Pkt[6:len(fc.Pkt)], &fc.Tag);
}

func packCommon(fc *Fcall, size int, id uint8) ([]byte, *Error)
{
	size += 4+1+2; /* size[4] id[1] tag[2] */
	if len(fc.Pkt)<int(size) {
		return nil, &Error{"buffer too small", syscall.EINVAL};
	}

	p := fc.Pkt;
	p = ppint32(uint32(size), p, &fc.size);
	p = ppint8(id, p, &fc.Id);
	p = ppint16(Notag, p, &fc.Tag);

	return p, nil;
}

func PackTversion(fc *Fcall, msize uint32, version string) *Error
{
	size := 4 + 2 + len(version);	/* msize[4] version[s] */
	p, err := packCommon(fc, size, Tversion);
	if err!=nil {
		return err;
	}

	p = ppint32(msize, p, &fc.Msize);
	p = ppstr(version, p, &fc.Version);

	return nil;
}

func PackRversion(fc *Fcall, msize uint32, version string) *Error
{
	size := 4 + 2 + len(version);	/* msize[4] version[s] */
	p, err := packCommon(fc, size, Rversion);
	if err!=nil {
		return err;
	}

	p = ppint32(msize, p, &fc.Msize);
	p = ppstr(version, p, &fc.Version);

	return nil;
}

func PackTauth(fc *Fcall, fid uint32, uname string, aname string, nuname uint32, dotu bool) *Error
{
	size := 4+2+2+len(uname)+len(aname); /* fid[4] uname[s] aname[s] */
	if dotu {
		size += 4; /* n_uname[4] */
	}

	p, err := packCommon(fc, size, Tauth);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppstr(uname, p, &fc.Uname);
	p = ppstr(aname, p, &fc.Aname);
	if dotu {
		p = ppint32(nuname, p, &fc.Nuname);
	}

	return nil;
}

func PackRauth(fc *Fcall, aqid *Qid) *Error {
	size := 13;	/* aqid[13] */
	p, err := packCommon(fc, size, Rauth);
	if err!=nil {
		return err;
	}

	p = ppqid(aqid, p, &fc.Fqid);
	return nil;
}

func PackRerror(fc *Fcall, error string, nerror uint32, dotu bool) *Error
{
	size := 2+len(error);	/* ename[s] */
	if dotu {
		size += 4;	/* ecode[4] */
	}

	p, err := packCommon(fc, size, Rerror);
	if err!=nil {
		return err;
	}

	p = ppstr(error, p, &fc.Error);
	if dotu {
		p = ppint32(nerror, p, &fc.Nerror);
	}

	return nil;
}

func PackTflush(fc *Fcall, oldtag uint16) *Error
{
	p, err := packCommon(fc, 2, Tflush);
	if err!=nil {
		return err;
	}

	p = ppint16(oldtag, p, &fc.Oldtag);
	return nil;
}

func PackRflush(fc *Fcall) *Error
{
	_, err := packCommon(fc, 0, Rflush);

	return err;
}

func PackTattach(fc *Fcall, fid uint32, afid uint32, uname string, aname string, nuname uint32, dotu bool) *Error
{
	size := 4+4+2+len(uname)+2+len(aname); /* fid[4] afid[4] uname[s] aname[s] */
	if dotu {
		size += 4;
	}

	p, err := packCommon(fc, size, Tattach);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint32(afid, p, &fc.Afid);
	p = ppstr(uname, p, &fc.Uname);
	p = ppstr(aname, p, &fc.Aname);
	if dotu {
		p = ppint32(nuname, p, &fc.Nuname);
	}

	return nil;
}

func PackRattach(fc *Fcall, aqid *Qid) *Error {
	size := 13;	/* aqid[13] */
	p, err := packCommon(fc, size, Rattach);
	if err!=nil {
		return err;
	}

	p = ppqid(aqid, p, &fc.Fqid);
	return nil;
}

func PackTwalk(fc *Fcall, fid uint32, newfid uint32, wnames []string) *Error
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

	p = ppint32(fid, p, &fc.Fid);
	p = ppint32(newfid, p, &fc.Newfid);
	p = pint16(uint16(nwname), p);
	fc.Wnames = make([]string, nwname);
	for i:=0; i<nwname; i++ {
		p = ppstr(wnames[i], p, &fc.Wnames[i]);
	}

	return nil;
}

func PackRwalk(fc *Fcall, wqids []Qid) *Error
{
	nwqid := len(wqids);
	size := 2+nwqid*13; /* nwqid[2] nwname*wqid[13] */
	p, err := packCommon(fc, size, Rwalk);
	if err!=nil {
		return err;
	}

	p = pint16(uint16(nwqid), p);
	fc.Wqids = make([]Qid, nwqid);
	for i:=0; i<nwqid; i++ {
		p = ppqid(&wqids[i], p, &fc.Wqids[i]);
	}

	return nil;
}

func PackTopen(fc *Fcall, fid uint32, mode uint8) *Error
{
	size := 4+1;	/* fid[4] mode[1] */
	p, err := packCommon(fc, size, Topen);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint8(mode, p, &fc.Mode);
	return nil;
}

func PackRopen(fc *Fcall, qid *Qid, iounit uint32) *Error
{
	size := 13+4;	/* qid[13] iounit[4] */
	p, err := packCommon(fc, size, Ropen);
	if err!=nil {
		return err;
	}

	p = ppqid(qid, p, &fc.Fqid);
	p = ppint32(iounit, p, &fc.Iounit);
	return nil;
}

func PackTcreate(fc *Fcall, fid uint32, name string, perm uint32, mode uint8, ext string, dotu bool) *Error
{
	size := 4+2+len(name)+4+1;	/* fid[4] name[s] perm[4] mode[1] */

	if dotu {
		size += 2+len(ext);
	}

	p, err := packCommon(fc, size, Tcreate);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppstr(name, p, &fc.Name);
	p = ppint32(perm, p, &fc.Perm);
	p = ppint8(mode, p, &fc.Mode);

	if dotu {
		p = ppstr(ext, p, &fc.Ext);
	}

	return nil;
}

func PackRcreate(fc *Fcall, qid *Qid, iounit uint32) *Error
{
	size := 13+4;	/* qid[13] iounit[4] */
	p, err := packCommon(fc, size, Rcreate);
	if err!=nil {
		return err;
	}

	p = ppqid(qid, p, &fc.Fqid);
	p = ppint32(iounit, p, &fc.Iounit);
	return nil;
}

func PackTread(fc *Fcall, fid uint32, offset uint64, count uint32) *Error
{
	size := 4+8+4; /* fid[4] offset[8] count[4] */
	p, err := packCommon(fc, size, Tread);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint64(offset, p, &fc.Offset);
	p = ppint32(count, p, &fc.Count);
	return nil;
}

func InitRread(fc *Fcall, count uint32) *Error
{
	size := int(4+count); /* count[4] data[count] */
	p, err := packCommon(fc, size, Rread);
	if err!=nil {
		return err;
	}

	p = ppint32(count, p, &fc.Count);
	fc.Data = p;
	return nil;
}

func SetRreadCount(fc *Fcall, count uint32)
{
	/* we need to update both the packet size as well as the data count */
	size := 4+1+2+4+count;	/* size[4] id[1] tag[2] count[4] data[count] */
	ppint32(size, fc.Pkt, &fc.size);
	ppint32(count, fc.Pkt[7:11], &fc.Count);
}

func PackRread(fc *Fcall, data []byte) *Error
{
	count := uint32(len(data));
	err := InitRread(fc, count);
	if err!=nil {
		return err;
	}

	bytes.Copy(fc.Data, data);
	return nil;
}

func PackTwrite(fc *Fcall, fid uint32, offset uint64, data []byte) *Error
{
	count := len(data);
	size := 4+8+4+count;	/* fid[4] offset[8] count[4] data[count] */
	p, err := packCommon(fc, size, Twrite);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint64(offset, p, &fc.Offset);
	p = ppint32(uint32(count), p, &fc.Count);
	fc.Data = p;
	bytes.Copy(fc.Data, data);
	return nil;
}

func PackRwrite(fc *Fcall, count uint32) *Error
{
	p, err := packCommon(fc, 4, Rwrite);	/* count[4] */
	if err!=nil {
		return err;
	}

	p = ppint32(count, p, &fc.Count);
	return nil;
}

func PackTclunk(fc *Fcall, fid uint32) *Error
{
	p, err := packCommon(fc, 4, Tclunk);	/* fid[4] */
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	return nil;
}

func PackRclunk(fc *Fcall) *Error
{
	_, err := packCommon(fc, 0, Rclunk);
	return err;
}

func PackTremove(fc *Fcall, fid uint32) *Error
{
	p, err := packCommon(fc, 4, Tremove);	/* fid[4] */
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	return nil;
}

func PackRremove(fc *Fcall) *Error
{
	_, err := packCommon(fc, 0, Rremove);
	return err;
}

func PackTstat(fc *Fcall, fid uint32) *Error
{
	p, err := packCommon(fc, 4, Tstat);	/* fid[4] */
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	return nil;
}

func PackRstat(fc *Fcall, st *Stat, dotu bool) *Error
{
	stsz := statsz(st, dotu);
	size := 2+stsz;	/* stat[n] */
	p, err := packCommon(fc, size, Rstat);
	if err!=nil {
		return err;
	}

	p = pint16(uint16(stsz), p);
	p = ppstat(st, p, dotu, &fc.Fstat);
	return nil;
}

func PackTwstat(fc *Fcall, fid uint32, st *Stat, dotu bool) *Error
{
	stsz := statsz(st, dotu);
	size := 4+2+stsz;	/* fid[4] stat[n] */
	p, err := packCommon(fc, size, Twstat);
	if err!=nil {
		return err;
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppstat(st, p, dotu, &fc.Fstat);
	return nil;
}

func PackRwstat(fc *Fcall) *Error
{
	_, err := packCommon(fc, 0, Rwstat);
	return err;
}

func Unpack(buf []byte, dotu bool) (fc *Fcall, err *Error, fcsz int)
{
	var m uint16;

	fc = new(Fcall);
	fc.Fid = Nofid;
	fc.Afid = Nofid;
	fc.Newfid = Nofid;

	p := buf;
	fc.size, p = gint32(p);
	fc.Id, p = gint8(p);
	fc.Tag, p = gint16(p);

	p = p[0:fc.size-7];
	fc.Pkt = buf[0:fc.size];
	fcsz = int(fc.size);
	if fc.Id<Tfirst || fc.Id>=Tlast {
		return nil, &Error{"invalid id", syscall.EINVAL}, 0;
	}

	var sz uint32;
	if dotu {
		sz = minFcsize[fc.Id - Tfirst];
	} else {
		sz = minFcusize[fc.Id - Tfirst];
	}

	if fc.size<sz {
szerror:
		return nil, &Error{"invalid size", syscall.EINVAL}, 0;
	}

	err = nil;
	switch fc.Id {
	default:
		return nil, &Error{"invalid message id", syscall.EINVAL}, 0;

	case Tversion, Rversion:
		fc.Msize, p = gint32(p);
		fc.Version, p = gstr(p);
		if p==nil {
			goto szerror;
		}

	case Tauth:
		fc.Afid, p = gint32(p);
		fc.Uname, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		fc.Aname, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		if dotu {
			fc.Nuname, p = gint32(p);
		} else {
			fc.Nuname = Nouid;
		}

	case Rauth, Rattach:
		p = gqid(p, &fc.Fqid);

	case Tflush:
		fc.Oldtag, p = gint16(p);

	case Tattach:
		fc.Fid, p = gint32(p);
		fc.Afid, p = gint32(p);
		fc.Uname, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		fc.Aname, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		if dotu {
			fc.Nuname, p = gint32(p);
		}

	case Rerror:
		fc.Error, p = gstr(p);
		if p==nil {
			goto szerror;
		}

		if dotu {
			fc.Nerror, p = gint32(p);
		} else {
			fc.Nerror = 0;
		}

	case Twalk:
		fc.Fid, p = gint32(p);
		fc.Newfid, p = gint32(p);
		m, p = gint16(p);
		fc.Wnames = make([]string, m);
		for i:=0; i<int(m); i++ {
			fc.Wnames[i], p = gstr(p);
			if p==nil {
				goto szerror;
			}
		}

	case Rwalk:
		m, p = gint16(p);
		fc.Wqids = make([]Qid, m);
		for i:=0; i<int(m); i++ {
			p = gqid(p, &fc.Wqids[i]);
		}

	case Topen:
		fc.Fid, p = gint32(p);
		fc.Mode, p = gint8(p);

	case Ropen, Rcreate:
		p = gqid(p, &fc.Fqid);
		fc.Iounit, p = gint32(p);

	case Tcreate:
		fc.Fid, p = gint32(p);
		fc.Name, p = gstr(p);
		if p==nil {
			goto szerror;
		}
		fc.Perm, p = gint32(p);
		fc.Mode, p = gint8(p);
		if dotu {
			fc.Ext, p = gstr(p);
			if p==nil {
				goto szerror;
			}
		}

	case Tread:
		fc.Fid, p = gint32(p);
		fc.Offset, p = gint64(p);
		fc.Count, p = gint32(p);

	case Rread:
		fc.Count, p = gint32(p);
		if len(p)<int(fc.Count) {
			goto szerror;
		}

	case Twrite:
		fc.Fid, p = gint32(p);
		fc.Offset, p = gint64(p);
		fc.Count, p = gint32(p);
		if len(p)<int(fc.Count) {
			goto szerror;
		}

	case Rwrite:
		fc.Count, p = gint32(p);

	case Tclunk, Tremove, Tstat:
		fc.Fid, p = gint32(p);

	case Rstat:
		m, p = gint16(p);
		p = gstat(p, &fc.Fstat, dotu);
		if p==nil {
			goto szerror;
		}

	case Twstat:
		fc.Fid, p = gint32(p);
		m, p = gint16(p);
		p = gstat(p, &fc.Fstat, dotu);

	case Rflush, Rclunk, Rremove, Rwstat:
	}

	if len(p)>0 {
		goto szerror;
	}

	return;
}

type OsUser struct {
	uid	int;
	uname	string;
}

func (u *OsUser) Name() string
{
	return u.uname;
}

func (u *OsUser) Id() int
{
	return u.uid;
}

func (u *OsUser) Groups() []*Group
{
	return nil;
}

type OsGroup struct {
	gid	int;
	name	string;
}

func (g *OsGroup) Name() string
{
	return g.name;
}

func (g *OsGroup) Id() int
{
	return g.gid;
}

func (g *OsGroup) Members() []*User
{
	return nil;
}

type osUsers struct {
	users map[int] *OsUser;
	groups map[int] *OsGroup;
	sync.Mutex;
};

var OsUsers *osUsers;

func (up *osUsers) Uid2User(uid int) User
{
	OsUsers.Lock();
	user, present := OsUsers.users[uid];
	if present {
		OsUsers.Unlock();
		return user;
	}

	user = new(OsUser);
	user.uid = uid;
	OsUsers.users[uid] = user;
	OsUsers.Unlock();
	return user;
}

func (up *osUsers) Uname2User(uname string) User
{
	return nil;
}

func (up *osUsers) Gid2Group(gid int) Group
{
	OsUsers.Lock();
	group, present := OsUsers.groups[gid];
	if present {
		OsUsers.Unlock();
		return group;
	}

	group = new(OsGroup);
	group.gid = gid;
	OsUsers.groups[gid] = group;
	OsUsers.Unlock();
	return group;
}

func (up *osUsers) Gname2Group(gname string) Group
{
	return nil;
}

func init()
{
	OsUsers = new(osUsers);
	OsUsers.users = make(map[int] *OsUser);
	OsUsers.groups = make(map[int] *OsGroup);
}

