package def

import "time"
type Effect uint8 
const (
	EffectNone Effect = iota 
	EffectHeal 
	EffectFire 
	EffectPoision
	EffectStun 
	EffectStatBuff
	EffectCount
)
func (e Effect)IsValid()bool{
	return e >EffectNone && e < EffectCount
}
type Input uint8
const (
	InputW     Input = 1 << iota
	InputS     
	InputA     
	InputD     
	InputLeftClick 
	InputRightClick 
	InputSpace 
	InputQ 
)
func (i Input) IsSet(flag Input) bool { return i&flag != 0 }

type Cast uint8 
const (
	CastQ Cast = 1 << iota
	CastLM
	CastRM
	CastDash
)
func (c Cast) IsSet(flag Cast) bool { return c&flag != 0 }

type Shape uint8
const (
	ShapeNone   Shape = iota
	ShapeCircle           // 1
	ShapeBox              // 2 (AABB)
	ShapeOBB              // 3 (Xoay được - Oriented Bounding Box)
)

// --- 2. CHỈ SỐ CƠ BẢN (BASE STATS) ---
const (
	StatBaseHP    float32 = 1000.0
	StatBaseAD    float32 = 0.0
	StatBaseSpeed float32 = 500.0
	
	DashCooldown = 8 * time.Second
	DashDistance = 300.0
)

// Loại chỉ số (Dùng Bitmask để đánh dấu Stat nào bị thay đổi - Dirty Flags)
type Stat uint8
const (
	StatSpeed Stat = 1 << iota // 1
	StatAD                         // 2
	StatHP                         // 4
)

// --- 3. CHIÊU THỨC (SPELLS) ---
type Spell uint8
const (
	SpellNone       Spell = iota
	SpellFireball           // 1
	SpellIceLance           // 2
	SpellToxicSpray         // 3
	SpellWindShear
	SpellShockwave   
	SpellBoulderfall 
)

// --- 4. GIAO THỨC MẠNG (NETWORK EVENTS) ---
const (
	EventSpawnProjectile byte = 1
	EventHitPlayer       byte = 2
	EventWelcome         byte = 3
	EventMatchEnd        byte = 4
	EventSendMap         byte = 5
	EventRemoveEntity    byte = 6
	EventUpdateProjectile byte = 7
	EventSpawnVFX        byte = 10 // Event dùng chung cho mọi loại VFX
)

// --- 5. HIỆU ỨNG HÌNH ẢNH (VFX) ---

// Hình dạng của VFX (Để Client biết cách vẽ)
type VFXShape uint8
const (
	VFXShapeCircle VFXShape = iota
	VFXShapeBox    
)

// Loại hiệu ứng (Màu sắc, Sprite, Logic vẽ)
type VFXType uint8
const (
	VFXNone                VFXType = iota
	VFXFireExplosion               // 1
	VFXPoisonExplosion             // 2
	VFXIceExplosion                // 3
	VFXFlamewall                   // 4
	VFXToxicCloud                  // 5
	VFXIceTrail                    // 6
	VFXIceWarning                  // 7 (Hết lỗi trùng ID 3!)
	VFXLightningStrike             // 8
	VFXTornado
	

	VFXShockwave     
	VFXBoulderWarning 
	VFXBoulderCrash  
)

// const (
// 	ShapeTypeCircle = 1
// 	ShapeTypeBox = 2
// )