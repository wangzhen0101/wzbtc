package ffldb

import (
	"bytes"
	"fmt"
	"github.com/btcsuite/goleveldb/leveldb"
	"github.com/btcsuite/goleveldb/leveldb/iterator"
	"github.com/btcsuite/goleveldb/leveldb/util"
	"github.com/wangzhen0101/wzbtc/database/internal/treap"
	"sync"
	"time"
)


const (
	// defaultCacheSize is the default size for the database cache.
	defaultCacheSize = 100 * 1024 * 1024 // 100 MB

	// defaultFlushSecs is the default number of seconds to use as a
	// threshold in between database cache flushes when the cache size has
	// not been exceeded.
	defaultFlushSecs = 300 // 5 minutes

	// ldbBatchHeaderSize is the size of a leveldb batch header which
	// includes the sequence header and record counter.
	//
	// ldbRecordIKeySize is the size of the ikey used internally by leveldb
	// when appending a record to a batch.
	//
	// These are used to help preallocate space needed for a batch in one
	// allocation instead of letting leveldb itself constantly grow it.
	// This results in far less pressure on the GC and consequently helps
	// prevent the GC from allocating a lot of extra unneeded space.
	ldbBatchHeaderSize = 12
	ldbRecordIKeySize  = 8
)

// ldbCacheIter wraps a treap iterator to provide the additional functionality
// needed to satisfy the leveldb iterator.Iterator interface.
type ldbCacheIter struct {
	*treap.Iterator
}

// Enforce ldbCacheIterator implements the leveldb iterator.Iterator interface.
var _ iterator.Iterator = (*ldbCacheIter)(nil)

// Error is only provided to satisfy the iterator interface as there are no
// errors for this memory-only structure.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *ldbCacheIter) Error() error {
	return nil
}

// SetReleaser is only provided to satisfy the iterator interface as there is no
// need to override it.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *ldbCacheIter) SetReleaser(releaser util.Releaser) {
}

// Release is only provided to satisfy the iterator interface.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *ldbCacheIter) Release() {
}



func newLdbCacheIter(snap *dbCacheSnapshot, slice *util.Range) *ldbCacheIter {
	iter := snap.pendingKeys.Iterator(slice.Start, slice.Limit)
	return &ldbCacheIter{iter}
}

// dbCacheIterator defines an iterator over the key/value pairs in the database
// cache and underlying database.
type dbCacheIterator struct {
	cacheSnapshot *dbCacheSnapshot
	dbIter        iterator.Iterator
	cacheIter     iterator.Iterator
	currentIter   iterator.Iterator
	released      bool
}

// Enforce dbCacheIterator implements the leveldb iterator.Iterator interface.
var _ iterator.Iterator = (*dbCacheIterator)(nil)

// skipPendingUpdates skips any keys at the current database iterator position
// that are being updated by the cache.  The forwards flag indicates the
// direction the iterator is moving.
func (iter *dbCacheIterator) skipPendingUpdates(forwards bool) {
	for iter.dbIter.Valid() {
		var skip bool
		key := iter.dbIter.Key()
		if iter.cacheSnapshot.pendingRemove.Has(key) {
			skip = true
		} else if iter.cacheSnapshot.pendingKeys.Has(key) {
			skip = true
		}
		if !skip {
			break
		}

		if forwards {
			iter.dbIter.Next()
		} else {
			iter.dbIter.Prev()
		}
	}
}

// chooseIterator first skips any entries in the database iterator that are
// being updated by the cache and sets the current iterator to the appropriate
// iterator depending on their validity and the order they compare in while taking
// into account the direction flag.  When the iterator is being moved forwards
// and both iterators are valid, the iterator with the smaller key is chosen and
// vice versa when the iterator is being moved backwards.
func (iter *dbCacheIterator) chooseIterator(forwards bool) bool {
	// Skip any keys at the current database iterator position that are
	// being updated by the cache.
	iter.skipPendingUpdates(forwards)

	// When both iterators are exhausted, the iterator is exhausted too.
	if !iter.dbIter.Valid() && !iter.cacheIter.Valid() {
		iter.currentIter = nil
		return false
	}

	// Choose the database iterator when the cache iterator is exhausted.
	if !iter.cacheIter.Valid() {
		iter.currentIter = iter.dbIter
		return true
	}

	// Choose the cache iterator when the database iterator is exhausted.
	if !iter.dbIter.Valid() {
		iter.currentIter = iter.cacheIter
		return true
	}

	// Both iterators are valid, so choose the iterator with either the
	// smaller or larger key depending on the forwards flag.
	compare := bytes.Compare(iter.dbIter.Key(), iter.cacheIter.Key())
	if (forwards && compare > 0) || (!forwards && compare < 0) {
		iter.currentIter = iter.cacheIter
	} else {
		iter.currentIter = iter.dbIter
	}
	return true
}

