package ffldb

import (
	"encoding/binary"
	"fmt"
	"github.com/btcsuite/goleveldb/leveldb"
	ldberrors "github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/btcsuite/goleveldb/leveldb/filter"
	"github.com/btcsuite/goleveldb/leveldb/opt"
	"github.com/wangzhen0101/wzbtc/database"
	"github.com/wangzhen0101/wzbtc/database/internal/treap"
	"github.com/wangzhen0101/wzbtc/wire"
	"os"
	"path/filepath"
	"sync"
)

const (
	// metadataDbName is the name used for the metadata database.
	metadataDbName = "metadata"

	// blockHdrSize is the size of a block header.  This is simply the
	// constant from wire and is only provided here for convenience since
	// wire.MaxBlockHeaderPayload is quite long.
	blockHdrSize = wire.MaxBlockHeaderPayload

	// blockHdrOffset defines the offsets into a block index row for the
	// block header.
	//
	// The serialized block index row format is:
	//   <blocklocation><blockheader>
	blockHdrOffset = blockLocSize
)

var (
	// byteOrder is the preferred byte order used through the database and
	// block files.  Sometimes big endian will be used to allow ordered byte
	// sortable integer values.
	byteOrder = binary.LittleEndian

	// bucketIndexPrefix is the prefix used for all entries in the bucket
	// index.
	bucketIndexPrefix = []byte("bidx")

	// curBucketIDKeyName is the name of the key used to keep track of the
	// current bucket ID counter.
	curBucketIDKeyName = []byte("bidx-cbid")

	// metadataBucketID is the ID of the top-level metadata bucket.
	// It is the value 0 encoded as an unsigned big-endian uint32.
	metadataBucketID = [4]byte{}

	// blockIdxBucketID is the ID of the internal block metadata bucket.
	// It is the value 1 encoded as an unsigned big-endian uint32.
	blockIdxBucketID = [4]byte{0x00, 0x00, 0x00, 0x01}

	// blockIdxBucketName is the bucket used internally to track block
	// metadata.
	blockIdxBucketName = []byte("ffldb-blockidx")

	// writeLocKeyName is the key used to store the current write file
	// location.
	writeLocKeyName = []byte("ffldb-writeloc")
)

// Common error strings.
const (
	// errDbNotOpenStr is the text to use for the database.ErrDbNotOpen
	// error code.
	errDbNotOpenStr = "database is not open"

	// errTxClosedStr is the text to use for the database.ErrTxClosed error
	// code.
	errTxClosedStr = "database tx is closed"
)

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// bulkFetchData is allows a block location to be specified along with the
// index it was requested from.  This in turn allows the bulk data loading
// functions to sort the data accesses based on the location to improve
// performance while keeping track of which result the data is for.
type bulkFetchData struct {
	*blockLocation
	replyIndex int
}

// bulkFetchDataSorter implements sort.Interface to allow a slice of
// bulkFetchData to be sorted.  In particular it sorts by file and then
// offset so that reads from files are grouped and linear.
type bulkFetchDataSorter []bulkFetchData

// Len returns the number of items in the slice.  It is part of the
// sort.Interface implementation.
func (s bulkFetchDataSorter) Len() int {
	return len(s)
}

