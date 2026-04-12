package main

import (
	"math"
)

type ReliableEvent struct {
	SeqID   uint16
	Type    byte
	Payload []byte
	HasData bool
}
type ReliableReciever struct {
	ExpectedSeq uint16
	buffer      [256]ReliableEvent
	ackQueue    []uint16
}

func NewReliableChannel() *ReliableReciever {
	return &ReliableReciever{
		ExpectedSeq: 1,
		ackQueue:    make([]uint16, 0, 256),
	}
}
func (r *ReliableReciever) RecieveEvent(seq uint16, evType byte, payload []byte) []ReliableEvent {
	if seq < r.ExpectedSeq {
		r.ackQueue = append(r.ackQueue, seq)
		return nil
	}
	if seq-r.ExpectedSeq > 255 {
		//////fmt.println("Cảnh báo: Rớt mạng quá nặng, gói tin bay mất quá nhiều!")
		return nil
	}
	idx := seq&255
	if !r.buffer[idx].HasData{
		r.buffer[idx]=ReliableEvent{
			SeqID: seq,
			Type: evType,
			Payload: payload,
			HasData: true,
		}
	}
	r.ackQueue=append(r.ackQueue, seq )
	var readyToProcess []ReliableEvent
	for {
		idx := r.ExpectedSeq &255
		if !r.buffer[idx].HasData{
			break
		}
		readyToProcess=append(readyToProcess, r.buffer[idx])
		r.buffer[idx].HasData=false
		r.ExpectedSeq++
	}
	return readyToProcess
}
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
func (r *PacketReader)ReadUint32() uint32{
	i := uint32(r.Buf[r.Offset])<<24 | uint32(r.Buf[r.Offset+1])<<16 | uint32(r.Buf[r.Offset+2])<<8 | uint32(r.Buf[r.Offset+3])
	r.Offset += 4
	return i
}

func ( r *PacketReader)ReadBytes(len int)[]byte{
	payload := make([]byte,len)
	copy(payload,r.Buf[r.Offset: r.Offset+len])
	r.Offset +=len
	return payload
}
func( r *PacketReader)HasMore() bool{
	return r.Offset < len(r.Buf)
}
type PacketWriter struct {
	Buf []byte
}

func NewPacketWriter( payload []byte) *PacketWriter {
	w:= &PacketWriter{Buf: payload}
	return  w
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