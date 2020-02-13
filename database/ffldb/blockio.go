package ffldb

import (
	"container/list"
	"encoding/binary"
	"fmt"
	"github.com/wangzhen0101/wzbtc/database"
	"github.com/wangzhen0101/wzbtc/chaincfg/chainhash"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"github.com/wangzhen0101/wzbtc/wire"
)

const (
	// The Bitcoin protocol encodes block height as int32, so max number of
	// blocks is 2^31.  Max block size per the protocol is 32MiB per block.
	// So the theoretical max at the time this comment was written is 64PiB
	// (pebibytes).  With files @ 512MiB each, this would require a maximum
	// of 134,217,728 files.  Thus, choose 9 digits of precision for the
	// filenames.  An additional benefit is 9 digits provides 10^9 files @
	// 512MiB each for a total of ~476.84PiB (roughly 7.4 times the current
	// theoretical max), so there is room for the max block size to grow in
	// the future.
	blockFilenameTemplate = "%09d.fdb"

	// maxOpenFiles is the max number of open files to maintain in the
	// open blocks cache.  Note that this does not include the current
	// write file, so there will typically be one more than this value open.
	maxOpenFiles = 25

	// maxBlockFileSize is the maximum size for each file used to store
	// blocks.
	//
	// NOTE: The current code uses uint32 for all offsets, so this value
	// must be less than 2^32 (4 GiB).  This is also why it's a typed
	// constant.
	maxBlockFileSize uint32 = 512 * 1024 * 1024 // 512 MiB

	// blockLocSize is the number of bytes the serialized block location
	// data that is stored in the block index.
	//
	// The serialized block location format is:
	//
	//  [0:4]  Block file (4 bytes)
	//  [4:8]  File offset (4 bytes)
	//  [8:12] Block length (4 bytes)
	blockLocSize = 12
)

var (
	// castagnoli houses the Catagnoli polynomial used for CRC-32 checksums.
	castagnoli = crc32.MakeTable(crc32.Castagnoli)
)

type filer interface {
	io.Closer
	io.WriterAt
	io.ReaderAt
	Truncate(size int64) error
	Sync() error
}

type lockableFile struct {
	sync.RWMutex
	file filer
}

// writeCursor represents the current file and offset of the block file on disk
// for performing all writes. It also contains a read-write mutex to support
// multiple concurrent readers which can reuse the file handle.
type writeCursor struct {
	sync.RWMutex

	// curFile is the current block file that will be appended to when
	// writing new blocks.
	curFile *lockableFile

	// curFileNum is the current block file number and is used to allow
	// readers to use the same open file handle.
	curFileNum uint32

	// curOffset is the offset in the current write block file where the
	// next new block will be written.
	curOffset uint32
}


type blockStore struct {
	// network is the specific network to use in the flat files for each
	// block.
	network wire.BitcoinNet

	// basePath is the base path used for the flat block files and metadata.
	basePath string

	// maxBlockFileSize is the maximum size for each file used to store
	// blocks.  It is defined on the store so the whitebox tests can
	// override the value.
	maxBlockFileSize uint32

	// The following fields are related to the flat files which hold the
	// actual blocks.   The number of open files is limited by maxOpenFiles.
	//
	// obfMutex protects concurrent access to the openBlockFiles map.  It is
	// a RWMutex so multiple readers can simultaneously access open files.
	//
	// openBlockFiles houses the open file handles for existing block files
	// which have been opened read-only along with an individual RWMutex.
	// This scheme allows multiple concurrent readers to the same file while
	// preventing the file from being closed out from under them.
	//
	// lruMutex protects concurrent access to the least recently used list
	// and lookup map.
	//
	// openBlocksLRU tracks how the open files are refenced by pushing the
	// most recently used files to the front of the list thereby trickling
	// the least recently used files to end of the list.  When a file needs
	// to be closed due to exceeding the the max number of allowed open
	// files, the one at the end of the list is closed.
	//
	// fileNumToLRUElem is a mapping between a specific block file number
	// and the associated list element on the least recently used list.
	//
	// Thus, with the combination of these fields, the database supports
	// concurrent non-blocking reads across multiple and individual files
	// along with intelligently limiting the number of open file handles by
	// closing the least recently used files as needed.
	//
	// NOTE: The locking order used throughout is well-defined and MUST be
	// followed.  Failure to do so could lead to deadlocks.  In particular,
	// the locking order is as follows:
	//   1) obfMutex
	//   2) lruMutex
	//   3) writeCursor mutex
	//   4) specific file mutexes
	//
	// None of the mutexes are required to be locked at the same time, and
	// often aren't.  However, if they are to be locked simultaneously, they
	// MUST be locked in the order previously specified.
	//
	// Due to the high performance and multi-read concurrency requirements,
	// write locks should only be held for the minimum time necessary.
	obfMutex         sync.RWMutex
	lruMutex         sync.Mutex
	openBlocksLRU    *list.List // Contains uint32 block file numbers.
	fileNumToLRUElem map[uint32]*list.Element
	openBlockFiles   map[uint32]*lockableFile

	// writeCursor houses the state for the current file and location that
	// new blocks are written to.
	writeCursor *writeCursor

	// These functions are set to openFile, openWriteFile, and deleteFile by
	// default, but are exposed here to allow the whitebox tests to replace
	// them when working with mock files.
	openFileFunc      func(fileNum uint32) (*lockableFile, error)
	openWriteFileFunc func(fileNum uint32) (filer, error)
	deleteFileFunc    func(fileNum uint32) error
}

