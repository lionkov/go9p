// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

// Create a Tversion message in the specified Fcall.
func PackTversion(fc *Fcall, msize uint32, version string) *Error {
	size := 4 + 2 + len(version);	/* msize[4] version[s] */
	p, err := packCommon(fc, size, Tversion);
	if err != nil {
		return err
	}

	p = ppint32(msize, p, &fc.Msize);
	p = ppstr(version, p, &fc.Version);

	return nil;
}

// Create a Tauth message in the specified Fcall.
func PackTauth(fc *Fcall, fid uint32, uname string, aname string, nuname uint32, dotu bool) *Error {
	size := 4 + 2 + 2 + len(uname) + len(aname);	/* fid[4] uname[s] aname[s] */
	if dotu {
		size += 4	/* n_uname[4] */
	}

	p, err := packCommon(fc, size, Tauth);
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppstr(uname, p, &fc.Uname);
	p = ppstr(aname, p, &fc.Aname);
	if dotu {
		p = ppint32(nuname, p, &fc.Nuname)
	}

	return nil;
}

// Create a Tflush message in the specified Fcall.
func PackTflush(fc *Fcall, oldtag uint16) *Error {
	p, err := packCommon(fc, 2, Tflush);
	if err != nil {
		return err
	}

	p = ppint16(oldtag, p, &fc.Oldtag);
	return nil;
}

// Create a Tattach message in the specified Fcall. If dotu is true,
// the function will create 9P2000.u including the nuname value, otherwise
// nuname is ignored.
func PackTattach(fc *Fcall, fid uint32, afid uint32, uname string, aname string, nuname uint32, dotu bool) *Error {
	size := 4 + 4 + 2 + len(uname) + 2 + len(aname);	/* fid[4] afid[4] uname[s] aname[s] */
	if dotu {
		size += 4
	}

	p, err := packCommon(fc, size, Tattach);
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint32(afid, p, &fc.Afid);
	p = ppstr(uname, p, &fc.Uname);
	p = ppstr(aname, p, &fc.Aname);
	if dotu {
		p = ppint32(nuname, p, &fc.Nuname)
	}

	return nil;
}

// Create a Twalk message in the specified Fcall.
func PackTwalk(fc *Fcall, fid uint32, newfid uint32, wnames []string) *Error {
	nwname := len(wnames);
	size := 4 + 4 + 2 + nwname*2;	/* fid[4] newfid[4] nwname[2] nwname*wname[s] */
	for i := 0; i < nwname; i++ {
		size += len(wnames[i])
	}

	p, err := packCommon(fc, size, Twalk);
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint32(newfid, p, &fc.Newfid);
	p = pint16(uint16(nwname), p);
	fc.Wnames = make([]string, nwname);
	for i := 0; i < nwname; i++ {
		p = ppstr(wnames[i], p, &fc.Wnames[i])
	}

	return nil;
}

// Create a Topen message in the specified Fcall.
func PackTopen(fc *Fcall, fid uint32, mode uint8) *Error {
	size := 4 + 1;	/* fid[4] mode[1] */
	p, err := packCommon(fc, size, Topen);
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint8(mode, p, &fc.Mode);
	return nil;
}

// Create a Tcreate message in the specified Fcall. If dotu is true,
// the function will create a 9P2000.u message that includes ext.
// Otherwise the ext value is ignored.
func PackTcreate(fc *Fcall, fid uint32, name string, perm uint32, mode uint8, ext string, dotu bool) *Error {
	size := 4 + 2 + len(name) + 4 + 1;	/* fid[4] name[s] perm[4] mode[1] */

	if dotu {
		size += 2 + len(ext)
	}

	p, err := packCommon(fc, size, Tcreate);
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppstr(name, p, &fc.Name);
	p = ppint32(perm, p, &fc.Perm);
	p = ppint8(mode, p, &fc.Mode);

	if dotu {
		p = ppstr(ext, p, &fc.Ext)
	}

	return nil;
}

// Create a Tread message in the specified Fcall.
func PackTread(fc *Fcall, fid uint32, offset uint64, count uint32) *Error {
	size := 4 + 8 + 4;	/* fid[4] offset[8] count[4] */
	p, err := packCommon(fc, size, Tread);
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint64(offset, p, &fc.Offset);
	p = ppint32(count, p, &fc.Count);
	return nil;
}

// Create a Twrite message in the specified Fcall.
func PackTwrite(fc *Fcall, fid uint32, offset uint64, data []byte) *Error {
	count := len(data);
	size := 4 + 8 + 4 + count;	/* fid[4] offset[8] count[4] data[count] */
	p, err := packCommon(fc, size, Twrite);
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppint64(offset, p, &fc.Offset);
	p = ppint32(uint32(count), p, &fc.Count);
	fc.Data = p;
	for i := 0; i < len(data); i++ {
		fc.Data[i] = data[i]
	}
	return nil;
}

// Create a Tclunk message in the specified Fcall.
func PackTclunk(fc *Fcall, fid uint32) *Error {
	p, err := packCommon(fc, 4, Tclunk);	/* fid[4] */
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	return nil;
}

// Create a Tremove message in the specified Fcall.
func PackTremove(fc *Fcall, fid uint32) *Error {
	p, err := packCommon(fc, 4, Tremove);	/* fid[4] */
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	return nil;
}

// Create a Tstat message in the specified Fcall.
func PackTstat(fc *Fcall, fid uint32) *Error {
	p, err := packCommon(fc, 4, Tstat);	/* fid[4] */
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	return nil;
}

// Create a Tauth message in the specified Fcall. If dotu is true
// the function will create 9P2000.u message, otherwise the 9P2000.u
// specific fields from the Stat value will be ignored.
func PackTwstat(fc *Fcall, fid uint32, st *Stat, dotu bool) *Error {
	stsz := statsz(st, dotu);
	size := 4 + 2 + stsz;	/* fid[4] stat[n] */
	p, err := packCommon(fc, size, Twstat);
	if err != nil {
		return err
	}

	p = ppint32(fid, p, &fc.Fid);
	p = ppstat(st, p, dotu, &fc.Fstat);
	return nil;
}