// First positions the iterator at the first key/value pair and returns whether
// or not the pair exists.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) First() bool {
	// Seek to the first key in both the database and cache iterators and
	// choose the iterator that is both valid and has the smaller key.
	iter.dbIter.First()
	iter.cacheIter.First()
	return iter.chooseIterator(true)
}

// Last positions the iterator at the last key/value pair and returns whether or
// not the pair exists.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Last() bool {
	// Seek to the last key in both the database and cache iterators and
	// choose the iterator that is both valid and has the larger key.
	iter.dbIter.Last()
	iter.cacheIter.Last()
	return iter.chooseIterator(false)
}

// Next moves the iterator one key/value pair forward and returns whether or not
// the pair exists.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Next() bool {
	// Nothing to return if cursor is exhausted.
	if iter.currentIter == nil {
		return false
	}

	// Move the current iterator to the next entry and choose the iterator
	// that is both valid and has the smaller key.
	iter.currentIter.Next()
	return iter.chooseIterator(true)
}

// Prev moves the iterator one key/value pair backward and returns whether or
// not the pair exists.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Prev() bool {
	// Nothing to return if cursor is exhausted.
	if iter.currentIter == nil {
		return false
	}

	// Move the current iterator to the previous entry and choose the
	// iterator that is both valid and has the larger key.
	iter.currentIter.Prev()
	return iter.chooseIterator(false)
}

// Seek positions the iterator at the first key/value pair that is greater than
// or equal to the passed seek key.  Returns false if no suitable key was found.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Seek(key []byte) bool {
	// Seek to the provided key in both the database and cache iterators
	// then choose the iterator that is both valid and has the larger key.
	iter.dbIter.Seek(key)
	iter.cacheIter.Seek(key)
	return iter.chooseIterator(true)
}

// Valid indicates whether the iterator is positioned at a valid key/value pair.
// It will be considered invalid when the iterator is newly created or exhausted.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Valid() bool {
	return iter.currentIter != nil
}

// Key returns the current key the iterator is pointing to.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Key() []byte {
	// Nothing to return if iterator is exhausted.
	if iter.currentIter == nil {
		return nil
	}

	return iter.currentIter.Key()
}

// Value returns the current value the iterator is pointing to.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Value() []byte {
	// Nothing to return if iterator is exhausted.
	if iter.currentIter == nil {
		return nil
	}

	return iter.currentIter.Value()
}

// SetReleaser is only provided to satisfy the iterator interface as there is no
// need to override it.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) SetReleaser(releaser util.Releaser) {
}

// Release releases the iterator by removing the underlying treap iterator from
// the list of active iterators against the pending keys treap.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Release() {
	if !iter.released {
		iter.dbIter.Release()
		iter.cacheIter.Release()
		iter.currentIter = nil
		iter.released = true
	}
}

// Error is only provided to satisfy the iterator interface as there are no
// errors for this memory-only structure.
//
// This is part of the leveldb iterator.Iterator interface implementation.
func (iter *dbCacheIterator) Error() error {
	return nil
}

// dbCacheSnapshot defines a snapshot of the database cache and underlying
// database at a particular point in time.
type dbCacheSnapshot struct {
	dbSnapshot    *leveldb.Snapshot
	pendingKeys   *treap.Immutable
	pendingRemove *treap.Immutable
}

// Has returns whether or not the passed key exists.
func (snap *dbCacheSnapshot) Has(key []byte) bool {
	// Check the cached entries first.
	if snap.pendingRemove.Has(key) {
		return false
	}
	if snap.pendingKeys.Has(key) {
		return true
	}

	// Consult the database.
	hasKey, _ := snap.dbSnapshot.Has(key, nil)
	return hasKey
}

