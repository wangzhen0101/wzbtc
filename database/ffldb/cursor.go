package ffldb

import (
	"github.com/btcsuite/goleveldb/leveldb/iterator"
	"github.com/wangzhen0101/wzbtc/database"
)

// cursor is an internal type used to represent a cursor over key/value pairs
// and nested buckets of a bucket and implements the database.Cursor interface.
type cursor struct {
	bucket      *bucket
	dbIter      iterator.Iterator
	pendingIter iterator.Iterator
	currentIter iterator.Iterator
}

// Enforce cursor implements the database.Cursor interface.
var _ database.Cursor = (*cursor)(nil)

func (c *cursor) Bucket() database.Bucket{}
func (c *cursor) Delete() error{}
func (c *cursor) First() bool{}
func (c *cursor) Last() bool{}
func (c *cursor) Next() bool{}
func (c *cursor) Prev() bool{}
func (c *cursor) Seek(seek []byte) bool{}
func (c *cursor) Key() []byte{}
func (c *cursor) Value() []byte{}
