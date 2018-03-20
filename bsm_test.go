// test parsing of BSM files
package main

import (
	"bytes"
	"strconv"
	"testing"
)

func Test_bytesToUint32(t *testing.T) {
	testdata := map[uint32][]byte{
		0:        []byte{0x00},
		1:        []byte{0x01},
		2:        []byte{0x00, 0x02},
		3:        []byte{0x00, 0x00, 0x00, 0x03},
		256:      []byte{0x01, 0x00},
		257:      []byte{0x01, 0x01},
		258:      []byte{0x00, 0x01, 0x02},
		65536:    []byte{0x00, 0x01, 0x00, 0x00},
		16777218: []byte{0x01, 0x00, 0x00, 0x02},
	}
	for k, v := range testdata {
		number, err := bytesToUint32(v)
		if err != nil {
			t.Error(err.Error())
		}
		if number != k {
			t.Error("could not decode " + strconv.Itoa(int(k)) + " correctly, got " + strconv.Itoa(int(number)))
		}
	}
	_, err := bytesToUint32([]byte{0xff, 0x01, 0xac, 0xb4, 0x2c})
	if err == nil {
		t.Error("did not catch overflow")
	}
}

func TestRecordsFromFile(t *testing.T) {
	data := []byte{0x00}
	err := RecordsFromFile(bytes.NewBuffer(data))
	if err == nil {
		t.Error("one byte record should yield an error")
	}
}

// fixed sized tokens
func Test_determineTokenSize_fixed(t *testing.T) {
	testData := map[byte]int{
		0x13: 7,  // trailer token
		0x14: 19, // 32 bit header token
		0x24: 37, // 32 bit subject token
		0x27: 6,  // 32 bit return token
		0x2a: 5,  // in_addr token
		0x2b: 21, // ip token
		0x2c: 3,  // iport token
		0x3e: 26, // 32bit attribute token
		0x52: 9,  // exit token
		0x72: 10, // 64 bit return token
		0x73: 30, // 64bit attribute token
		0x74: 27, // 64 bit header token
		0x75: 41, // 64 bit subject token
		0x7e: 18, // expanded in_addr token
	}
	for tokenID, count := range testData {
		dcount, _, err := determineTokenSize([]byte{tokenID})
		if err != nil {
			t.Error(err)
		}
		if dcount != count {
			t.Errorf("token size does not match expectation of token ID 0x%x", tokenID)
		}
	}
}

