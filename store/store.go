package store

import "errors"

type DB interface {
	Put(key []byte, val []byte) error
	Get(key []byte) ([]byte, error)
}

var (
	ErrNotFoundKey = errors.New("not found key")
)

const (
	READ = iota
	WRITE
)

type storeReq struct {
	typ  int
	key  []byte
	val  []byte
	err  error
	Done chan *storeReq
}

func (r *storeReq) done() {
	r.Done <- r
}

type Store struct {
	db    DB
	reqCh chan *storeReq
}

func NewStore(db DB) *Store {
	s := &Store{
		db:    db,
		reqCh: make(chan *storeReq, 1000),
	}
	go func() {
		for req := range s.reqCh {
			switch req.typ {
			case READ:
				{
					val, err := s.db.Get(req.key)
					req.val = val
					req.err = err
					req.done()
				}
			case WRITE:
				{
					err := s.db.Put(req.key, req.val)
					req.err = err
					req.done()
				}
			}
		}
	}()

	return s
}

func (s *Store) Read(key []byte) ([]byte, error) {
	req := &storeReq{
		typ:  READ,
		key:  key,
		Done: make(chan *storeReq, 1),
	}
	s.reqCh <- req
	<-req.Done
	return req.val, req.err
}

func (s *Store) Write(key, val []byte) error {
	req := &storeReq{
		typ:  WRITE,
		key:  key,
		val:  val,
		Done: make(chan *storeReq, 1),
	}
	s.reqCh <- req
	<-req.Done
	return req.err
}