// Get returns the value for the passed key.  The function will return nil when
// the key does not exist.
func (snap *dbCacheSnapshot) Get(key []byte) []byte {
	// Check the cached entries first.
	if snap.pendingRemove.Has(key) {
		return nil
	}
	if value := snap.pendingKeys.Get(key); value != nil {
		return value
	}

	// Consult the database.
	value, err := snap.dbSnapshot.Get(key, nil)
	if err != nil {
		return nil
	}
	return value
}

// Release releases the snapshot.
func (snap *dbCacheSnapshot) Release() {
	snap.dbSnapshot.Release()
	snap.pendingKeys = nil
	snap.pendingRemove = nil
}

func (snap *dbCacheSnapshot) NewInterator(slice *util.Range) *dbCacheIterator {
	return &dbCacheIterator{
		cacheSnapshot: snap,
		dbIter:        snap.dbSnapshot.NewIterator(slice, nil),
		cacheIter:     newLdbCacheIter(snap, slice),
	}
}


type dbCache struct {
	// ldb is the underlying leveldb DB for metadata.
	ldb *leveldb.DB

	// store is used to sync blocks to flat files.
	store *blockStore

	// The following fields are related to flushing the cache to persistent
	// storage.  Note that all flushing is performed in an opportunistic
	// fashion.  This means that it is only flushed during a transaction or
	// when the database cache is closed.
	//
	// maxSize is the maximum size threshold the cache can grow to before
	// it is flushed.
	//
	// flushInterval is the threshold interval of time that is allowed to
	// pass before the cache is flushed.
	//
	// lastFlush is the time the cache was last flushed.  It is used in
	// conjunction with the current time and the flush interval.
	//
	// NOTE: These flush related fields are protected by the database write
	// lock.
	maxSize       uint64
	flushInterval time.Duration
	lastFlush     time.Time

	// The following fields hold the keys that need to be stored or deleted
	// from the underlying database once the cache is full, enough time has
	// passed, or when the database is shutting down.  Note that these are
	// stored using immutable treaps to support O(1) MVCC snapshots against
	// the cached data.  The cacheLock is used to protect concurrent access
	// for cache updates and snapshots.
	cacheLock    sync.RWMutex
	cachedKeys   *treap.Immutable
	cachedRemove *treap.Immutable
}


func (c *dbCache) Snapshot() (*dbCacheSnapshot, error) {
	dbSnapshot, err := c.ldb.GetSnapshot()
	if err != nil {
		str := "failed to open transaction"
		return nil, convertErr(str, err)
	}

	c.cacheLock.Lock()
	cacheSnapshot := &dbCacheSnapshot{
		dbSnapshot:    dbSnapshot,
		pendingKeys:   c.cachedKeys,
		pendingRemove: c.cachedRemove,
	}
	c.cacheLock.Unlock()

	return cacheSnapshot, nil
}

// updateDB invokes the passed function in the context of a managed leveldb
// transaction.  Any errors returned from the user-supplied function will cause
// the transaction to be rolled back and are returned from this function.
// Otherwise, the transaction is committed when the user-supplied function
// returns a nil error.
func (c *dbCache) updateDB(fn func(ldbTx *leveldb.Transaction) error) error {
	// Start a leveldb transaction.
	ldbTx, err := c.ldb.OpenTransaction()
	if err != nil {
		return convertErr("failed to open ldb transaction", err)
	}

	if err := fn(ldbTx); err != nil {
		ldbTx.Discard()
		return err
	}

	// Commit the leveldb transaction and convert any errors as needed.
	if err := ldbTx.Commit(); err != nil {
		return convertErr("failed to commit leveldb transaction", err)
	}
	return nil
}

// TreapForEacher is an interface which allows iteration of a treap in ascending
// order using a user-supplied callback for each key/value pair.  It mainly
// exists so both mutable and immutable treaps can be atomically committed to
// the database with the same function.
type TreapForEacher interface {
	ForEach(func(k, v []byte) bool)
}


