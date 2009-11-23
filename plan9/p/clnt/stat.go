// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clnt

import "plan9/p"

// Returns the metadata for the file associated with the Fid, or an Error.
func (clnt *Clnt) Stat(fid *Fid) (*p.Stat, *p.Error) {
	tc := p.NewFcall(clnt.Msize);
	err := p.PackTstat(tc, fid.Fid);
	if err != nil {
		return nil, err
	}

	rc, err := clnt.rpc(tc);
	if err != nil {
		return nil, err
	}

	return &rc.Fstat, nil;
}

// Returns the metadata for a named file, or an Error.
func (clnt *Clnt) FStat(path string) (*p.Stat, *p.Error) {
	fid, err := clnt.FWalk(path);
	if err != nil {
		return nil, err
	}

	st, err := clnt.Stat(fid);
	clnt.Clunk(fid);
	return st, err;
}
