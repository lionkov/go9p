// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The p9 package provides the definitions and functions used to implement
// the 9P2000 protocol.
package p

import "os"
import "syscall"
//import "log"
import "strings"

// 9P2000 message types
const (
	Tfirst, Tversion	uint8	= 100 + iota, 100 + iota;
	Rversion			= 100 + iota;
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
)

const (
	MSize	= 8216;	// default message size (8192+IOHdrSz)
	IOHdrSz	= 24;	// the non-data size of the Twrite messages
	Port	= 564;	// default port for 9P file servers
)

// Qid types
const (
	QTFILE		= 1 << iota;	// regular file
	QTSYMLINK;	// symlink (9P2000.u)
	QTTMP;		// non-backed-up file
	QTAUTH;		// authentication file
	QTMOUNT;	// mounted channel
	QTEXCL;		// exclusive use file
	QTAPPEND;	// append-only file
	QTDIR;		// directory
)

// Flags for the mode field in Topen and Tcreate messages
const (
	OREAD	= iota;	// open read-only
	OWRITE;	// open write-only
	ORDWR;	// open read-write
	OEXEC;	// execute (== read but check execute permission)
	OTRUNC	= 16;	// or'ed in (except for exec), truncate file first
	OCEXEC	= 32;	// or'ed in, close on exec
	ORCLOSE	= 64;	// or'ed in, remove on close
)

// File modes
const (
	DMDIR		= 0x80000000;	// mode bit for directories
	DMAPPEND	= 0x40000000;	// mode bit for append only files
	DMEXCL		= 0x20000000;	// mode bit for exclusive use files
	DMMOUNT		= 0x10000000;	// mode bit for mounted channel
	DMAUTH		= 0x08000000;	// mode bit for authentication file
	DMTMP		= 0x04000000;	// mode bit for non-backed-up file
	DMSYMLINK	= 0x02000000;	// mode bit for symbolic link (Unix, 9P2000.u)
	DMLINK		= 0x01000000;	// mode bit for hard link (Unix, 9P2000.u)
	DMDEVICE	= 0x00800000;	// mode bit for device file (Unix, 9P2000.u)
	DMNAMEDPIPE	= 0x00200000;	// mode bit for named pipe (Unix, 9P2000.u)
	DMSOCKET	= 0x00100000;	// mode bit for socket (Unix, 9P2000.u)
	DMSETUID	= 0x00080000;	// mode bit for setuid (Unix, 9P2000.u)
	DMSETGID	= 0x00040000;	// mode bit for setgid (Unix, 9P2000.u)
	DMREAD		= 0x4;		// mode bit for read permission
	DMWRITE		= 0x2;		// mode bit for write permission
	DMEXEC		= 0x1;		// mode bit for execute permission
)

const (
	Notag	uint16	= 0xFFFF;	// no tag specified
	Nofid	uint32	= 0xFFFFFFFF;	// no fid specified
	Nouid	uint32	= 0xFFFFFFFF;	// no uid specified
)

// Error represents a 9P2000 (and 9P2000.u) error
type Error struct {
	Error	string;		// textual representation of the error
	Nerror	os.Errno;	// numeric representation of the error (9P2000.u)
}

// File identifier
type Qid struct {
	Type	uint8;	// type of the file (high 8 bits of the mode)
	Version	uint32;	// version number for the path
	Path	uint64;	// server's unique identification of the file
}

// Stat describes a file
type Stat struct {
	Size	uint16;	// size-2 of the Stat on the wire
	Type	uint16;
	Dev	uint32;
	Sqid	Qid;	// file's Qid
	Mode	uint32;	// permissions and flags
	Atime	uint32;	// last access time in seconds
	Mtime	uint32;	// last modified time in seconds
	Length	uint64;	// file length in bytes
	Name	string;	// file name
	Uid	string;	// owner name
	Gid	string;	// group name
	Muid	string;	// name of the last user that modified the file

	/* 9P2000.u extension */
	Ext	string;	// special file's descriptor
	Nuid	uint32;	// owner ID
	Ngid	uint32;	// group ID
	Nmuid	uint32;	// ID of the last user that modified the file
}