func (c *dbCache) commitTreaps(pendingKeys, pendingRemove TreapForEacher) error {
	return c.updateDB(func(ldbTx *leveldb.Transaction) error {
		var innerErr error
		pendingKeys.ForEach(func(k, v []byte) bool {
			if dbErr := ldbTx.Put(k, v, nil); dbErr != nil {
				str := fmt.Sprintf("failed to put key %q to ldb transaction", k)
				innerErr = convertErr(str, dbErr)
				return false
			}
			return true
		})

		if innerErr != nil {
			return innerErr
		}

		pendingRemove.ForEach(func(k,v []byte) bool {
			if dbErr := ldbTx.Delete(k, nil); dbErr != nil {
				str := fmt.Sprintf("failed to delete key %q from ldb transaction", k)
				innerErr = convertErr(str, dbErr)
				return false
			}
			return true
		})

		return innerErr
	})
}

func (c *dbCache) flush() error {
	c.lastFlush = time.Now()

	// 将文件内容刷新的磁盘
	if err := c.store.syncBlocks(); err != nil {
		return err
	}

	c.cacheLock.RLock()
	cachedKeys := c.cachedKeys
	cachedRemove  := c.cachedRemove
	c.cacheLock.RUnlock()

	if cachedKeys.Len() == 0 && cachedRemove.Len() == 0 {
		return nil
	}

	// 把所有缓存的内容全部提交
	if err := c.commitTreaps(cachedKeys, cachedRemove); err != nil {
		return err
	}

	// 清空所有的缓存
	c.cacheLock.Lock()
	c.cachedKeys  = treap.NewImmutable()
	c.cachedRemove = treap.NewImmutable()
	c.cacheLock.Unlock()

	return nil
}


//判断缓存中的内容是否需要flushed
func (c *dbCache) needsFlush(tx *transaction) bool {

	// 时间到了需要刷新
	if time.Since(c.lastFlush) > c.flushInterval {
		return true
	}

	//时间未到，判断大小是否需要刷新
	snap := tx.snapshot
	totalSize := snap.pendingKeys.Size() + snap.pendingRemove.Size()
	totalSize = uint64(float64(totalSize) * 1.5)
	return totalSize > c.maxSize
}

func (c *dbCache) commitTx(tx *transaction) error {
	if c.needsFlush(tx) {
		if err := c.flush(); err != nil {
			return err
		}

		err := c.commitTreaps(tx.pendingKeys, tx.pendingRemove)
		if err != nil {
			return err
		}

		tx.pendingKeys = nil
		tx.pendingRemove = nil
		return nil
	}

	c.cacheLock.RLock()
	newCachedKeys  := c.cachedKeys
	newCachedRemove := c.cachedRemove
	c.cacheLock.RUnlock()

	tx.pendingKeys.ForEach(func(k, v []byte) bool {
		newCachedRemove = newCachedRemove.Delete(k)
		newCachedKeys = newCachedKeys.Put(k, v)
		return true
	})
	tx.pendingKeys = nil

	tx.pendingRemove.ForEach(func(k, v []byte) bool {
		newCachedKeys = newCachedKeys.Delete(k)
		newCachedRemove = newCachedRemove.Put(k, nil)
		return true
	})
	tx.pendingRemove = nil

	c.cacheLock.Lock()
	c.cachedKeys  = newCachedKeys
	c.cachedRemove = newCachedRemove
	c.cacheLock.Unlock()

	return nil
}

// Close cleanly shuts down the database cache by syncing all data and closing
// the underlying leveldb database.
//
// This function MUST be called with the database write lock held.
func (c *dbCache) Close() error {
	// Flush any outstanding cached entries to disk.
	if err := c.flush(); err != nil {
		// Even if there is an error while flushing, attempt to close
		// the underlying database.  The error is ignored since it would
		// mask the flush error.
		_ = c.ldb.Close()
		return err
	}

	// Close the underlying leveldb database.
	if err := c.ldb.Close(); err != nil {
		str := "failed to close underlying leveldb database"
		return convertErr(str, err)
	}

	return nil
}


// newDbCache returns a new database cache instance backed by the provided
// leveldb instance.  The cache will be flushed to leveldb when the max size
// exceeds the provided value or it has been longer than the provided interval
// since the last flush.
func newDbCache(ldb *leveldb.DB, store *blockStore, maxSize uint64, flushIntervalSecs uint32) *dbCache {
	return &dbCache{
		ldb:           ldb,
		store:         store,
		maxSize:       maxSize,
		flushInterval: time.Second * time.Duration(flushIntervalSecs),
		lastFlush:     time.Now(),
		cachedKeys:    treap.NewImmutable(),
		cachedRemove:  treap.NewImmutable(),
	}
}

