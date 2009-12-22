// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clnt

import "plan9/p"

// Clunks a fid. Returns nil if successful.
func (clnt *Clnt) Clunk(fid *Fid) *p.Error {
	tc := p.NewFcall(clnt.Msize);
	err := p.PackTclunk(tc, fid.Fid);
	if err != nil {
		return err
	}

	rc, err := clnt.rpc(tc);
	if err != nil {
		return err
	}

	clnt.fidpool.putId(fid.Fid);

	if rc.Type == p.Rerror {
		return &p.Error{rc.Error, int(rc.Errornum)}
	}


	return err;
}

// Closes a file. Returns nil if successful.
func (file *File) Close() *p.Error {
	// Should we cancel all pending requests for the File
	return file.fid.Clnt.Clunk(file.fid)
}
