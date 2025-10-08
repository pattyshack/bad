package common

type BitsAppender struct {
	result []byte

	// lower bits are free, upper numBufferedBits bits are filled.
	buffer          byte
	numBufferedBits int
}

func (ba *BitsAppender) AppendSlice(s []byte, bitOffset int, bitSize int) {
	s = s[(bitOffset / 8):]
	bitOffset %= 8

	if ba.numBufferedBits == 0 && bitOffset == 0 {
		alignedSize := bitSize / 8
		bitSize %= 8
		ba.result = append(ba.result, s[:alignedSize]...)
		s = s[alignedSize:]
	}

	if bitSize == 0 {
		return
	}

	for _, b := range s {
		numUsableBits := 8 - bitOffset
		if numUsableBits > bitSize {
			numUsableBits = bitSize
		}

		if bitOffset > 0 {
			b <<= bitOffset
			bitOffset = 0
		}

		ba.appendByte(b, numUsableBits)
		bitSize -= numUsableBits

		if bitSize == 0 {
			break
		}
	}
}

// bits are left aligned (bits offset = 0)
func (ba *BitsAppender) appendByte(b byte, bitSize int) {
	if bitSize == 8 && ba.numBufferedBits == 0 {
		ba.result = append(ba.result, b)
		return
	}

	// clear bottom bits
	b >>= (8 - bitSize)
	b <<= (8 - bitSize)

	ba.buffer |= b >> ba.numBufferedBits
	ba.numBufferedBits += bitSize

	if ba.numBufferedBits >= 8 {
		ba.result = append(ba.result, ba.buffer)
		ba.numBufferedBits -= 8
		ba.buffer = b << (bitSize - ba.numBufferedBits)
	}
}

func (ba *BitsAppender) Finalize() []byte {
	if ba.numBufferedBits > 0 {
		ba.result = append(ba.result, ba.buffer)
		ba.buffer = 0
		ba.numBufferedBits = 0
	}

	return ba.result
}
