package p9cl

type Clnt struct {
	sync.Mutex;
	DebugLevel	int;
	Msize		uint32;
	Dotu		bool;
	Root		*Fid;

	conn		*net.Conn;
	tagPool		*pool;
	fidPool		*pool;
	reqFirst	*req;
	reqLast		*req;
}

type Fid struct {
	sync.Mutex;
	Fs		Client;
	Iounit		uint32;
	Fqid		p9.Qid;
	Mode		uint8;
	Fid		uint32;
	offset		uint64;
}

type pool struct {
	sync.Mutex;
	maxid		uint32;
	imap		[]byte;
}

type req struct {
	sync.Mutex;
	tc		*p9.Fcall;
	rc		*p9.Fcall;
	err		*p9.Error;
	done		chan *req;
	prev, next	*req;
};

func NewClnt(c *net.Conn, msize uint32, dotu bool) (Clnt *)
{
	conn := new(Conn);
	conn.conn = c;
	conn.Msize = msize;
	conn.Dotu = dotu;
	conn.tagPool = newPool(p9.NoTag);
	conn.fidPool = newPool(p9.NoFid);
	return conn;
}

func Mount(net, addr string, msize uint32, dotu bool) (Clnt *, p9.Error *)
{
	c, err := net.Dial(net, "", addr);
	if err!=nil {
		return nil, &p9.Error{err.String(), syscall.EIO};
	}

	conn := NewClnt(c, msize, dotu);
	conn.version
}
