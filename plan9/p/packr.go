// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

// Create a Rversion message in the specified Fcall.
func PackRversion(fc *Fcall, msize uint32, version string) *Error {
	size := 4 + 2 + len(version);	/* msize[4] version[s] */
	p, err := packCommon(fc, size, Rversion);
	if err != nil {
		return err
	}

	p = ppint32(msize, p, &fc.Msize);
	p = ppstr(version, p, &fc.Version);

	return nil;
}

// Create a Rauth message in the specified Fcall.
func PackRauth(fc *Fcall, aqid *Qid) *Error {
	size := 13;	/* aqid[13] */
	p, err := packCommon(fc, size, Rauth);
	if err != nil {
		return err
	}

	p = ppqid(aqid, p, &fc.Fqid);
	return nil;
}

// Create a Rerror message in the specified Fcall. If dotu is true,
// the function will create a 9P2000.u message. If false, nerror is
// ignored.
func PackRerror(fc *Fcall, error string, nerror uint32, dotu bool) *Error {
	size := 2 + len(error);	/* ename[s] */
	if dotu {
		size += 4	/* ecode[4] */
	}

	p, err := packCommon(fc, size, Rerror);
	if err != nil {
		return err
	}

	p = ppstr(error, p, &fc.Error);
	if dotu {
		p = ppint32(nerror, p, &fc.Nerror)
	}

	return nil;
}

// Create a Rflush message in the specified Fcall.
func PackRflush(fc *Fcall) *Error {
	_, err := packCommon(fc, 0, Rflush);

	return err;
}

// Create a Rattach message in the specified Fcall.
func PackRattach(fc *Fcall, aqid *Qid) *Error {
	size := 13;	/* aqid[13] */
	p, err := packCommon(fc, size, Rattach);
	if err != nil {
		return err
	}

	p = ppqid(aqid, p, &fc.Fqid);
	return nil;
}

// Create a Rwalk message in the specified Fcall.
func PackRwalk(fc *Fcall, wqids []Qid) *Error {
	nwqid := len(wqids);
	size := 2 + nwqid*13;	/* nwqid[2] nwname*wqid[13] */
	p, err := packCommon(fc, size, Rwalk);
	if err != nil {
		return err
	}

	p = pint16(uint16(nwqid), p);
	fc.Wqids = make([]Qid, nwqid);
	for i := 0; i < nwqid; i++ {
		p = ppqid(&wqids[i], p, &fc.Wqids[i])
	}

	return nil;
}

// Create a Ropen message in the specified Fcall.
func PackRopen(fc *Fcall, qid *Qid, iounit uint32) *Error {
	size := 13 + 4;	/* qid[13] iounit[4] */
	p, err := packCommon(fc, size, Ropen);
	if err != nil {
		return err
	}

	p = ppqid(qid, p, &fc.Fqid);
	p = ppint32(iounit, p, &fc.Iounit);
	return nil;
}

// Create a Rcreate message in the specified Fcall.
func PackRcreate(fc *Fcall, qid *Qid, iounit uint32) *Error {
	size := 13 + 4;	/* qid[13] iounit[4] */
	p, err := packCommon(fc, size, Rcreate);
	if err != nil {
		return err
	}

	p = ppqid(qid, p, &fc.Fqid);
	p = ppint32(iounit, p, &fc.Iounit);
	return nil;
}

// Initializes the specified Fcall value to contain Rread message.
// The user should copy the returned data to the slice pointed by
// fc.Data and call SetRreadCount to update the data size to the
// actual value.
func InitRread(fc *Fcall, count uint32) *Error {
	size := int(4 + count);	/* count[4] data[count] */
	p, err := packCommon(fc, size, Rread);
	if err != nil {
		return err
	}

	p = ppint32(count, p, &fc.Count);
	fc.Data = p[0:fc.Count];
	return nil;
}

// Updates the size of the data returned by Rread. Expects that
// the Fcall value is already initialized by InitRread.
func SetRreadCount(fc *Fcall, count uint32) {
	/* we need to update both the packet size as well as the data count */
	size := 4 + 1 + 2 + 4 + count;	/* size[4] id[1] tag[2] count[4] data[count] */
	ppint32(size, fc.Pkt, &fc.size);
	ppint32(count, fc.Pkt[7:len(fc.Pkt)], &fc.Count);
	fc.Pkt = fc.Pkt[0:size];
	fc.Data = fc.Data[0:count];
	fc.size = size;
}

// Create a Rread message in the specified Fcall.
func PackRread(fc *Fcall, data []byte) *Error {
	count := uint32(len(data));
	err := InitRread(fc, count);
	if err != nil {
		return err
	}

	for i := 0; i < len(data); i++ {
		fc.Data[i] = data[i]
	}

	return nil;
}

// Create a Rwrite message in the specified Fcall.
func PackRwrite(fc *Fcall, count uint32) *Error {
	p, err := packCommon(fc, 4, Rwrite);	/* count[4] */
	if err != nil {
		return err
	}

	p = ppint32(count, p, &fc.Count);
	return nil;
}

// Create a Rclunk message in the specified Fcall.
func PackRclunk(fc *Fcall) *Error {
	_, err := packCommon(fc, 0, Rclunk);
	return err;
}

// Create a Rremove message in the specified Fcall.
func PackRremove(fc *Fcall) *Error {
	_, err := packCommon(fc, 0, Rremove);
	return err;
}

// Create a Rstat message in the specified Fcall. If dotu is true, the
// function will create a 9P2000.u stat representation that includes
// st.Nuid, st.Ngid, st.Nmuid and st.Ext. Otherwise these values will be
// ignored.
func PackRstat(fc *Fcall, st *Stat, dotu bool) *Error {
	stsz := statsz(st, dotu);
	size := 2 + stsz;	/* stat[n] */
	p, err := packCommon(fc, size, Rstat);
	if err != nil {
		return err
	}

	p = pint16(uint16(stsz), p);
	p = ppstat(st, p, dotu, &fc.Fstat);
	return nil;
}

// Create a Rwstat message in the specified Fcall.
func PackRwstat(fc *Fcall) *Error {
	_, err := packCommon(fc, 0, Rwstat);
	return err;
}
