//go:build !windows
// +build !windows

package stores

import (
	"bytes"
	"errors"
	"log"

	"github.com/linguohua/titan/node/fsutil"
	"github.com/tecbot/gorocksdb"
)

var (
	db           *gorocksdb.DB
	writeOptions *gorocksdb.WriteOptions
	readOptions  *gorocksdb.ReadOptions
)

func newOptions() {
	writeOptions = gorocksdb.NewDefaultWriteOptions()
	writeOptions.SetSync(true)

	readOptions = gorocksdb.NewDefaultReadOptions()
	readOptions.SetFillCache(false)

}

func openRocksDB(path string) (*gorocksdb.DB, error) {
	bbto := gorocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetBlockCache(gorocksdb.NewLRUCache(64 * 1024))
	opts := gorocksdb.NewDefaultOptions()
	opts.SetBlockBasedTableFactory(bbto)
	// opts.setco
	opts.SetCreateIfMissing(true)
	opts.SetMaxOpenFiles(5000)
	opts.SetInfoLogLevel(gorocksdb.DebugInfoLogLevel)

	db, err := gorocksdb.OpenDb(opts, path)

	if err != nil {
		log.Fatalln("OPEN DB error", db, err)
		db.Close()
		return nil, errors.New("fail to open db")
	} else {
		log.Println("OPEN DB success", db)
	}
	return db, nil
}

func getRocksDB(path string) (*gorocksdb.DB, error) {
	if db == nil {
		rdb, err := openRocksDB(path)
		if err != nil {
			log.Fatal("Open rocks db failed:", err)
			return nil, err
		}

		db = rdb

		newOptions()
	}

	return db, nil
}

type rocksdb struct {
	Path string
}

func (r rocksdb) Type() string {
	return "RocksDB"
}

func (r rocksdb) Put(key string, value []byte) error {
	rdb, err := getRocksDB(r.Path)
	if err != nil {
		log.Fatal("Get rocks db failed:", err)
		return err
	}

	var k = []byte(key)
	return rdb.Put(writeOptions, k, value)
}

func (r rocksdb) Get(key string) ([]byte, error) {
	rdb, err := getRocksDB(r.Path)
	if err != nil {
		log.Fatal("Get rocks db failed:", err)
		return nil, err
	}

	var k = []byte(key)
	slice, err := rdb.Get(readOptions, k)
	if err != nil {
		return nil, err
	}

	if !slice.Exists() {
		return nil, datastore.ErrNotFound
	}

	defer slice.Free()

	return slice.Data(), nil
}

func (r rocksdb) Delete(key string) error {
	rdb, err := getRocksDB(r.Path)
	if err != nil {
		log.Fatal("Get rocks db failed:", err)
		return err
	}

	var k = []byte(key)
	return rdb.Delete(writeOptions, k)
}

func (r rocksdb) GetReader(key string) (BlockReader, error) {
	rdb, err := getRocksDB(r.Path)
	if err != nil {
		log.Fatal("Get rocks db failed:", err)
		return nil, err
	}

	var k = []byte(key)
	slice, err := rdb.Get(readOptions, k)
	if err != nil {
		return nil, err
	}

	defer slice.Free()

	reader := bytes.NewReader(slice.Data())
	return &Reader{reader}, nil
}

func (r rocksdb) Has(key string) (exists bool, err error) {
	rdb, err := getRocksDB(r.Path)
	if err != nil {
		log.Fatal("Get rocks db failed:", err)
		return false, err
	}

	var k = []byte(key)
	slice, err := rdb.Get(readOptions, k)
	if err != nil {
		return false, err
	}

	return slice.Exists(), nil
}

func (r rocksdb) GetSize(key string) (size int, err error) {
	rdb, err := getRocksDB(r.Path)
	if err != nil {
		log.Fatal("Get rocks db failed:", err)
		return 0, err
	}

	var k = []byte(key)
	slice, err := rdb.Get(readOptions, k)
	if err != nil {
		return 0, err
	}

	return slice.Size(), nil
}

func (r rocksdb) Stat() (fsutil.FsStat, error) {
	return fsutil.Statfs(r.Path)
}

func (r rocksdb) DiskUsage() (int64, error) {
	si, err := fsutil.FileSize(r.Path)
	if err != nil {
		return 0, err
	}
	return si.OnDisk, nil
}

func (r rocksdb) KeyCount() (int, error) {
	return 0, nil
}

type Reader struct {
	r *bytes.Reader
}

func (r Reader) Read(p []byte) (n int, err error) {
	return r.r.Read(p)
}

func (r Reader) Close() error {
	return nil
}

func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	return r.r.Seek(offset, whence)
}

func (r *Reader) Size() int64 {
	return r.r.Size()
}
