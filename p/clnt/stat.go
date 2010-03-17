// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clnt

import "code.google.com/p/go9p/p"

// Returns the metadata for the file associated with the Fid, or an Error.
func (clnt *Clnt) Stat(fid *Fid) (*p.Dir, *p.Error) {
	tc := p.NewFcall(clnt.Msize)
	err := p.PackTstat(tc, fid.Fid)
	if err != nil {
		return nil, err
	}

	rc, err := clnt.rpc(tc)
	if err != nil {
		return nil, err
	}
	if rc.Type == p.Rerror {
		return nil, &p.Error{rc.Error, int(rc.Errornum)}
	}

	return &rc.Dir, nil
}

// Returns the metadata for a named file, or an Error.
func (clnt *Clnt) FStat(path string) (*p.Dir, *p.Error) {
	fid, err := clnt.FWalk(path)
	if err != nil {
		return nil, err
	}

	d, err := clnt.Stat(fid)
	clnt.Clunk(fid)
	return d, err
}