// Fcall represents a 9P2000 message
type Fcall struct {
	size	uint32;	// size of the message
	Id	uint8;	// message type
	Tag	uint16;	// message tag

	Fid	uint32;		// file identifier
	Msize	uint32;		// maximum message size (used by Tversion, Rversion)
	Version	string;		// protocol version (used by Tversion, Rversion)
	Afid	uint32;		// authentication fid (used by Tauth, Tattach)
	Uname	string;		// user name (used by Tauth, Tattach)
	Aname	string;		// attach name (used by Tauth, Tattach)
	Fqid	Qid;		// file Qid (used by Rauth, Rattach, Ropen, Rcreate)
	Error	string;		// error (used by Rerror)
	Oldtag	uint16;		// tag of the message to flush (used by Tflush)
	Newfid	uint32;		// the fid that represents the file walked to (used by Twalk)
	Wnames	[]string;	// list of names to walk (used by Twalk)
	Wqids	[]Qid;		// list of Qids for the walked files (used by Rwalk)
	Mode	uint8;		// open mode (used by Topen, Tcreate)
	Iounit	uint32;		// maximum bytes read without breaking in multiple messages (used by Ropen, Rcreate)
	Name	string;		// file name (used by Tcreate)
	Perm	uint32;		// file permission (mode) (used by Tcreate)
	Offset	uint64;		// offset in the file to read/write from/to (used by Tread, Twrite)
	Count	uint32;		// number of bytes read/written (used by Tread, Rread, Twrite, Rwrite)
	Fstat	Stat;		// file description (used by Rstat, Twstat)
	Data	[]uint8;	// data read/to-write (used by Rread, Twrite)

	/* 9P2000.u extensions */
	Nerror	uint32;	// error code, 9P2000.u only (used by Rerror)
	Ext	string;	// special file description, 9P2000.u only (used by Tcreate)
	Nuname	uint32;	// user ID, 9P2000.u only (used by Tauth, Tattach)

	Pkt	[]uint8;	// raw packet data
}

// Interface for accessing users and groups
type Users interface {
	Uid2User(uid int) User;
	Uname2User(uname string) User;
	Gid2Group(gid int) Group;
	Gname2Group(gname string) Group;
}

// Represents a user
type User interface {
	Name() string;		// user name
	Id() int;		// user id
	Groups() []Group;	// groups the user belongs to (can return nil)
}

// Represents a group of users
type Group interface {
	Name() string;		// group name
	Id() int;		// group id
	Members() []User;	// list of members that belong to the group (can return nil)
}

// minimum size of a 9P2000 message for a type
var minFcsize = [...]uint32{
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
}

// minimum size of a 9P2000.u message for a type
var minFcusize = [...]uint32{
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
}

func gint8(buf []byte) (uint8, []byte)	{ return buf[0], buf[1:len(buf)] }

func gint16(buf []byte) (uint16, []byte) {
	return uint16(buf[0]) | (uint16(buf[1]) << 8), buf[2:len(buf)]
}

func gint32(buf []byte) (uint32, []byte) {
	return uint32(buf[0]) | (uint32(buf[1]) << 8) | (uint32(buf[2]) << 16) |
		(uint32(buf[3]) << 24),
		buf[4:len(buf)]
}

func Gint32(buf []byte) (uint32, []byte)	{ return gint32(buf) }

func gint64(buf []byte) (uint64, []byte) {
	return uint64(buf[0]) | (uint64(buf[1]) << 8) | (uint64(buf[2]) << 16) |
		(uint64(buf[3]) << 24) | (uint64(buf[4]) << 32) | (uint64(buf[5]) << 40) |
		(uint64(buf[6]) << 48) | (uint64(buf[7]) << 56),
		buf[8:len(buf)]
}

func gstr(buf []byte) (string, []byte) {
	var n uint16;

	if buf == nil {
		return "", nil
	}

	n, buf = gint16(buf);
	if int(n) > len(buf) {
		return "", nil
	}

	return string(buf[0:n]), buf[n:len(buf)];
}

func gqid(buf []byte, qid *Qid) []byte {
	qid.Type, buf = gint8(buf);
	qid.Version, buf = gint32(buf);
	qid.Path, buf = gint64(buf);

	return buf;
}