type blockLocation struct {
	blockFileNum uint32
	fileOffset uint32
	blockLen uint32
}

func deserializeBlockLoc(serializedLoc []byte) blockLocation {
	return blockLocation{
		blockFileNum: byteOrder.Uint32(serializedLoc[0:4]),
		fileOffset:   byteOrder.Uint32(serializedLoc[4:8]),
		blockLen:     byteOrder.Uint32(serializedLoc[8:12]),
	}
}

func serializeBlockLoc(loc blockLocation) []byte {
	// The serialized block location format is:
	//
	//  [0:4]  Block file (4 bytes)
	//  [4:8]  File offset (4 bytes)
	//  [8:12] Block length (4 bytes)
	var serializedData [12]byte
	byteOrder.PutUint32(serializedData[0:4], loc.blockFileNum)
	byteOrder.PutUint32(serializedData[4:8], loc.fileOffset)
	byteOrder.PutUint32(serializedData[8:12], loc.blockLen)
	return serializedData[:]
}

// openWriteFile returns a file handle for the passed flat file number in
// read/write mode.  The file will be created if needed.  It is typically used
// for the current file that will have all new data appended.  Unlike openFile,
// this function does not keep track of the open file and it is not subject to
// the maxOpenFiles limit.
func (s *blockStore) openWriteFile(fileNum uint32) (filer, error) {
	// The current block file needs to be read-write so it is possible to
	// append to it.  Also, it shouldn't be part of the least recently used
	// file.
	filePath := blockFilePath(s.basePath, fileNum)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		str := fmt.Sprintf("failed to open file %q: %v", filePath, err)
		return nil, makeDbErr(database.ErrDriverSpecific, str, err)
	}

	return file, nil
}


//blockFile返回的句柄已经加上的读锁
func (s *blockStore) blockFile(fileNum uint32) (*lockableFile, error) {

	// 检查正在写的文件是否就是要block的文件
	// 如果是加上读锁之后返回文件接口
	wc := s.writeCursor
	wc.RLock()
	if fileNum == wc.curFileNum && wc.curFile != nil {
		obf := wc.curFile
		obf.RLock()
		wc.RUnlock()
		return obf, nil
	}
	wc.RUnlock()

	// 检查已经打开的文件map中是否有需要的文件
	s.obfMutex.RLock()
	if obf, ok := s.openBlockFiles[fileNum]; ok {
		s.lruMutex.Lock()
		s.openBlocksLRU.MoveToFront(s.fileNumToLRUElem[fileNum])
		s.lruMutex.Unlock()

		obf.RLock()
		s.obfMutex.RUnlock()
		return obf, nil
	}
	s.obfMutex.RUnlock()

	//以写锁的模式,执行打开文件操作
	//在打开之前先检查一下是否有其他的线程打开
	s.obfMutex.Lock()
	if obf, ok := s.openBlockFiles[fileNum]; ok {
		obf.RLock()
		s.obfMutex.Unlock()
		return obf, nil
	}

	obf, err := s.openFileFunc(fileNum)
	if err != nil {
		s.obfMutex.Unlock()
		return nil, err
	}
	obf.RLock()
	s.obfMutex.Unlock()
	return obf, nil
}

