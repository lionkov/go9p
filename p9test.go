package main

import (
	"log";
	"time";
    "./p9";
	"./p9srv";
)

type TimeFs struct {
	started string
}

func (tfs *TimeFs) Start(srv *p9srv.Srv, impl p9srv.SrvImpl) {
	// this is never called :(
	log.Stderr("starting at:", tfs.started);
	tfs.started = time.LocalTime().String();
	return;
}

func (tfs *TimeFs) ConnOpened(*p9srv.Conn) {
	// nothing
}
func (tfs *TimeFs) ConnClosed(*p9srv.Conn){
	// nothing
}
func (tfs *TimeFs) FidDestroy(*p9srv.Fid){
}
func (tfs *TimeFs) ReqProcess(req *p9srv.Req){
}
func (tfs *TimeFs) ReqDestroy(*p9srv.Req){
}

func (tfs *TimeFs) Attach(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Flush(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Walk(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Open(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Create(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Read(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Write(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Clunk(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Remove(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Stat(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}
func (tfs *TimeFs) Wstat(req *p9srv.Req){
	req.RespondError(&p9.Error{"unimplemented", 0})
}

func main() {
	tm := new(TimeFs);
	srv := new(p9srv.Srv);
	srv.Debuglevel = 1;
	srv.Start(tm);
	p9srv.StartListener("tcp", ":10000", srv);
}
