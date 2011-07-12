// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clnt

import "os"
import "go9p.googlecode.com/hg/p"


// Removes the file associated with the Fid. Returns nil if the
// operation is successful.
func (clnt *Clnt) Remove(fid *Fid) os.Error {
	tc := clnt.NewFcall()
	err := p.PackTremove(tc, fid.Fid)
	if err != nil {
		return err
	}

	rc, err := clnt.Rpc(tc)
	clnt.fidpool.putId(fid.Fid)
	fid.Fid = p.NOFID

	if rc.Type == p.Rerror {
		return &p.Error{rc.Error, int(rc.Errornum)}
	}

	return err
}

// Removes the named file. Returns nil if the operation is successful.
func (clnt *Clnt) FRemove(path string) os.Error {
	var err os.Error
	fid, err := clnt.FWalk(path)
	if err != nil {
		return err
	}

	err = clnt.Remove(fid)
	return err
}