func (s *blockStore) openFile(fileNum uint32) (*lockableFile, error) {
	filePath := blockFilePath(s.basePath, fileNum)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, makeDbErr(database.ErrDriverSpecific, err.Error(), err)
	}
	blockFile := &lockableFile{
		file:    file,
	}

	s.lruMutex.Lock()
	lruList := s.openBlocksLRU
	if lruList.Len() > maxOpenFiles {
		lruFileNum := lruList.Remove(lruList.Back()).(uint32)
		oldBlockFile := s.openBlockFiles[lruFileNum]

		oldBlockFile.Lock()
		_ = oldBlockFile.file.Close()
		oldBlockFile.Unlock()

		delete(s.openBlockFiles, lruFileNum)
		delete(s.fileNumToLRUElem, lruFileNum)
	}
	s.fileNumToLRUElem[fileNum] = lruList.PushFront(fileNum)
	s.lruMutex.Unlock()
	s.openBlockFiles[fileNum] = blockFile

	return blockFile, nil
}

func (s *blockStore) deleteFile(fileNum uint32) error {
	filePath := blockFilePath(s.basePath, fileNum)
	if err := os.Remove(filePath); err != nil {
		return makeDbErr(database.ErrDriverSpecific, err.Error(), err)
	}
	return nil
}

// writeData is a helper function for writeBlock which writes the provided data
// at the current write offset and updates the write cursor accordingly.  The
// field name parameter is only used when there is an error to provide a nicer
// error message.
//
// The write cursor will be advanced the number of bytes actually written in the
// event of failure.
//
// NOTE: This function MUST be called with the write cursor current file lock
// held and must only be called during a write transaction so it is effectively
// locked for writes.  Also, the write cursor current file must NOT be nil.
func (s *blockStore) writeData(data []byte, fieldName string) error {
	wc := s.writeCursor
	n, err := wc.curFile.file.WriteAt(data, int64(wc.curOffset))
	wc.curOffset += uint32(n)
	if err != nil {
		str := fmt.Sprintf("failed to write %s to file %d at "+
			"offset %d: %v", fieldName, wc.curFileNum,
			wc.curOffset-uint32(n), err)
		return makeDbErr(database.ErrDriverSpecific, str, err)
	}

	return nil
}

// Format: <network><block length><serialized block><checksum>
func (s *blockStore) writeBlock(rawBlock []byte) (blockLocation, error) {
	blockLen := uint32(len(rawBlock))
	fullLen := blockLen + 12

	wc := s.writeCursor
	finalOffset := wc.curOffset + fullLen
	if finalOffset < wc.curOffset || finalOffset > s.maxBlockFileSize {
		wc.Lock()
		wc.curFile.Lock()
		if wc.curFile.file != nil {
			_ = wc.curFile.file.Close()
			wc.curFile.file = nil
		}
		wc.curFile.Unlock()

		wc.curFileNum++
		wc.curOffset = 0
		wc.Unlock()
	}

	wc.curFile.Lock()
	defer wc.curFile.Unlock()

	if wc.curFile.file == nil {
		file, err := s.openWriteFileFunc(wc.curFileNum)
		if err != nil {
			return blockLocation{}, err
		}
		wc.curFile.file = file
	}

	origOffset := wc.curOffset
	hasher := crc32.New(castagnoli)
	var scratch [4]byte
	byteOrder.PutUint32(scratch[:], uint32(s.network))
	if err := s.writeData(scratch[:], "network"); err != nil {
		return blockLocation{}, err
	}
	_, _ = hasher.Write(scratch[:])

	byteOrder.PutUint32(scratch[:], blockLen)
	if err := s.writeData(scratch[:], "block length"); err != nil {
		return blockLocation{}, err
	}
	_, _ = hasher.Write(scratch[:])

	if err := s.writeData(rawBlock[:], "block"); err != nil {
		return blockLocation{}, err
	}
	_, _ = hasher.Write(rawBlock[:])

	if err := s.writeData(hasher.Sum(nil), "checksum"); err != nil {
		return blockLocation{}, err
	}

	loc := blockLocation{
		blockFileNum: wc.curFileNum,
		fileOffset:   origOffset,
		blockLen:     fullLen,
	}

	return loc, nil
}

