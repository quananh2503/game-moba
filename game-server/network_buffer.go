package main

import "math"

// ==========================================
// ĐÓNG GÓI DỮ LIỆU (Dùng ở Server)
// ==========================================
type PacketWriter struct {
	Buf []byte
}

func NewPacketWriter(capacity int) *PacketWriter {
	return &PacketWriter{Buf: make([]byte, 0, capacity)}
}

func (w *PacketWriter) WriteUint8(v uint8) {
	w.Buf = append(w.Buf, v)
}

func (w *PacketWriter) WriteUint16(v uint16) {
	w.Buf = append(w.Buf, byte(v>>8), byte(v))
}
func (w *PacketWriter) WriteUint32(v uint32) {
	w.Buf = append(w.Buf, byte(v>>24), byte(v>>16),byte(v>>8),byte(v))
}

// Float32 là thứ khó gửi nhất, hàm này chuyển Float thành uint32 để gửi an toàn
func (w *PacketWriter) WriteFloat32(v float32) {
	i := math.Float32bits(v)
	w.Buf = append(w.Buf, byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
}
func (w *PacketWriter) WriteBytes(payload []byte) {
	w.Buf = append(w.Buf, payload...)
}

func (w *PacketWriter) Bytes() []byte {
	return w.Buf
}

func (w *PacketWriter) Reset() {
	w.Buf = w.Buf[:0]
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