func gstat(buf []byte, st *Stat, dotu bool) []byte {
	st.Size, buf = gint16(buf);
	st.Type, buf = gint16(buf);
	st.Dev, buf = gint32(buf);
	buf = gqid(buf, &st.Sqid);
	st.Mode, buf = gint32(buf);
	st.Atime, buf = gint32(buf);
	st.Mtime, buf = gint32(buf);
	st.Length, buf = gint64(buf);
	st.Name, buf = gstr(buf);
	if buf == nil {
		return nil
	}

	st.Uid, buf = gstr(buf);
	if buf == nil {
		return nil
	}
	st.Gid, buf = gstr(buf);
	if buf == nil {
		return nil
	}

	st.Muid, buf = gstr(buf);
	if buf == nil {
		return nil
	}

	if dotu {
		st.Ext, buf = gstr(buf);
		if buf == nil {
			return nil
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

func pint8(val uint8, buf []byte) []byte {
	buf[0] = val;
	return buf[1:len(buf)];
}

func ppint8(val uint8, buf []byte, pval *uint8) []byte {
	*pval = val;
	return pint8(val, buf);
}

func pint16(val uint16, buf []byte) []byte {
	buf[0] = uint8(val);
	buf[1] = uint8(val >> 8);
	return buf[2:len(buf)];
}

func ppint16(val uint16, buf []byte, pval *uint16) []byte {
	*pval = val;
	return pint16(val, buf);
}

func pint32(val uint32, buf []byte) []byte {
	buf[0] = uint8(val);
	buf[1] = uint8(val >> 8);
	buf[2] = uint8(val >> 16);
	buf[3] = uint8(val >> 24);
	return buf[4:len(buf)];
}

func ppint32(val uint32, buf []byte, pval *uint32) []byte {
	*pval = val;
	return pint32(val, buf);
}

func pint64(val uint64, buf []byte) []byte {
	buf[0] = uint8(val);
	buf[1] = uint8(val >> 8);
	buf[2] = uint8(val >> 16);
	buf[3] = uint8(val >> 24);
	buf[4] = uint8(val >> 32);
	buf[5] = uint8(val >> 40);
	buf[6] = uint8(val >> 48);
	buf[7] = uint8(val >> 58);
	return buf[8:len(buf)];
}

func ppint64(val uint64, buf []byte, pval *uint64) []byte {
	*pval = val;
	return pint64(val, buf);
}

func pstr(val string, buf []byte) []byte {
	n := uint16(len(val));
	buf = pint16(n, buf);
	b := strings.Bytes(val);
	for i := 0; i < len(b); i++ {
		buf[i] = b[i]
	}
	return buf[n:len(buf)];
}

func ppstr(val string, buf []byte, pval *string) []byte {
	*pval = val;
	return pstr(val, buf);
}

func pqid(val *Qid, buf []byte) []byte {
	buf = pint8(val.Type, buf);
	buf = pint32(val.Version, buf);
	buf = pint64(val.Path, buf);

	return buf;
}

func ppqid(val *Qid, buf []byte, pval *Qid) []byte {
	*pval = *val;
	return pqid(val, buf);
}

func statsz(st *Stat, dotu bool) int {
	sz := 2 + 2 + 4 + 13 + 4 + 4 + 4 + 8 + 2 + 2 + 2 + 2 + len(st.Name) + len(st.Uid) + len(st.Gid) + len(st.Muid);
	if dotu {
		sz += 2 + 4 + 4 + 4 + len(st.Ext)
	}

	return sz;
}

func pstat(st *Stat, buf []byte, dotu bool) []byte {
	sz := statsz(st, dotu);
	buf = pint16(uint16(sz-2), buf);
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

func ppstat(st *Stat, buf []byte, dotu bool, pval *Stat) []byte {
	*pval = *st;
	return pstat(st, buf, dotu);
}

// Converts a Stat value to its on-the-wire representation and writes it to
// the buf. Returns the number of bytes written, 0 if there is not enough space.
func PackStat(st *Stat, buf []byte, dotu bool) int {
	sz := statsz(st, dotu);
	if sz > len(buf) {
		return 0
	}

	buf = pstat(st, buf, dotu);
	return sz;
}


// Converts the on-the-wire representation of a stat to Stat value.
// Returns an error if the conversion is impossible, otherwise
// a pointer to a Stat value.
func UnpackStat(buf []byte, dotu bool) (st *Stat, err *Error) {
	sz := 2 + 2 + 4 + 13 + 4 +	/* size[2] type[2] dev[4] qid[13] mode[4] */
					4 + 4 + 8 +	/* atime[4] mtime[4] length[8] */
					2 + 2 + 2 + 2;	/* name[s] uid[s] gid[s] muid[s] */

	if dotu {
		sz += 2 + 4 + 4 + 4	/* extension[s] n_uid[4] n_gid[4] n_muid[4] */
	}

	if len(buf) < sz {
	szerror:
		return nil, &Error{"short buffer", syscall.EINVAL}
	}

	st = new(Stat);
	buf = gstat(buf, st, dotu);
	if buf == nil {
		goto szerror
	}

	return st, nil;
}

// Allocates a new Fcall.
func NewFcall(sz uint32) *Fcall {
	fc := new(Fcall);
	fc.Pkt = make([]byte, sz);

	return fc;
}

// Sets the tag of a Fcall.
func SetTag(fc *Fcall, tag uint16)	{ ppint16(tag, fc.Pkt[5:len(fc.Pkt)], &fc.Tag) }

func packCommon(fc *Fcall, size int, id uint8) ([]byte, *Error) {
	size += 4 + 1 + 2;	/* size[4] id[1] tag[2] */
	if len(fc.Pkt) < int(size) {
		return nil, &Error{"buffer too small", syscall.EINVAL}
	}

	p := fc.Pkt;
	p = ppint32(uint32(size), p, &fc.size);
	p = ppint8(id, p, &fc.Id);
	p = ppint16(Notag, p, &fc.Tag);
	fc.Pkt = fc.Pkt[0:size];

	return p, nil;
}

func (err *Error) String() string	{ return err.Error }