// readBlock reads the specified block record and returns the serialized block.
// It ensures the integrity of the block data by checking that the serialized
// network matches the current network associated with the block store and
// comparing the calculated checksum against the one stored in the flat file.
// This function also automatically handles all file management such as opening
// and closing files as necessary to stay within the maximum allowed open files
// limit.
//
// Returns ErrDriverSpecific if the data fails to read for any reason and
// ErrCorruption if the checksum of the read data doesn't match the checksum
// read from the file.
//
// Format: <network><block length><serialized block><checksum>
func (s *blockStore) readBlock(hash *chainhash.Hash, loc blockLocation) ([]byte, error) {
	// Get the referenced block file handle opening the file as needed.  The
	// function also handles closing files as needed to avoid going over the
	// max allowed open files.
	blockFile, err := s.blockFile(loc.blockFileNum)
	if err != nil {
		return nil, err
	}

	serializedData := make([]byte, loc.blockLen)
	n, err := blockFile.file.ReadAt(serializedData, int64(loc.fileOffset))
	blockFile.RUnlock()
	if err != nil {
		str := fmt.Sprintf("failed to read block %s from file %d, "+
			"offset %d: %v", hash, loc.blockFileNum, loc.fileOffset,
			err)
		return nil, makeDbErr(database.ErrDriverSpecific, str, err)
	}

	// Calculate the checksum of the read data and ensure it matches the
	// serialized checksum.  This will detect any data corruption in the
	// flat file without having to do much more expensive merkle root
	// calculations on the loaded block.
	serializedChecksum := binary.BigEndian.Uint32(serializedData[n-4:])
	calculatedChecksum := crc32.Checksum(serializedData[:n-4], castagnoli)
	if serializedChecksum != calculatedChecksum {
		str := fmt.Sprintf("block data for block %s checksum "+
			"does not match - got %x, want %x", hash,
			calculatedChecksum, serializedChecksum)
		return nil, makeDbErr(database.ErrCorruption, str, nil)
	}

	// The network associated with the block must match the current active
	// network, otherwise somebody probably put the block files for the
	// wrong network in the directory.
	serializedNet := byteOrder.Uint32(serializedData[:4])
	if serializedNet != uint32(s.network) {
		str := fmt.Sprintf("block data for block %s is for the "+
			"wrong network - got %d, want %d", hash, serializedNet,
			uint32(s.network))
		return nil, makeDbErr(database.ErrDriverSpecific, str, nil)
	}

	// The raw block excludes the network, length of the block, and
	// checksum.
	return serializedData[8 : n-4], nil
}


// readBlockRegion reads the specified amount of data at the provided offset for
// a given block location.  The offset is relative to the start of the
// serialized block (as opposed to the beginning of the block record).  This
// function automatically handles all file management such as opening and
// closing files as necessary to stay within the maximum allowed open files
// limit.
//
// Returns ErrDriverSpecific if the data fails to read for any reason.
func (s *blockStore) readBlockRegion(loc blockLocation, offset, numBytes uint32) ([]byte, error) {
	// Get the referenced block file handle opening the file as needed.  The
	// function also handles closing files as needed to avoid going over the
	// max allowed open files.
	blockFile, err := s.blockFile(loc.blockFileNum)
	if err != nil {
		return nil, err
	}

	// Regions are offsets into the actual block, however the serialized
	// data for a block includes an initial 4 bytes for network + 4 bytes
	// for block length.  Thus, add 8 bytes to adjust.
	readOffset := loc.fileOffset + 8 + offset
	serializedData := make([]byte, numBytes)
	_, err = blockFile.file.ReadAt(serializedData, int64(readOffset))
	blockFile.RUnlock()
	if err != nil {
		str := fmt.Sprintf("failed to read region from block file %d, "+
			"offset %d, len %d: %v", loc.blockFileNum, readOffset,
			numBytes, err)
		return nil, makeDbErr(database.ErrDriverSpecific, str, err)
	}

	return serializedData, nil
}

// syncBlocks performs a file system sync on the flat file associated with the
// store's current write cursor.  It is safe to call even when there is not a
// current write file in which case it will have no effect.
//
// This is used when flushing cached metadata updates to disk to ensure all the
// block data is fully written before updating the metadata.  This ensures the
// metadata and block data can be properly reconciled in failure scenarios.
func (s *blockStore) syncBlocks() error {
	wc := s.writeCursor
	wc.RLock()
	defer wc.RUnlock()

	// Nothing to do if there is no current file associated with the write
	// cursor.
	wc.curFile.RLock()
	defer wc.curFile.RUnlock()
	if wc.curFile.file == nil {
		return nil
	}

	// Sync the file to disk.
	if err := wc.curFile.file.Sync(); err != nil {
		str := fmt.Sprintf("failed to sync file %d: %v", wc.curFileNum,
			err)
		return makeDbErr(database.ErrDriverSpecific, str, err)
	}

	return nil
}

