package common

import (
	"testing"

	"github.com/pattyshack/gt/testing/expect"
	"github.com/pattyshack/gt/testing/suite"
)

type BitsAppenderSuite struct{}

func TestBitsAppender(t *testing.T) {
	suite.RunTests(t, &BitsAppenderSuite{})
}

func (BitsAppenderSuite) TestByteAligned(t *testing.T) {
	ba := &BitsAppender{}

	ba.AppendSlice([]byte(" Hello sam"), 8, 5*8)
	expect.Equal(t, "Hello", string(ba.result))
	expect.Equal(t, 0, ba.buffer)
	expect.Equal(t, 0, ba.numBufferedBits)

	ba.AppendSlice([]byte("   world! championship"), 2*8, 7*8)
	expect.Equal(t, "Hello world!", string(ba.result))
	expect.Equal(t, 0, ba.buffer)

	expect.Equal(t, "Hello world!", string(ba.Finalize()))
}

func (BitsAppenderSuite) TestMisalignedBits(t *testing.T) {
	ba := &BitsAppender{}

	ba.AppendSlice([]byte{0b00_1101_11}, 2, 4)
	expect.Equal(t, 0, len(ba.result))
	expect.Equal(t, 4, ba.numBufferedBits)
	expect.Equal(t, 0b1101_0000, ba.buffer)

	ba.AppendSlice([]byte{0b0_11_01010}, 1, 2)
	expect.Equal(t, 0, len(ba.result))
	expect.Equal(t, 6, ba.numBufferedBits)
	expect.Equal(t, 0b1101_11_00, ba.buffer)

	ba.AppendSlice([]byte{0b000_10_110, 0b01_101010}, 3, 7)
	expect.Equal(t, []byte{0b1101_11_10}, ba.result)
	expect.Equal(t, 5, ba.numBufferedBits)
	expect.Equal(t, 0b110_01_000, ba.buffer)

	ba.AppendSlice([]byte{0b01_000000}, 0, 2)
	expect.Equal(t, []byte{0b1101_11_10}, ba.result)
	expect.Equal(t, 7, ba.numBufferedBits)
	expect.Equal(t, 0b110_01_01_0, ba.buffer)

	ba.AppendSlice([]byte{0xff, 0x80, 0b0_1001_101}, 0, 8+8+5)
	expect.Equal(t, []byte{0b1101_11_10, 0b110_01_01_1, 0xff, 0}, ba.result)
	expect.Equal(t, 4, ba.numBufferedBits)
	expect.Equal(t, 0b1001_0000, ba.buffer)

	ba.AppendSlice([]byte{0xbe, 0xef, 0b1111_0110, 0xab, 0xcd}, 8+8+4, 4+8+8)
	expect.Equal(
		t,
		[]byte{0b1101_11_10, 0b110_01_01_1, 0xff, 0, 0b1001_0110, 0xab, 0xcd},
		ba.result)
	expect.Equal(t, 0, ba.numBufferedBits)
	expect.Equal(t, 0, ba.buffer)

	ba.AppendSlice([]byte{0b00000_1_00}, 5, 1)
	expect.Equal(
		t,
		[]byte{0b1101_11_10, 0b110_01_01_1, 0xff, 0, 0b1001_0110, 0xab, 0xcd},
		ba.result)
	expect.Equal(t, 1, ba.numBufferedBits)
	expect.Equal(t, 0b1_0000000, ba.buffer)

	expect.Equal(
		t,
		[]byte{0b1101_11_10, 0b110_01_01_1, 0xff, 0, 0b1001_0110, 0xab, 0xcd, 0x80},
		ba.Finalize())
}
