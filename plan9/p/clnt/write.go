// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clnt

import "plan9/p"

// Write up to len(data) bytes starting from offset. Returns the
// number of bytes written, or an Error.
func (clnt *Clnt) Write(fid *Fid, data []byte, offset uint64) (int, *p.Error) {
	tc := p.NewFcall(clnt.Msize)
	err := p.PackTwrite(tc, fid.Fid, offset, data)
	if err != nil {
		return 0, err
	}

	rc, err := clnt.rpc(tc)
	if err != nil {
		return 0, err
	}
	if rc.Type == p.Rerror {
		return 0, &p.Error{rc.Error, int(rc.Errornum)}
	}

	return int(rc.Count), nil
}

// Writes up to len(buf) bytes to a file. Returns the number of
// bytes written, or an Error.
func (file *File) Write(buf []byte) (int, *p.Error) {
	n, err := file.WriteAt(buf, file.offset)
	if err == nil {
		file.offset += uint64(n)
	}

	return n, err
}

// Writes up to len(buf) bytes starting from offset. Returns the number
// of bytes written, or an Error.
func (file *File) WriteAt(buf []byte, offset uint64) (int, *p.Error) {
	return file.fid.Clnt.Write(file.fid, buf, offset)
}

// Writes exactly len(buf) bytes starting from offset. Returns the number of
// bytes written. If Error is returned the number of bytes can be less
// than len(buf).
func (file *File) Writen(buf []byte, offset uint64) (int, *p.Error) {
	ret := 0
	for len(buf) > 0 {
		n, err := file.WriteAt(buf, offset)
		if err != nil {
			return ret, err
		}

		if n == 0 {
			break
		}

		buf = buf[n:len(buf)]
		offset += uint64(n)
		ret += n
	}

	return ret, nil
}
