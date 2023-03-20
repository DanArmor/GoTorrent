package bitfield

type Bitfield []byte

func (bf Bitfield) HasPiece(index int64) bool {
	byteIndex := index / 8
	offset := index % 8
	if byteIndex < 0 || byteIndex >= int64(len(bf)) {
		return false
	}
	return bf[byteIndex]>>uint(7 - offset)&1 != 0
}

func (bf Bitfield) SetPiece(index int64) {
	byteIndex := index / 8
	offset := index % 8
	if byteIndex < 0 || byteIndex >= int64(len(bf)) {
		return
	}
	bf[byteIndex] |= 1 << uint(7 - offset)
}