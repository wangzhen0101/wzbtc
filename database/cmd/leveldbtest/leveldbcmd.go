package main

import (
	"fmt"
	"github.com/btcsuite/goleveldb/leveldb"
	"github.com/btcsuite/goleveldb/leveldb/filter"
	"github.com/btcsuite/goleveldb/leveldb/opt"
	"time"
)

var (
	lldb    *leveldb.DB
	DataDir = "F:\\goproject\\data\\mainnet\\blocks_ffldb\\metadata"
)

func openRawDb() error {
	opts := opt.Options{
		Compression:  opt.NoCompression,
		ErrorIfExist: false,
		Filter:       filter.NewBloomFilter(10),
		Strict:       opt.DefaultStrict,
	}
	fmt.Printf("dbPath:%s\n", DataDir)
	db, err := leveldb.OpenFile(DataDir, &opts)
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

	//err := lldb.Put([]byte("key13"), []byte("value11"), nil)
	//lldb.Put([]byte("key12"), []byte("value12"), nil)
	//if err != nil {
	//	fmt.Errorf("Put error:%s", err)
	//}
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
		//value := iter.Value()
		//fmt.Printf("interator key:%x[%s], value:%x[%s]\n", key, key, value, value)
		fmt.Printf("key:%x[%s]\n", key, key)
		time.Sleep(time.Millisecond * 100)
	}
	iter.Release()
	//err = iter.Error()
	//if err != nil {
	//	fmt.Errorf("iter error :%s", err)
	//}

	closeRawDb()
}
