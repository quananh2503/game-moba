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
type SpellData struct {
	Speed   float32
	Radius  float32
	MaxTime float32
}

func GetSpellData(spell Spell) SpellData {
	switch spell {
	case SpellFireball:   return SpellData{Speed: 1200, Radius: 20, MaxTime: 0.45}
	case SpellToxicSpray: return SpellData{Speed: 800,  Radius: 10, MaxTime: 0.50}
	case SpellIceLance:   return SpellData{Speed: 500,  Radius: 15, MaxTime: 1.00}
	case SpellWindShear:  return SpellData{Speed: 1500, Radius: 15, MaxTime: 0.40}
	case SpellShockwave:  return SpellData{Speed: 750,  Radius: 0,  MaxTime: 1.00} // Radius = 0 vì dùng OBB
	default:              return SpellData{Speed: 0,    Radius: 10, MaxTime: 0.0}
	}
}

// 2. Dữ liệu cho VFX (Vùng hiệu ứng/Cảnh báo/Nổ)
type VFXData struct {
	Shape   VFXShape
	Radius  float32 // Dùng cho hình tròn
	W, H    float32 // Dùng cho hình chữ nhật
	MaxTime float32 // Thời gian tồn tại tối đa (để vẽ Animation)
}

func GetVFXData(vfx VFXType) VFXData {
	switch vfx {
	case VFXFireExplosion:   return VFXData{Shape: VFXShapeCircle, Radius: 100, MaxTime: 0.5}
	case VFXPoisonExplosion: return VFXData{Shape: VFXShapeCircle, Radius: 100, MaxTime: 0.5}
	case VFXIceExplosion:    return VFXData{Shape: VFXShapeCircle, Radius: 250, MaxTime: 0.5}
	case VFXToxicCloud:      return VFXData{Shape: VFXShapeCircle, Radius: 300, MaxTime: 8.0}
	case VFXIceTrail:        return VFXData{Shape: VFXShapeCircle, Radius: 15,  MaxTime: 2.0}
	case VFXIceWarning:      return VFXData{Shape: VFXShapeCircle, Radius: 250, MaxTime: 0.75}
	case VFXTornado:         return VFXData{Shape: VFXShapeCircle, Radius: 350, MaxTime: 6.0}
	case VFXBoulderWarning:  return VFXData{Shape: VFXShapeCircle, Radius: 180, MaxTime: 1.2}
	case VFXBoulderCrash:    return VFXData{Shape: VFXShapeCircle, Radius: 180, MaxTime: 0.6}
	case VFXFlamewall:       return VFXData{Shape: VFXShapeBox,    W: 150, H: 200, MaxTime: 4.0}
	default:                 return VFXData{Shape: VFXShapeCircle, Radius: 50, MaxTime: 1.0}
	}
}