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
	expect.Equal(t, 0b0000_1101, ba.buffer)

	ba.AppendSlice([]byte{0b01010_11_0}, 1, 2)
	expect.Equal(t, 0, len(ba.result))
	expect.Equal(t, 6, ba.numBufferedBits)
	expect.Equal(t, 0b00_11_1101, ba.buffer)

	ba.AppendSlice([]byte{0b110_10_000, 0b101010_01}, 3, 7)
	expect.Equal(t, []byte{0b10_11_1101}, ba.result)
	expect.Equal(t, 5, ba.numBufferedBits)
	expect.Equal(t, 0b000_01_110, ba.buffer)

	ba.AppendSlice([]byte{0b000000_01}, 0, 2)
	expect.Equal(t, []byte{0b10_11_1101}, ba.result)
	expect.Equal(t, 7, ba.numBufferedBits)
	expect.Equal(t, 0b0_01_01_110, ba.buffer)

	ba.AppendSlice([]byte{0xff, 0x01, 0b101_1001_0}, 0, 8+8+5)
	expect.Equal(t, []byte{0b10_11_1101, 0b1_01_01_110, 0xff, 0}, ba.result)
	expect.Equal(t, 4, ba.numBufferedBits)
	expect.Equal(t, 0b0000_1001, ba.buffer)

	ba.AppendSlice([]byte{0xbe, 0xef, 0b0110_1111, 0xab, 0xcd}, 8+8+4, 4+8+8)
	expect.Equal(
		t,
		[]byte{0b10_11_1101, 0b1_01_01_110, 0xff, 0, 0b0110_1001, 0xab, 0xcd},
		ba.result)
	expect.Equal(t, 0, ba.numBufferedBits)
	expect.Equal(t, 0, ba.buffer)

	ba.AppendSlice([]byte{0b00_1_00000}, 5, 1)
	expect.Equal(
		t,
		[]byte{0b10_11_1101, 0b1_01_01_110, 0xff, 0, 0b0110_1001, 0xab, 0xcd},
		ba.result)
	expect.Equal(t, 1, ba.numBufferedBits)
	expect.Equal(t, 0b0000000_1, ba.buffer)

	expect.Equal(
		t,
		[]byte{0b10_11_1101, 0b1_01_01_110, 0xff, 0, 0b0110_1001, 0xab, 0xcd, 0x01},
		ba.Finalize())
}
