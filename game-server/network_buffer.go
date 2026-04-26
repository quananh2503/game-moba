package main

import "math"

// ==========================================
// ĐÓNG GÓI DỮ LIỆU (Dùng ở Server)
// ==========================================

type PacketWriter struct {
	Buf []byte
	Pos int // Dùng biến này để quản lý vị trí thay cho len()
}

func NewPacketWriter(capacity int) *PacketWriter {
	// CHIẾU THỨC 1: Cấp phát full length ngay từ đầu
	// Điều này giúp loại bỏ hoàn toàn logic "len < cap" của append
	return &PacketWriter{
		Buf: make([]byte, capacity),
		Pos: 0,
	}
}

func (w *PacketWriter) WriteUint8(v uint8) {
	// CHIẾU THỨC 2: Gán trực tiếp qua Index
	// Trình biên dịch sẽ Inline hàm này và biến nó thành 1 lệnh MOV duy nhất
	w.Buf[w.Pos] = v
	w.Pos++
}

func (w *PacketWriter) WriteUint16(v uint16) {
	// Ghi trực tiếp 2 bytes, không thông qua trung gian
	w.Buf[w.Pos] = byte(v >> 8)
	w.Buf[w.Pos+1] = byte(v)
	w.Pos += 2
}

func (w *PacketWriter) WriteUint32(v uint32) {
	b := w.Buf[w.Pos : w.Pos+4]
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
	w.Pos += 4
}

func (w *PacketWriter) WriteFloat32(v float32) {
	i := math.Float32bits(v)
	// Tái sử dụng WriteUint32
	w.WriteUint32(i)
}

func (w *PacketWriter) WriteBytes(payload []byte) {
	// l := int(ev.Len)
// if l > 32 { l = 32 }
	// copy() của Go thực chất là lệnh memmove trong Assembly
	// Nó nhanh hơn mọi vòng lặp thủ công
	n := copy(w.Buf[w.Pos:], payload)
	w.Pos += n
}

func (w *PacketWriter) Bytes() []byte {
	// Trả về slice ảo từ 0 đến Pos. Zero-copy.
	return w.Buf[:w.Pos]
}

func (w *PacketWriter) Reset() {
	// Chỉ cần đưa con trỏ về 0. Cực nhanh.
	w.Pos = 0
}
// ==========================================
// GIẢI NÉN DỮ LIỆU (Dùng ở Client)
// ==========================================
type PacketReader struct {
	Buf    []byte
	Offset int
}

func NewPacketReader(data []byte) *PacketReader {
	return &PacketReader{Buf: data, Offset: 0}
}

func (r *PacketReader) ReadUint8() uint8 {
	v := r.Buf[r.Offset]
	r.Offset += 1
	return v
}

func (r *PacketReader) ReadUint16() uint16 {
	v := uint16(r.Buf[r.Offset])<<8 | uint16(r.Buf[r.Offset+1])
	r.Offset += 2
	return v
}

func (r *PacketReader) ReadFloat32() float32 {
	i := uint32(r.Buf[r.Offset])<<24 | uint32(r.Buf[r.Offset+1])<<16 | uint32(r.Buf[r.Offset+2])<<8 | uint32(r.Buf[r.Offset+3])
	r.Offset += 4
	return math.Float32frombits(i)
}
func (r *PacketReader) ReadUInt32() uint32 {
	i := uint32(r.Buf[r.Offset])<<24 | uint32(r.Buf[r.Offset+1])<<16 | uint32(r.Buf[r.Offset+2])<<8 | uint32(r.Buf[r.Offset+3])
	r.Offset += 4
	return i
}