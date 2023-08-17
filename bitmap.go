package goev

type Bitmap struct {
	bytes []uint64
	size  int
}

func NewBitMap(size int) *Bitmap {
	bcap := (size + 63) / 64 // 计算最大倍数
	bm := &Bitmap{
		bytes: make([]uint64, bcap),
		size:  size,
	}
	return bm
}
func (b *Bitmap) Size() int {
	return b.size
}
func (b *Bitmap) Set(pos int) bool {
	if pos < b.size {
		b.bytes[pos>>6] |= 1 << (pos & 0x3f)
		return true
	}
	return false
}
func (b *Bitmap) Unset(pos int) bool {
	if pos < b.size {
		b.bytes[pos>>6] &= ^(1 << (pos & 0x3f))
		return true
	}
	return false
}
func (b *Bitmap) IsSet(pos int) bool {
	if pos < b.size {
		if b.bytes[pos>>6]&(1<<(pos&0x3f)) > 0 {
			return true
		}
	}
	return false
}
func (b *Bitmap) firstUnSet() int {
	for i, val := range b.bytes {
		if ^val == 0 {
			continue
		}
		for j := 0; j < 64 && (i*64+j) < b.size; j++ {
			if val&(1<<j) == 0 {
				return i*64 + j
			}
		}
	}
	return -1
}
