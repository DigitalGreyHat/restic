package pack

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/restic/restic/internal/crypto"
	"github.com/restic/restic/internal/restic"
	rtest "github.com/restic/restic/internal/test"
)

func TestParseHeaderEntry(t *testing.T) {
	h := headerEntry{
		Type:   0, // Blob.
		Length: 100,
	}
	for i := range h.ID {
		h.ID[i] = byte(i)
	}

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, &h)

	b, err := parseHeaderEntry(buf.Bytes())
	rtest.OK(t, err)
	rtest.Equals(t, restic.DataBlob, b.Type)
	t.Logf("%v %v", h.ID, b.ID)
	rtest.Assert(t, bytes.Equal(h.ID[:], b.ID[:]), "id mismatch")
	rtest.Equals(t, uint(h.Length), b.Length)

	h.Type = 0xae
	buf.Reset()
	_ = binary.Write(buf, binary.LittleEndian, &h)

	b, err = parseHeaderEntry(buf.Bytes())
	rtest.Assert(t, err != nil, "no error for invalid type")

	h.Type = 0
	buf.Reset()
	_ = binary.Write(buf, binary.LittleEndian, &h)

	b, err = parseHeaderEntry(buf.Bytes()[:entrySize-1])
	rtest.Assert(t, err != nil, "no error for short input")
}

type countingReaderAt struct {
	delegate        io.ReaderAt
	invocationCount int
}

func (rd *countingReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	rd.invocationCount++
	return rd.delegate.ReadAt(p, off)
}

func TestReadHeaderEagerLoad(t *testing.T) {

	testReadHeader := func(dataSize, entryCount, expectedReadInvocationCount int) {
		expectedHeader := rtest.Random(0, entryCount*int(entrySize)+crypto.Extension)

		buf := &bytes.Buffer{}
		buf.Write(rtest.Random(0, dataSize))                                             // pack blobs data
		buf.Write(expectedHeader)                                                        // pack header
		rtest.OK(t, binary.Write(buf, binary.LittleEndian, uint32(len(expectedHeader)))) // pack header length

		rd := &countingReaderAt{delegate: bytes.NewReader(buf.Bytes())}

		header, err := readHeader(rd, int64(buf.Len()))
		rtest.OK(t, err)

		rtest.Equals(t, expectedHeader, header)
		rtest.Equals(t, expectedReadInvocationCount, rd.invocationCount)
	}

	// basic
	testReadHeader(100, 1, 1)

	// header entries == eager entries
	testReadHeader(100, eagerEntries-1, 1)
	testReadHeader(100, eagerEntries, 1)
	testReadHeader(100, eagerEntries+1, 2)

	// file size == eager header load size
	eagerLoadSize := int((eagerEntries * entrySize) + crypto.Extension)
	headerSize := int(1*entrySize) + crypto.Extension
	dataSize := eagerLoadSize - headerSize - binary.Size(uint32(0))
	testReadHeader(dataSize-1, 1, 1)
	testReadHeader(dataSize, 1, 1)
	testReadHeader(dataSize+1, 1, 1)
	testReadHeader(dataSize+2, 1, 1)
	testReadHeader(dataSize+3, 1, 1)
	testReadHeader(dataSize+4, 1, 1)
}

func TestReadRecords(t *testing.T) {
	testReadRecords := func(dataSize, entryCount, totalRecords int) {
		totalHeader := rtest.Random(0, totalRecords*int(entrySize)+crypto.Extension)
		off := len(totalHeader) - (entryCount*int(entrySize) + crypto.Extension)
		if off < 0 {
			off = 0
		}
		expectedHeader := totalHeader[off:]

		buf := &bytes.Buffer{}
		buf.Write(rtest.Random(0, dataSize))                                          // pack blobs data
		buf.Write(totalHeader)                                                        // pack header
		rtest.OK(t, binary.Write(buf, binary.LittleEndian, uint32(len(totalHeader)))) // pack header length

		rd := bytes.NewReader(buf.Bytes())

		header, count, err := readRecords(rd, int64(rd.Len()), entryCount)
		rtest.OK(t, err)
		rtest.Equals(t, expectedHeader, header)
		rtest.Equals(t, totalRecords, count)
	}

	// basic
	testReadRecords(100, 1, 1)
	testReadRecords(100, 0, 1)
	testReadRecords(100, 1, 0)

	// header entries ~ eager entries
	testReadRecords(100, eagerEntries, eagerEntries-1)
	testReadRecords(100, eagerEntries, eagerEntries)
	testReadRecords(100, eagerEntries, eagerEntries+1)

	// file size == eager header load size
	eagerLoadSize := int((eagerEntries * entrySize) + crypto.Extension)
	headerSize := int(1*entrySize) + crypto.Extension
	dataSize := eagerLoadSize - headerSize - binary.Size(uint32(0))
	testReadRecords(dataSize-1, 1, 1)
	testReadRecords(dataSize, 1, 1)
	testReadRecords(dataSize+1, 1, 1)
	testReadRecords(dataSize+2, 1, 1)
	testReadRecords(dataSize+3, 1, 1)
	testReadRecords(dataSize+4, 1, 1)

	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			testReadRecords(dataSize, i, j)
		}
	}
}
