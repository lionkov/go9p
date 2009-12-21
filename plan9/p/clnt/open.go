// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clnt

import "plan9/p"
import "strings"

// Opens the file associated with the fid. Returns nil if
// the operation is successful.
func (clnt *Clnt) Open(fid *Fid, mode uint8) *p.Error {
	tc := p.NewFcall(clnt.Msize);
	err := p.PackTopen(tc, fid.Fid, mode);
	if err != nil {
		return err
	}

	rc, err := clnt.rpc(tc);
	if err != nil {
		return err
	}

	fid.Qid = rc.Qid;
	fid.Iounit = rc.Iounit;
	if fid.Iounit == 0 || fid.Iounit > clnt.Msize-p.IOHDRSZ {
		fid.Iounit = clnt.Msize - p.IOHDRSZ
	}
	fid.Mode = mode;
	return nil;
}

// Creates a file in the directory associated with the fid. Returns nil
// if the operation is successful.
func (clnt *Clnt) Create(fid *Fid, name string, perm uint32, mode uint8, ext string) *p.Error {
	tc := p.NewFcall(clnt.Msize);
	err := p.PackTcreate(tc, fid.Fid, name, perm, mode, ext, clnt.Dotu);
	if err != nil {
		return err
	}

	rc, err := clnt.rpc(tc);
	if err != nil {
		return err
	}

	fid.Qid = rc.Qid;
	fid.Iounit = rc.Iounit;
	if fid.Iounit == 0 || fid.Iounit > clnt.Msize-p.IOHDRSZ {
		fid.Iounit = clnt.Msize - p.IOHDRSZ
	}
	fid.Mode = mode;
	return nil;
}

// Creates and opens a named file.
// Returns the file if the operation is successful, or an Error.
func (clnt *Clnt) FCreate(path string, perm uint32, mode uint8) (*File, *p.Error) {
	n := strings.LastIndex(path, "/");
	if n < 0 {
		n = 0
	}

	fid, err := clnt.FWalk(path[0:n]);
	if err != nil {
		return nil, err
	}

	err = clnt.Create(fid, path[n+1:len(path)], perm, mode, "");
	if err != nil {
		clnt.Clunk(fid);
		return nil, err;
	}

	return &File{fid, 0}, nil;
}

// Opens a named file. Returns the opened file, or an Error.
func (clnt *Clnt) FOpen(path string, mode uint8) (*File, *p.Error) {
	fid, err := clnt.FWalk(path);
	if err != nil {
		return nil, err
	}

	err = clnt.Open(fid, mode);
	if err != nil {
		clnt.Clunk(fid);
		return nil, err;
	}

	return &File{fid, 0}, nil;
}
