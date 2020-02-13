package ffldb

import "github.com/wangzhen0101/wzbtc/database"

type bucket struct {
	tx *transaction
	id [4]byte
}

// Enforce bucket implements the database.Bucket interface.
var _ database.Bucket = (*bucket)(nil)

func bucketIndexKey(parentID [4]byte, key []byte) []byte {
	// The serialized bucket index key format is:
	//   <bucketindexprefix><parentbucketid><bucketname>
	indexKey := make([]byte, len(bucketIndexPrefix)+4+len(key))
	copy(indexKey, bucketIndexPrefix)
	copy(indexKey[len(bucketIndexPrefix):], parentID[:])
	copy(indexKey[len(bucketIndexPrefix)+4:], key)
	return indexKey
}

func bucketizedKey(bucketID [4]byte, key []byte) []byte {
	// The serialized block index key format is:
	//   <bucketid><key>
	bKey := make([]byte, 4 + len(key))
	copy(bKey, bucketID[:])
	copy(bKey[4:],key)
	return bKey
}


func (b *bucket) Bucket(key []byte) database.Bucket{
	if err := b.tx.checkClosed(); err != nil {
		return nil
	}
}
func (b *bucket) CreateBucket(key []byte) (database.Bucket, error){}
func (b *bucket) CreateBucketIfNotExists(key []byte) (database.Bucket, error){}
func (b *bucket) DeleteBucket(key []byte) error{}
func (b *bucket) ForEach(func(k, v []byte) error) error{}
func (b *bucket) ForEachBucket(func(k []byte) error) error{}
func (b *bucket) Cursor() database.Cursor{}
func (b *bucket) Writable() bool{}
func (b *bucket) Put(key, value []byte) error{}
func (b *bucket) Get(key []byte) []byte{}
func (b *bucket) Delete(key []byte) error{}

