package goev

type Bitmap struct {
	bytes []uint8
	size  int64
}

func NewBitMap(size int64) *Bitmap {
	size = (size + 7) / 8 * 8 // 计算最大倍数
	bm := &Bitmap{
		bytes: make([]uint8, size/8, size/8),
		size:  size,
	}
	return bm
}
func (b *Bitmap) Set(pos int64) bool {
    if pos < b.size {
        b.bytes[pos>>3] |= 1 << (pos & 0x07)
        return true
    }
    return false
}
func (b *Bitmap) Unset(pos int64) bool {
	if pos < b.size {
        b.bytes[pos>>3] &= ^(1 << (pos & 0x07))
        return true
    }
    return false
}
func (b *Bitmap) IsSet(pos int64) bool {
    if pos < b.size {
        if b.bytes[pos>>3]&(1<<(pos&0x07)) > 0 {
            return true
        }
    }
	return false
}
