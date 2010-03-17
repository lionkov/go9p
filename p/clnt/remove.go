// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clnt

import "go9p.googlecode.com/hg/p"


// Removes the file associated with the Fid. Returns nil if the
// operation is successful.
func (clnt *Clnt) Remove(fid *Fid) *p.Error {
	tc := p.NewFcall(clnt.Msize)
	err := p.PackTremove(tc, fid.Fid)
	if err != nil {
		return err
	}

	rc, err := clnt.rpc(tc)
	clnt.fidpool.putId(fid.Fid)

	if rc.Type == p.Rerror {
		return &p.Error{rc.Error, int(rc.Errornum)}
	}

	return err
}

// Removes the named file. Returns nil if the operation is successful.
func (clnt *Clnt) FRemove(path string) *p.Error {
	var err *p.Error
	fid, err := clnt.FWalk(path)
	if err != nil {
		return err
	}

	err = clnt.Remove(fid)
	clnt.Clunk(fid) // Remove always clunks

	return err
}
