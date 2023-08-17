package goev

// Bitmap bitmap opt
type Bitmap struct {
	bytes []uint64
	size  int
}

// NewBitMap return an instance
func NewBitMap(size int) *Bitmap {
	bcap := (size + 63) / 64 // 计算最大倍数
	bm := &Bitmap{
		bytes: make([]uint64, bcap),
		size:  size,
	}
	return bm
}

// Size return size
func (b *Bitmap) Size() int {
	return b.size
}

// Set the specified bit to 1, ensuring it does not exceed the size
func (b *Bitmap) Set(pos int) bool {
	if pos < b.size {
		b.bytes[pos>>6] |= 1 << (pos & 0x3f)
		return true
	}
	return false
}

// UnSet set the specified bit to 0, ensuring it does not exceed the size
func (b *Bitmap) UnSet(pos int) bool {
	if pos < b.size {
		b.bytes[pos>>6] &= ^(1 << (pos & 0x3f))
		return true
	}
	return false
}

// IsSet returns whether the specified position is 1
func (b *Bitmap) IsSet(pos int) bool {
	if pos < b.size {
		if b.bytes[pos>>6]&(1<<(pos&0x3f)) > 0 {
			return true
		}
	}
	return false
}

// FirstUnSet returns the position of the first occurrence of 0
func (b *Bitmap) FirstUnSet() int {
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
