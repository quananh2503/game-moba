package main

import def "game/pkg"

// --- NHÓM VẬT LÝ ---
type Transform struct { X, Y float32; Angle uint16 }
type Velocity  struct { Dx, Dy float32 }
type Collider  struct { Radius float32; Height,Width float32; ShapeType def.Shape }

// --- NHÓM CHỈ SỐ ---
type Vitality struct { 
    HP, MaxHP, Shield float32 
}

type StatSheet struct {
    BaseSpeed, CurrSpeed float32
    BaseAD, CurrAD       float32
    Dirties def.Stat // Cờ báo hiệu cần tính lại
    Modifiers []StatModifier
    Armor float32

}

// --- NHÓM CHIẾN ĐẤU ---
type SkillCooldowns struct {
    LMB, RMB, Space float32
}

type DamageDealer struct {
    Effects []EffectPayload
    Amount   float32
    SourceID Entity
    Type     uint8 // 1: Fire, 2: Toxic, v.v.
    DestroyOnHit bool
    TickRate float32

    TargetCount  uint8       // Dùng uint8 vì 50 < 255
	Targets      [50]Entity
	TimeLefts    [50]float32
    
}
type HitEvent struct{
    Effects []EffectPayload
    SourceID Entity
    TargetID Entity
    Damage float32
    DamageType uint8 

}


// --- NHÓM HIỆU ỨNG (DoT/HoT) ---

type ScheduledTask struct{
    TaskType uint8
    TimeLeft float32
}
type Item struct{
    ItemType byte 
    Value uint16
}

// --- NHÓM TAG (FILTER) ---
type TagDead struct{}
type TagStunned struct{}
type TagSilenced struct{}
type TagRooted struct{}   // Trói chân (không đi được nhưng bắn được)
type TagInvincible struct{}
type TagFire struct{}
type TagToxic struct{}
type TagIce struct{}
type TagWind struct{}
type TagStone struct{}
type TagLightning struct{}
type TagArea struct{}     // Đây là một vùng đất (AOE) chứ không phải đạn bay
type TagStatic struct{}
type TagGhost struct{}
type TagBush struct{}
type TagWall struct{}
type TagItem struct{}
type ItemData struct{
    ItemType uint8
    Value uint16
}

type Intention struct {
    MoveX, MoveY float32
    AimAngle     uint16
    Casts def.Cast
   Dist uint16
}
type StatModifier struct {
	SourceID Entity // Biết bùa này từ đâu ra để sau này xóa
	Stat     def.Stat  // StatTypeSpeed, StatTypeAD... (Dùng hằng số của bạn)
	Flat     float32 // Cộng thẳng (VD: +20 AD)
	Percent  float32 // Cộng phần trăm (VD: +0.15 = 15%)
}
type Equipment struct{
    PrimaryElement uint8 
    SecondaryElement uint8
    ActiveSlot uint8
}
func(e *Equipment)GetActiveElement() uint8{
    if e.ActiveSlot == 1{
        return e.PrimaryElement
    }
    return e.SecondaryElement
}
type Faction struct{
    TeamID uint8
}
type TrailEmitter struct {
    Interval float32
    Timer    float32
    Action   func(x, y float32) 
}
type PullForce struct{
    Force float32
}
type ActiveStatusEffects struct{
    ActiveMask uint32 
    Effects [def.EffectCount]StatusEffectInstance
}
type StatusEffectInstance struct{
    SourceID Entity
    TimeLeft float32
    TickTimer float32
    Payload EffectPayload
}
type EffectPayload struct{
    Value float32
    Duration float32
    TickRate float32
    EffectType def.Effect 
    Stat def.Stat
    RemoveMask ComponentMask
}

type NetSync struct{
    NetID uint16
}
type TagProjectile struct{}

type SpawnOnDead struct{
    Action func(x,y float32)
}
type Bounce struct {
    Remaining int8 // Số lần còn được nảy (Gió = 1)
}
type WallHit struct{
    HitX,HitY bool
}
type SolidBody struct{} 
type Fragile struct{} 
type OverlapEvent struct {
    SourceID Entity 
    TargetID  Entity
}
type VisionMask struct{
    bits [16]uint64
}
func ( s *VisionMask)Set(index uint16){
    s.bits[index/64]=(s.bits[index/64] | ( 1 <<(index%64)))
}
func( s *VisionMask)Has(index uint16) bool{
    return ( s.bits[index/64] & ( 1<<(index%64)) )!=0
}
func( s *VisionMask)Clear(){
    for i:=range s.bits{
        s.bits[i]=0
    }
}
type SpatialState struct{
    OldCell uint16
    CurrentCell uint16 
}
type VisibilityMask struct{
    KnownByTeams [4]uint64
}
func ( m *VisibilityMask)Has(teamID uint8) bool{
    return (m.KnownByTeams[teamID/64] & ( 1 << (teamID%64)) ) !=0
}
func ( m *VisibilityMask)Set(teamID uint8){
    m.KnownByTeams[teamID/64] = m.KnownByTeams[teamID/64] | (1 << (teamID%64))
}
type NetVisual struct{
    VisualID uint8
}
