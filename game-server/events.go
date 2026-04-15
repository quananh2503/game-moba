package main

import (
	def "game/pkg"
	"math"
)
const MaxEventPayload = 32
type RawEvent struct{
	Type byte 
	Len uint8
	Payload [MaxEventPayload]byte 
}
func NewEvent(evType byte) RawEvent{
	return RawEvent{
		Type: evType,
		Len:0,
	}
}
func (e *RawEvent) WriteUint8(v uint8) {
	e.Payload[e.Len] = v
	e.Len++
}

func (e *RawEvent) WriteUint16(v uint16) {
	// Dịch bit gọn gàng và giấu kín trong hàm này
	e.Payload[e.Len] = byte(v >> 8)
	e.Payload[e.Len+1] = byte(v)
	e.Len += 2
}

func (e *RawEvent) WriteUint32(v uint32) {
	e.Payload[e.Len] = byte(v >> 24)
	e.Payload[e.Len+1] = byte(v >> 16)
	e.Payload[e.Len+2] = byte(v >> 8)
	e.Payload[e.Len+3] = byte(v)
	e.Len += 4
}

// WriteFloat32 cực kỳ hữu dụng cho tọa độ X, Y nếu bạn không ép về uint16
func (e *RawEvent) WriteFloat32(v float32) {
	bits := math.Float32bits(v)
	e.WriteUint32(bits)
}

// Hỗ trợ ghi mảng byte (Ví dụ: chuỗi tên người chơi)
func (e *RawEvent) WriteBytes(b []byte) {
	n := copy(e.Payload[e.Len:], b)
	e.Len += uint8(n)
}

func NewRemoveEntityEvent(entityID Entity) RawEvent{
	ev := RawEvent{
		Type: def.EventRemoveEntity,
	}
	ev.WriteUint32(uint32(entityID))
	return ev
}
// func NewSpawnVFXCircleEvent(vfxType def.VFXType, x, y float32, radius float32, duration float32) RawEvent {
// 	ev := RawEvent{Type: def.EventSpawnVFX}
// 	ev.WriteUint8(uint8(vfxType))
// 	ev.WriteUint8(uint8(def.VFXShapeCircle)) // Báo cho Client đây là hình Tròn
// 	ev.WriteFloat32(x)
// 	ev.WriteFloat32(y)
// 	ev.WriteFloat32(radius)
// 	ev.WriteFloat32(duration)
// 	return ev
// }
func NewSpawnVFX(vfxType def.VFXType,e Entity, x, y float32, angle uint16) RawEvent {
	ev := RawEvent{Type: def.EventSpawnVFX}
	ev.WriteUint32(uint32(e))
	ev.WriteUint8(uint8(vfxType))
	ev.WriteFloat32(x)
	ev.WriteFloat32(y)
	ev.WriteUint16(angle) // Tường có thể bị xoay chéo
	return ev
}
func NewUpdateProjectileEvent(entityID Entity, x, y float32, newAngle uint16) RawEvent {
	ev := RawEvent{
		Type: def.EventUpdateProjectile,
	}
	ev.WriteUint32(uint32(entityID))
	ev.WriteFloat32(x)
	ev.WriteFloat32(y)
	ev.WriteUint16(newAngle)
	return ev
}
func NewSpawnProjectEvent(entityID Entity,SpellID def.Spell, x,y float32,angle uint16) RawEvent{
	ev :=RawEvent{
		Type: def.EventSpawnProjectile,
	}
	ev.WriteUint32(uint32(entityID))
	ev.WriteUint8(uint8(SpellID))
	ev.WriteFloat32(x)
	ev.WriteFloat32(y)
	ev.WriteUint16(angle)
	return ev 
}

func NewSpawnVisual(entity Entity, visualID uint8 , x,y float32, angle uint16) RawEvent{
	ev :=RawEvent{
	}
	ev.WriteUint32(uint32(entity))
	ev.WriteUint8(visualID)
	ev.WriteFloat32(x)
	ev.WriteFloat32(y)
	ev.WriteUint16(angle)
	return ev
}