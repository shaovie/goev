package goev

// RingBuffer using value array, only suitable for tiny struct
type RingBuffer[T any] struct {
	size   int
	head   int
	tail   int
	len    int
	zero   T
	buffer []T
}

// NewRingBuffer return an instance
func NewRingBuffer[T any](initCap int) *RingBuffer[T] {
	return &RingBuffer[T]{
		buffer: make([]T, initCap),
		size:   initCap,
		head:   0,
		tail:   0,
		len:    0,
	}
}

// IsEmpty return is empty or not
func (rb *RingBuffer[T]) IsEmpty() bool {
	return rb.len == 0
}

// IsFull return is full or not
func (rb *RingBuffer[T]) IsFull() bool {
	return rb.len == rb.size
}

// Size return the latest buffer size
func (rb *RingBuffer[T]) Size() int {
	return rb.size
}

// Len reutrn the current buffer length
func (rb *RingBuffer[T]) Len() int {
	return rb.len
}

// PushBack an item
func (rb *RingBuffer[T]) PushBack(data T) {
	if rb.len == rb.size {
		rb.grow()
	}
	rb.buffer[rb.tail] = data
	rb.tail = (rb.tail + 1) % rb.size // TODO optimize to & operator
	rb.len++
}

// PopFront an item
func (rb *RingBuffer[T]) PopFront() (data T, ok bool) {
	if rb.len == 0 {
		return
	}
	data = rb.buffer[rb.head]
	rb.buffer[rb.head] = rb.zero // Quickly release memory
	rb.head = (rb.head + 1) % rb.size
	rb.len--
	ok = true
	return
}

// PushFront an item
func (rb *RingBuffer[T]) PushFront(data T) {
	if rb.len == rb.size {
		rb.grow()
	}
	rb.head = (rb.size + rb.head - 1) % rb.size // prev
	rb.buffer[rb.head] = data
	rb.len++
}

func (rb *RingBuffer[T]) grow() {
	newCapacity := rb.size * 2
	newBuffer := make([]T, newCapacity)

	var n int
	if rb.tail > rb.head {
		n = copy(newBuffer, rb.buffer[rb.head:rb.tail])
	} else {
		n = copy(newBuffer, rb.buffer[rb.head:])
		n += copy(newBuffer[n:], rb.buffer[:rb.tail])
	}

	rb.buffer = newBuffer
	rb.size = newCapacity
	rb.head = 0
	rb.tail = n
}