// Swap swaps the items at the passed indices.  It is part of the
// sort.Interface implementation.
func (s bulkFetchDataSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the item with index i should sort before the item with
// index j.  It is part of the sort.Interface implementation.
func (s bulkFetchDataSorter) Less(i, j int) bool {
	if s[i].blockFileNum < s[j].blockFileNum {
		return true
	}
	if s[i].blockFileNum > s[j].blockFileNum {
		return false
	}

	return s[i].fileOffset < s[j].fileOffset
}

// makeDbErr creates a database.Error given a set of arguments.
func makeDbErr(c database.ErrorCode, desc string, err error) database.Error {
	return database.Error{ErrorCode: c, Description: desc, Err: err}
}

// convertErr converts the passed leveldb error into a database error with an
// equivalent error code  and the passed description.  It also sets the passed
// error as the underlying error.
func convertErr(desc string, ldbErr error) database.Error {
	// Use the driver-specific error code by default.  The code below will
	// update this with the converted error if it's recognized.
	var code = database.ErrDriverSpecific

	switch {
	// Database corruption errors.
	case ldberrors.IsCorrupted(ldbErr):
		code = database.ErrCorruption

	// Database open/create errors.
	case ldbErr == leveldb.ErrClosed:
		code = database.ErrDbNotOpen

	// Transaction errors.
	case ldbErr == leveldb.ErrSnapshotReleased:
		code = database.ErrTxClosed
	case ldbErr == leveldb.ErrIterReleased:
		code = database.ErrTxClosed
	}

	return database.Error{ErrorCode: code, Description: desc, Err: ldbErr}
}

type db struct {
	writeLock sync.Mutex
	closeLock sync.RWMutex
	closed    bool
	store     *blockStore
	cache     *dbCache
}

var _ database.DB = (*db)(nil)

func (db *db) Type() string {
	return dbType
}

func (db *db) begin(writable bool) (*transaction, error) {
	if writable {
		db.writeLock.Lock()
	}

	db.closeLock.RLock()
	if db.closed {
		db.closeLock.RUnlock()
		if writable {
			db.writeLock.Unlock()
		}

		return nil, makeDbErr(database.ErrDbNotOpen, errDbNotOpenStr,
			nil)
	}

	snapshot, err := db.cache.Snapshot()
	if err != nil {
		db.closeLock.RUnlock()
		if writable {
			db.writeLock.Unlock()
		}

		return nil, err
	}

	tx := &transaction{
		writable:      writable,
		db:            db,
		snapshot:      snapshot,
		pendingKeys:   treap.NewMutable(),
		pendingRemove: treap.NewMutable(),
	}

	tx.metaBucket = &bucket{tx, metadataBucketID}
	tx.blockIdxBucket = &bucket{tx, blockIdxBucketID}

	return tx, nil
}

func (db *db) Begin(writable bool) (database.Tx, error) {
	return db.begin(writable)
}

func rollbackOnPanic(tx *transaction) {
	if err := recover(); err != nil {
		tx.managed = false
		_ = tx.Rollback()
		panic(err)
	}
}

func (db *db) View(fn func(database.Tx) error) error {
	tx, err := db.begin(false)
	if err != nil {
		return err
	}

	defer rollbackOnPanic(tx)

	tx.managed = true
	err = fn(tx)
	tx.managed = false

	if err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Rollback()
}

func (db *db) Update(fn func(database.Tx) error) error {
	// Start a read-write transaction.
	tx, err := db.begin(true)
	if err != nil {
		return err
	}

	// Since the user-provided function might panic, ensure the transaction
	// releases all mutexes and resources.  There is no guarantee the caller
	// won't use recover and keep going.  Thus, the database must still be
	// in a usable state on panics due to caller issues.
	defer rollbackOnPanic(tx)

	tx.managed = true
	err = fn(tx)
	tx.managed = false
	if err != nil {
		// The error is ignored here because nothing was written yet
		// and regardless of a rollback failure, the tx is closed now
		// anyways.
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (db *db) Close() error {
	// Since all transactions have a read lock on this mutex, this will
	// cause Close to wait for all readers to complete.
	db.closeLock.Lock()
	defer db.closeLock.Unlock()

	if db.closed {
		return makeDbErr(database.ErrDbNotOpen, errDbNotOpenStr, nil)
	}
	db.closed = true

	// NOTE: Since the above lock waits for all transactions to finish and
	// prevents any new ones from being started, it is safe to flush the
	// cache and clear all state without the individual locks.

	// Close the database cache which will flush any existing entries to
	// disk and close the underlying leveldb database.  Any error is saved
	// and returned at the end after the remaining cleanup since the
	// database will be marked closed even if this fails given there is no
	// good way for the caller to recover from a failure here anyways.
	closeErr := db.cache.Close()

	// Close any open flat files that house the blocks.
	wc := db.store.writeCursor
	if wc.curFile.file != nil {
		_ = wc.curFile.file.Close()
		wc.curFile.file = nil
	}
	for _, blockFile := range db.store.openBlockFiles {
		_ = blockFile.file.Close()
	}
	db.store.openBlockFiles = nil
	db.store.openBlocksLRU.Init()
	db.store.fileNumToLRUElem = nil

	return closeErr
}

func initDB(ldb *leveldb.DB) error {
	batch := new(leveldb.Batch)

	//当前使用的文件编号和偏移量
	//0000ffldb-writeloc : 00000000checksum
	batch.Put(bucketizedKey(metadataBucketID, writeLocKeyName),
		serializeWriteRow(0, 0))

	//bidx0000ffldb-blockidx : 0001
	//bidx-cbid : 0001
	batch.Put(bucketIndexKey(metadataBucketID, blockIdxBucketName),
		blockIdxBucketID[:])

	batch.Put(curBucketIDKeyName, blockIdxBucketID[:])

	if err := ldb.Write(batch, nil); err != nil {
		str := fmt.Sprintf("failed to initialize metadata database: %v",
			err)
		return convertErr(str, err)
	}

	return nil
}

func openDB(dbPath string, network wire.BitcoinNet, create bool) (database.DB, error) {
	// Error if the database doesn't exist and the create flag is not set.
	metadataDbPath := filepath.Join(dbPath, metadataDbName)
	dbExists := fileExists(metadataDbPath)
	if !create && !dbExists {
		str := fmt.Sprintf("database %q does not exist", metadataDbPath)
		return nil, makeDbErr(database.ErrDbDoesNotExist, str, nil)
	}

	if !dbExists {
		_ = os.MkdirAll(dbPath, 0700)
	}

	opts := opt.Options{
		Compression:  opt.NoCompression,
		ErrorIfExist: create,
		Filter:       filter.NewBloomFilter(10),
		Strict:       opt.DefaultStrict,
	}

	ldb, err := leveldb.OpenFile(metadataDbPath, &opts)
	if err != nil {
		return nil, convertErr(err.Error(), err)
	}

	store := newBlockStore(dbPath, network)
	cache := newDbCache(ldb, store, defaultCacheSize, defaultFlushSecs)
	pdb := &db{store: store, cache: cache}

	return reconcileDB(pdb, create)
}
