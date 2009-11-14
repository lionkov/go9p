package p9cl

type Client interface {
	//XXX: how many of those are internal implementations?
	// These methods are intended to "know" about 9p, the client interface converting 
	Version(call) Call,
	Auth(call) Call,
	Attach(call) Call,
	Walk(call) Call,
	Open(call) Call,
	Create(call) Call,
	Read(call) Call,
	Write(call) Call,
	Clunk(call) Call,
	Remove(call) Call,
	Stat(call) Call,
	Wstat(call) Call,
	Flush(call) Call,
}
