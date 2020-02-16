package main

import (
	"fmt"
	"github.com/btcsuite/goleveldb/leveldb"
	"github.com/btcsuite/goleveldb/leveldb/filter"
	"github.com/btcsuite/goleveldb/leveldb/opt"
	"path/filepath"
)

var (
	lldb    *leveldb.DB
	DataDir = "F:\\goproject\\src\\github.com\\wangzhen0101\\wzbtc\\database\\cmd\\dbtest"
)

func blockDbPath2() string {
	// The database name is based on the database type.
	dbPath := filepath.Join(DataDir, "blocks_ffldb")
	dbPath = filepath.Join(dbPath, "metadata")
	return dbPath
}

func openRawDb() error {
	dbPath := blockDbPath2()
	opts := opt.Options{
		Compression:  opt.NoCompression,
		ErrorIfExist: false,
		Filter:       filter.NewBloomFilter(10),
		Strict:       opt.DefaultStrict,
	}
	fmt.Printf("dbPath:%s\n", dbPath)
	db, err := leveldb.OpenFile(dbPath, &opts)
	//db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		fmt.Errorf("open level db fail:%s", err)
		return err
	}

	fmt.Println("open leveldb success.")
	lldb = db
	return nil
}

func closeRawDb() {

	if lldb != nil {
		fmt.Println("close raw db...")
		lldb.Close()
	}
}

func main() {
	openRawDb()

	err := lldb.Put([]byte("key13"), []byte("value11"), nil)
	lldb.Put([]byte("key12"), []byte("value12"), nil)
	if err != nil {
		fmt.Errorf("Put error:%s", err)
	}
	//gv, err := lldb.Get([]byte("key11"), nil )
	//if err != nil || bytes.Compare(gv, []byte("value11")) != 0 {
	//	fmt.Errorf("value not equal :%s", err)
	//}
	////便利原始库中的所有的Key
	iter := lldb.NewIterator(nil, nil)
	for iter.Next() {
		// Remember that the contents of the returned slice should not be modified, and
		// only valid until the next call to Next.
		key := iter.Key()
		value := iter.Value()
		fmt.Printf("interator key:%x[%s], value:%x[%s]\n", key, key, value, value)
	}
	iter.Release()
	//err = iter.Error()
	//if err != nil {
	//	fmt.Errorf("iter error :%s", err)
	//}

	closeRawDb()
}