// handleRollback rolls the block files on disk back to the provided file number
// and offset.  This involves potentially deleting and truncating the files that
// were partially written.
//
// There are effectively two scenarios to consider here:
//   1) Transient write failures from which recovery is possible
//   2) More permanent failures such as hard disk death and/or removal
//
// In either case, the write cursor will be repositioned to the old block file
// offset regardless of any other errors that occur while attempting to undo
// writes.
//
// For the first scenario, this will lead to any data which failed to be undone
// being overwritten and thus behaves as desired as the system continues to run.
//
// For the second scenario, the metadata which stores the current write cursor
// position within the block files will not have been updated yet and thus if
// the system eventually recovers (perhaps the hard drive is reconnected), it
// will also lead to any data which failed to be undone being overwritten and
// thus behaves as desired.
//
// Therefore, any errors are simply logged at a warning level rather than being
// returned since there is nothing more that could be done about it anyways.
func (s *blockStore) handleRollback(oldBlockFileNum, oldBlockOffset uint32) {
	// Grab the write cursor mutex since it is modified throughout this
	// function.
	wc := s.writeCursor
	wc.Lock()
	defer wc.Unlock()

	// Nothing to do if the rollback point is the same as the current write
	// cursor.
	if wc.curFileNum == oldBlockFileNum && wc.curOffset == oldBlockOffset {
		return
	}

	// Regardless of any failures that happen below, reposition the write
	// cursor to the old block file and offset.
	defer func() {
		wc.curFileNum = oldBlockFileNum
		wc.curOffset = oldBlockOffset
	}()

	log.Debugf("ROLLBACK: Rolling back to file %d, offset %d",
		oldBlockFileNum, oldBlockOffset)

	// Close the current write file if it needs to be deleted.  Then delete
	// all files that are newer than the provided rollback file while
	// also moving the write cursor file backwards accordingly.
	if wc.curFileNum > oldBlockFileNum {
		wc.curFile.Lock()
		if wc.curFile.file != nil {
			_ = wc.curFile.file.Close()
			wc.curFile.file = nil
		}
		wc.curFile.Unlock()
	}
	for ; wc.curFileNum > oldBlockFileNum; wc.curFileNum-- {
		if err := s.deleteFileFunc(wc.curFileNum); err != nil {
			log.Warnf("ROLLBACK: Failed to delete block file "+
				"number %d: %v", wc.curFileNum, err)
			return
		}
	}

	// Open the file for the current write cursor if needed.
	wc.curFile.Lock()
	if wc.curFile.file == nil {
		obf, err := s.openWriteFileFunc(wc.curFileNum)
		if err != nil {
			wc.curFile.Unlock()
			log.Warnf("ROLLBACK: %v", err)
			return
		}
		wc.curFile.file = obf
	}

	// Truncate the to the provided rollback offset.
	if err := wc.curFile.file.Truncate(int64(oldBlockOffset)); err != nil {
		wc.curFile.Unlock()
		log.Warnf("ROLLBACK: Failed to truncate file %d: %v",
			wc.curFileNum, err)
		return
	}

	// Sync the file to disk.
	err := wc.curFile.file.Sync()
	wc.curFile.Unlock()
	if err != nil {
		log.Warnf("ROLLBACK: Failed to sync file %d: %v",
			wc.curFileNum, err)
		return
	}
}


func blockFilePath(dbPath string, fileNum uint32) string {
	fileName := fmt.Sprintf(blockFilenameTemplate, fileNum)
	return filepath.Join(dbPath, fileName)
}

func scanBlockFiles(dbPath string) (int, uint32) {
	lastFile := -1
	fileLen := uint32(0)
	for i := 0; ; i++ {
		filePath := blockFilePath(dbPath, uint32(i))
		st, err := os.Stat(filePath)
		if err != nil {
			break
		}

		lastFile = i
		fileLen = uint32(st.Size())
	}

	log.Tracef("Scan found latest block file #%d with length %d", lastFile, fileLen)

	return lastFile, fileLen
}

func newBlockStore(basePath string, network wire.BitcoinNet) *blockStore {
	fileNum, fileOff := scanBlockFiles(basePath)
	if fileNum == -1 {
		fileNum = 0
		fileOff = 0
	}

	store := &blockStore{
		network:           network,
		basePath:          basePath,
		maxBlockFileSize:  maxBlockFileSize,
		openBlocksLRU:     list.New(),
		fileNumToLRUElem:  make(map[uint32]*list.Element),
		openBlockFiles:    make(map[uint32]*lockableFile),
		writeCursor:       &writeCursor{
			curFile:    &lockableFile{},
			curFileNum: uint32(fileNum),
			curOffset:  fileOff,
		},
	}

	store.openFileFunc = store.openFile
	store.openWriteFileFunc = store.openWriteFile
	store.deleteFileFunc = store.deleteFile

	return store
}