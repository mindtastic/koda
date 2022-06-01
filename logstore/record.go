package logstore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"math"
)

const (
	kindValue = iota
	kindTombstone
)

const (
	MaxKeyLen = math.MaxUint32
	MaxValLen = math.MaxUint32
)

// ErrInsufficientData is returned when the given data is not enouch to be
// parsed into a Record
var ErrInsufficientData = errors.New("insufficient bytes to parse a record")

// ErrCorruptData is returned when the crc checksum is not matching the provided serialized data
var ErrCorruptData = errors.New("crc checksum doesnt match the provided record data")

func NewValue(key string, value []byte) *Record {
	return &Record{
		kind:  kindValue,
		key:   key,
		value: value,
	}
}

func NewTombstone(key string) *Record {
	return &Record{
		kind:  kindTombstone,
		key:   key,
		value: NoValue,
	}
}

// Record represents a database record
//
// A Record will be serialized to a sequence of bytes in the following format
// 	[checksum]: 4 bytes
//  [type]:		1 byte
// 	[keyLen]:	4 bytes
//  [valLen]:   4 bytes
//  [key]:		>= 1 bytes
//	[value]:	valLen
type Record struct {
	kind  byte
	key   string
	value []byte
}

const (
	checksumSize = 4
	kindSize     = 1
	keyLenSize   = 4
	valLenSize   = 4
	headerLength = checksumSize + kindSize + keyLenSize + valLenSize

	checksumOffset = 0
	kindOffset     = checksumSize
	keyLenOffset   = kindOffset + kindSize
	keyValOffset   = keyLenOffset + keyLenSize
	keyOffset      = keyValOffset + valLenSize
)

var NoValue = noValue{}

type noValue = []byte

func (r *Record) Key() string {
	return r.key
}

func (r *Record) Value() []byte {
	return r.value
}

func (r *Record) IsTombstone() bool {
	return r.kind == kindTombstone
}

// Serialize serializes a record into the specified binary format
func (r *Record) Serialize() []byte {
	keyBytes := []byte(r.key)
	keyLength := uint32(len(r.key))
	valLength := uint32(len(r.value))

	recordLength := headerLength + keyLength + valLength
	recordBuffer := allocateBufferOf(recordLength)

	buf := make([]byte, 4)

	// Write 4 empty bytes as placeholder for CRC to buffer
	recordBuffer.Write(buf)

	// Write header
	recordBuffer.WriteByte(r.kind)

	binary.BigEndian.PutUint32(buf, keyLength)
	recordBuffer.Write(buf)

	binary.BigEndian.PutUint32(buf, valLength)
	recordBuffer.Write(buf)

	// Append Key and Value data
	recordBuffer.Write(keyBytes)
	recordBuffer.Write(r.value)

	serializedRecord := recordBuffer.Bytes()

	// Calculate checksum
	crc := crc32.NewIEEE()
	crc.Write(serializedRecord[4:])
	binary.BigEndian.PutUint32(serializedRecord, crc.Sum32())

	return serializedRecord
}

func Deserialize(data []byte) (*Record, error) {
	if len(data) < headerLength {
		return nil, ErrInsufficientData
	}

	readBuf := bytes.NewBuffer(data)

	checksum := uint32(binary.BigEndian.Uint32(readBuf.Next(checksumSize)))
	kind, _ := readBuf.ReadByte()
	keyLength := uint32(binary.BigEndian.Uint32(readBuf.Next(keyLenSize)))
	valLength := uint32(binary.BigEndian.Uint32(readBuf.Next(valLenSize)))

	if uint32(len(data)) < headerLength+keyLength+valLength {
		return nil, ErrInsufficientData
	}
	key := make([]byte, keyLength)
	val := make([]byte, valLength)

	copy(key, readBuf.Next(int(keyLength)))
	copy(val, readBuf.Next(int(valLength)))

	check := crc32.NewIEEE()
	check.Write(data[kindOffset : headerLength+keyLength+valLength])
	if check.Sum32() != checksum {
		return nil, ErrCorruptData
	}

	return &Record{
		kind:  kind,
		key:   string(key),
		value: val,
	}, nil
}

func allocateBufferOf(len uint32) *bytes.Buffer {
	recordBuffer := new(bytes.Buffer)

	if len > math.MaxInt32 {
		recordBuffer.Grow(math.MaxInt32)
		len -= math.MaxInt32
	}
	recordBuffer.Grow(int(len))

	return recordBuffer
}

func (r *Record) Write(w io.Writer) (int, error) {
	return w.Write(r.Serialize())
}