func Test_determineTokenSize_file_token(t *testing.T) {
	testData := []byte{}

	// missing token ID
	_, more, err := determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 1 {
		t.Error("expected 1 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token ID, bot no more
	testData = []byte{0x11}
	_, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 10 {
		t.Error("expected 10 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token (in terms of size)
	testData = []byte{0x11, // token ID
		0x00, 0x01, 0x02, 0x03, // seconds
		0x04, 0x05, 0x06, 0x07, // microseconds
		0x23, 0xf8, // file name length
	}
	size, more, err := determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 0 {
		t.Error("expected 0 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}
	if size != (11 + 9208 + 1) { // 11 inital bytes + file name length (from hex) + NUL
		t.Error("wrong size: expected " + strconv.Itoa(11+9208+1) + ", got " + strconv.Itoa(size))
	}

}

func Test_determineTokenSize_expanded_32bit_subject_token(t *testing.T) {
	testData := []byte{}

	// missing token ID
	_, more, err := determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 1 {
		t.Error("expected 1 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token ID, bot no more
	testData = []byte{0x7a}
	_, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 33 {
		t.Error("expected 33 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token (in terms of size)
	testData = []byte{0x7a, // token ID
		0x00, 0x01, 0x02, 0x03, // audit user ID
		0x00, 0x01, 0x02, 0x03, // effective user ID
		0x00, 0x01, 0x02, 0x03, // effective group ID
		0x00, 0x01, 0x02, 0x03, // real user ID
		0x00, 0x01, 0x02, 0x03, // real group ID
		0x00, 0x01, 0x02, 0x03, // process ID
		0x00, 0x01, 0x02, 0x03, // audit session ID
		0x00, 0x01, 0x02, 0x03, // terminal port ID
		0x00,                   // length of address
		0x00, 0x01, 0x02, 0x03, // IPv4
	}
	size, more, err := determineTokenSize(testData)
	if err == nil {
		t.Error("expected an error on invalid address length")
	}
	testData[33] = 4 // IPv4
	size, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 0 {
		t.Error("expected 0 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}
	expSize := 38
	if size != expSize {
		t.Error("wrong size: expected " + strconv.Itoa(expSize) + ", got " + strconv.Itoa(size))
	}

}

func Test_determineTokenSize_expanded_64bit_subject_token(t *testing.T) {
	testData := []byte{}

	// missing token ID
	_, more, err := determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 1 {
		t.Error("expected 1 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token ID, bot no more
	testData = []byte{0x7c}
	_, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	moreBytes := 37
	if more != moreBytes {
		t.Error("expected " + strconv.Itoa(moreBytes) + " bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token (in terms of size)
	testData = []byte{0x7c, // token ID
		0x00, 0x01, 0x02, 0x03, // audit user ID
		0x00, 0x01, 0x02, 0x03, // effective user ID
		0x00, 0x01, 0x02, 0x03, // effective group ID
		0x00, 0x01, 0x02, 0x03, // real user ID
		0x00, 0x01, 0x02, 0x03, // real group ID
		0x00, 0x01, 0x02, 0x03, // process ID
		0x00, 0x01, 0x02, 0x03, // audit session ID
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, // terminal port ID
		0x00,                   // length of address
		0x00, 0x01, 0x02, 0x03, // IPv4
	}
	size, more, err := determineTokenSize(testData)
	if err == nil {
		t.Error("expected an error on invalid address length")
	}
	testData[37] = 4 // IPv4
	size, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 0 {
		t.Error("expected 0 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}
	expSize := 42
	if size != expSize {
		t.Error("wrong size: expected " + strconv.Itoa(expSize) + ", got " + strconv.Itoa(size))
	}

}

func Test_determineTokenSize_expanded_32bit_header_token(t *testing.T) {
	testData := []byte{}

	// missing token ID
	_, more, err := determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 1 {
		t.Error("expected 1 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token ID, bot no more
	testData = []byte{0x15}
	_, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	moreBytes := 14
	if more != moreBytes {
		t.Error("expected " + strconv.Itoa(moreBytes) + " bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token (in terms of size)
	testData = []byte{0x15, // token ID
		0x00, 0x01, 0x02, 0x03, // number of bytes in record
		0x00, 0x01, // record version number
		0x00, 0x01, // event type
		0x00, 0x01, // event modifier / sub-type
		0x00, 0x01, 0x02, 0x03, // host address type/length
		0x00, 0x01, 0x02, 0x03, // IPv4
		0x00, 0x01, 0x02, 0x03, // seconds timestamp
		0x00, 0x01, 0x02, 0x03, // nanosecond timestamp
	}
	size, more, err := determineTokenSize(testData)
	if err == nil {
		t.Error("expected an error on invalid address type")
	}
	testData[10] = 0
	testData[11] = 0
	testData[12] = 0
	testData[13] = 4 // IPv4
	size, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 0 {
		t.Error("expected 0 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}
	expSize := 27
	if size != expSize {
		t.Error("wrong size: expected " + strconv.Itoa(expSize) + ", got " + strconv.Itoa(size))
	}

}

func Test_determineTokenSize_expanded_64bit_header_token(t *testing.T) {
	testData := []byte{}

	// missing token ID
	_, more, err := determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 1 {
		t.Error("expected 1 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token ID, bot no more
	testData = []byte{0x15}
	_, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	moreBytes := 14
	if more != moreBytes {
		t.Error("expected " + strconv.Itoa(moreBytes) + " bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}

	// correct token (in terms of size)
	testData = []byte{0x79, // token ID
		0x00, 0x01, 0x02, 0x03, // number of bytes in record
		0x00, 0x01, // record version number
		0x00, 0x01, // event type
		0x00, 0x01, // event modifier / sub-type
		0x00, 0x01, 0x02, 0x03, // host address type/length
		0x00, 0x01, 0x02, 0x03, // IPv4
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, // seconds timestamp
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, // nanosecond timestamp
	}
	size, more, err := determineTokenSize(testData)
	if err == nil {
		t.Error("expected an error on invalid address type")
	}
	testData[10] = 0
	testData[11] = 0
	testData[12] = 0
	testData[13] = 4 // IPv4
	size, more, err = determineTokenSize(testData)
	if err != nil {
		t.Error(err)
	}
	if more != 0 {
		t.Error("expected 0 bytes more to read, but only " + strconv.Itoa(more) + " were requested")
	}
	expSize := 35
	if size != expSize {
		t.Error("wrong size: expected " + strconv.Itoa(expSize) + ", got " + strconv.Itoa(size))
	}

}

func TestParseHeaderToken32bit(t *testing.T) {
	data := []byte{0x14, // token ID \
		0x00, 0x00, 0x00, 0x38, // record byte number \
		0x0b, 0xaf, // version number
		0xc8, 0x00, // event type
		0x00, 0x5a, // event sub-type / modifier
		0x9a, 0xc2, 0xe6, 0x00, // seconds
		0x00, 0x03, 0x01, 0x28, // nanoseconds
	}
	token, err := ParseHeaderToken32bit(data)
	if err != nil {
		t.Error(err.Error())
	}
	if token.TokenID != 0x14 {
		t.Error("wrong token ID")
	}
	if token.RecordByteCount != 56 {
		t.Error("wrong record byte count, got " + strconv.Itoa(int(token.RecordByteCount)))
	}
	if token.VersionNumber != 2991 {
		t.Error("wrong version number")
	}
	if token.EventType != 51200 {
		t.Error("wrong event type")
	}
	if token.EventModifier != 90 {
		t.Error("wrong event modifier")
	}
	if token.Seconds != 2596464128 {
		t.Error("wrong number of seconds")
	}
	if token.NanoSeconds != 196904 {
		t.Error("wrong number of nanoseconds")
	}

}
