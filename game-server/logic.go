package main

import (
	def "game/pkg"
	"math"
)

type ZoneInfo struct {
	X, Y        float32
	Radius      float32
	TargetRad   float32 // Bán kính mục tiêu đợt thu bo tiếp theo
	Damage      int16
	ShrinkTimer uint64  // Đếm ngược tới lúc thu bo
}
// Hàm hỗ trợ tính Vận tốc
func getVelocity(angle uint16, speed float32) (float32, float32) {
	rad := float64(angle) * (math.Pi / 180.0)
	return float32(math.Cos(rad)) * speed, float32(math.Sin(rad)) * speed
}

// ==========================================
// 1. HỆ LỬA (FIRE)
// ==========================================
func SpawnFireball(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	e := engine.CreateEntity()
	vx, vy := getVelocity(angle, 1200.0)
	
	addComponent(engine, e, TagProjectile{})
	addComponent(engine, e, TagFire{})
	addComponent(engine, e, Transform{X: x, Y: y, Angle: angle})
	addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 20})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 0.4})
	addComponent(engine, e, Fragile{})
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 0, DestroyOnHit: true})

	// --- LOGIC TẦM NHÌN ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnProjectEvent(e, def.SpellFireball, tran.X, tran.Y, tran.Angle)
		},
	})

	// --- VỤ NỔ KHI CHẾT ---
	addComponent(engine, e, SpawnOnDead{
		Action: func(x, y float32) {
			exp := engine.CreateEntity()
			addComponent(engine, exp, Transform{X: x, Y: y})
			addComponent(engine, exp, Collider{ShapeType: 1, Radius: 100})
			addComponent(engine, exp, Faction{TeamID: team})
			addComponent(engine, exp, ScheduledTask{TimeLeft: 0.020})
			addComponent(engine, exp, DamageDealer{Amount: 200, SourceID: owner, DestroyOnHit: false})

			// Vụ nổ cũng cần tầm nhìn (Fog piercing)
			addComponent(engine, exp, VisibilityMask{})
			addComponent(engine, exp, NetVisual{
				createRawEvent: func(tran Transform) RawEvent {
					return NewSpawnVFX(def.VFXFireExplosion,e, tran.X, tran.Y, 100) // Client vẽ nổ 0.5s
				},
			})
		},
	})
}

func SpawnFlamewall(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16, dist uint16) {
	e := engine.CreateEntity()
	if dist > 400 { dist = 400 }
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*float32(dist)
	spawnY := y + float32(math.Sin(rad))*float32(dist)

	addComponent(engine, e, TagFire{})
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY, Angle: angle})
	// Gán thêm Radius bù trừ để VisionTriggerSystem hoạt động với hình Chữ nhật
	addComponent(engine, e, Collider{ShapeType: def.ShapeOBB, Width: 150, Height: 200, Radius: 125}) 
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 4.0})
	addComponent(engine, e, DamageDealer{
		SourceID: owner, Amount: 70, DestroyOnHit: false, TickRate: 0.5,
		Effects: []EffectPayload{{EffectType: def.EffectFire, Value: 50, TickRate: 1.0, Duration: 3.0}},
	})

	// --- LOGIC TẦM NHÌN ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnVFX(def.VFXFlamewall,e, tran.X, tran.Y,tran.Angle)
		},
	})
}

// ==========================================
// 2. HỆ ĐỘC (POISON)
// ==========================================
func SpawnToxicSpray(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	offsets := []int{-20, -10, 0, 10, 20}
	for _, offset := range offsets {
		e := engine.CreateEntity()
		finalAngle := uint16((int(angle) + offset + 360) % 360)
		vx, vy := getVelocity(finalAngle, 800.0)
		
		addComponent(engine, e, TagProjectile{})
		addComponent(engine, e, TagToxic{})
		addComponent(engine, e, Transform{X: x, Y: y, Angle: finalAngle})
		addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
		addComponent(engine, e, Collider{ShapeType: 1, Radius: 10})
		addComponent(engine, e, Faction{TeamID: team})
		addComponent(engine, e, ScheduledTask{TimeLeft: 0.5})
		addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 100, DestroyOnHit: true})
		addComponent(engine, e, Fragile{})

		// --- LOGIC TẦM NHÌN ---
		addComponent(engine, e, VisibilityMask{})
		addComponent(engine, e, NetVisual{
			createRawEvent: func(tran Transform) RawEvent {
				return NewSpawnProjectEvent(e, def.SpellToxicSpray, tran.X, tran.Y, tran.Angle)
			},
		})
	}
}

func SpawnToxicCloud(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16, dist uint16) {
	e := engine.CreateEntity()
	if dist > 200 { dist = 200 }
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*float32(dist)
	spawnY := y + float32(math.Sin(rad))*float32(dist)

	addComponent(engine, e, TagToxic{})
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 300})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 8.0})
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 50, DestroyOnHit: false, TickRate: 0.5})

	// --- LOGIC TẦM NHÌN ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnVFX(def.VFXToxicCloud,e, tran.X, tran.Y, 0)
		},
	})
}

// ==========================================
// 3. HỆ BĂNG (ICE) & CÁC HỆ KHÁC TƯƠNG TỰ
// ==========================================
func SpawnFlashFreeze(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16, dist uint16) {
	if dist > 300 { dist = 300 }
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*float32(dist)
	spawnY := y + float32(math.Sin(rad))*float32(dist)
	
	e := engine.CreateEntity()
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 0.75}) // Thời gian cảnh báo
	
	// PHẢI CÓ COLLIDER ĐỂ HỆ THỐNG VISION TÍNH ĐƯỢC TẦM NHÌN
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 250.0}) 
	
	// --- LOGIC TẦM NHÌN: CẢNH BÁO ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnVFX(def.VFXIceWarning, e,tran.X, tran.Y,0)
		},
	})

	// --- LOGIC TẦM NHÌN: VỤ NỔ ---
	addComponent(engine, e, SpawnOnDead{
		Action: func(x, y float32) {
			exp := engine.CreateEntity()
			addComponent(engine, exp, TagIce{})
			addComponent(engine, exp, Transform{X: x, Y: y})
			addComponent(engine, exp, Collider{ShapeType: 1, Radius: 250.0})
			addComponent(engine, exp, Faction{TeamID: team})
			addComponent(engine, exp, ScheduledTask{TimeLeft: 0.05})
			addComponent(engine, exp, DamageDealer{
				Effects: []EffectPayload{{EffectType: def.EffectStun, Duration: 2.0, RemoveMask: GetMask[TagStunned]()}},
			})
			
			addComponent(engine, exp, VisibilityMask{})
			addComponent(engine, exp, NetVisual{
				createRawEvent: func(tran Transform) RawEvent {
					return NewSpawnVFX(def.VFXIceExplosion,e, tran.X, tran.Y, 250)
				},
			})
		},
	})
}
func SpawnIceLance(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	e := engine.CreateEntity()
	vx, vy := getVelocity(angle, 500) // Bắn cực nhanh
	
	addComponent(engine, e, TagProjectile{})
	addComponent(engine, e, TagIce{})
	addComponent(engine, e, Transform{X: x, Y: y, Angle: angle})
	addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 15})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 1.0})
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 70, DestroyOnHit: false})

	// --- LOGIC TẦM NHÌN (Viên đạn băng) ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnProjectEvent(e, def.SpellIceLance, tran.X, tran.Y, tran.Angle)
		},
	})

	// --- VỆT BĂNG ĐỂ LẠI (Trail Emitter) ---
	addComponent(engine, e, TrailEmitter{
		Interval: 0.05, 
		Timer: 0,
		Action: func(tx, ty float32) {
			trail := engine.CreateEntity()
			addComponent(engine, trail, Transform{X: tx, Y: ty})
			addComponent(engine, trail, Collider{ShapeType: 1, Radius: 15}) // Phải có Collider để tính tầm nhìn
			addComponent(engine, trail, Faction{TeamID: team}) // Cho team mình để buff tốc
			addComponent(engine, trail, DamageDealer{
				Amount: 0, DestroyOnHit: false, TickRate: 0.2, SourceID: owner,
				Effects: []EffectPayload{
					{EffectType: def.EffectStatBuff, Value: 0.5, Duration: 0.3, Stat: def.StatSpeed},
				},
			})
			addComponent(engine, trail, ScheduledTask{TimeLeft: 2.0})

			// Tầm nhìn cho từng cục Trail
			addComponent(engine, trail, VisibilityMask{})
			addComponent(engine, trail, NetVisual{
				createRawEvent: func(tran Transform) RawEvent {
					return NewSpawnVFX(def.VFXIceTrail,trail, tran.X, tran.Y, 15)
				},
			})
		},
	})
}

// ==========================================
// 4. HỆ GIÓ (WIND)
// ==========================================
func SpawnWindShear(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	e := engine.CreateEntity()
	vx, vy := getVelocity(angle, 1500.0)
	
	addComponent(engine, e, TagProjectile{}) // Đạn xuyên thấu có thể đập tường nảy lại
	addComponent(engine, e, TagWind{})
	addComponent(engine, e, Transform{X: x, Y: y, Angle: angle})
	addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 15})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 0.4})
	addComponent(engine, e, Bounce{Remaining: 1})
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 0, DestroyOnHit: false})

	// --- LOGIC TẦM NHÌN ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnProjectEvent(e, def.SpellWindShear, tran.X, tran.Y, tran.Angle)
		},
	})
}

func SpawnTornado(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16, dist uint16) {
	if dist > 400 { dist = 300 }
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*float32(dist)
	spawnY := y + float32(math.Sin(rad))*float32(dist)
	
	e := engine.CreateEntity()
	addComponent(engine, e, TagWind{})
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 6.0})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 350}) // Bán kính hút gió
	addComponent(engine, e, PullForce{Force: 200.0})

	// --- LOGIC TẦM NHÌN ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnVFX(def.VFXTornado, e,tran.X, tran.Y, 0)
		},
	})
}

// ==========================================
// 5. HỆ ĐẤT (STONE)
// ==========================================
func SpawnShockwave(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	e := engine.CreateEntity()
	// rad := float64(angle) * (math.Pi / 180.0)
	// spawnX := x + float32(math.Cos(rad))*200.0
	// spawnY := y + float32(math.Sin(rad))*200.0
	vx, vy := getVelocity(angle, 750.0)

	addComponent(engine, e, TagProjectile{})
	addComponent(engine, e, TagStone{})
	addComponent(engine, e, Transform{X: x, Y: y, Angle: angle})
	addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
	
	// Lưu ý: ShapeOBB nhưng phải có Radius nội tiếp/ngoại tiếp để hệ thống Vision quét được
	addComponent(engine, e, Collider{ShapeType: def.ShapeOBB, Width: 50, Height: 400, Radius: 200})
	
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 75, DestroyOnHit: false})
	addComponent(engine, e, Fragile{}) // Đập tường là vỡ
	addComponent(engine, e, ScheduledTask{TimeLeft: 1.0})

	// --- LOGIC TẦM NHÌN ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnProjectEvent(e, def.SpellShockwave, tran.X, tran.Y, tran.Angle)
		},
	})
}

func SpawnBoulderfall(engine *ArchEngine, owner Entity, team uint8, aimX, aimY float32) {
	// 1. TẠO BÓNG TÂM ĐIỂM (Cảnh báo 1.2s)
	e := engine.CreateEntity()
	addComponent(engine, e, Transform{X: aimX, Y: aimY})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 1.2})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 180.0}) // Bắt buộc phải có để quét Vision
	
	// --- LOGIC TẦM NHÌN CẢNH BÁO ---
	addComponent(engine, e, VisibilityMask{})
	addComponent(engine, e, NetVisual{
		createRawEvent: func(tran Transform) RawEvent {
			return NewSpawnVFX(def.VFXBoulderWarning, e,tran.X, tran.Y, 180.0)
		},
	})

	// 2. ĐÁ RƠI XUỐNG VÀ NỔ BÙM
	addComponent(engine, e, SpawnOnDead{
		Action: func(x, y float32) {
			hitbox := engine.CreateEntity()
			addComponent(engine, hitbox, Transform{X: x, Y: y})
			addComponent(engine, hitbox, Collider{ShapeType: def.ShapeCircle, Radius: 180.0})
			addComponent(engine, hitbox, Faction{TeamID: team})
			addComponent(engine, hitbox, TagArea{}) // Vùng sát thương tĩnh
			addComponent(engine, hitbox, ScheduledTask{TimeLeft: 0.05}) // Tồn tại 1 nhịp để gây sát thương
			addComponent(engine, hitbox, DamageDealer{SourceID: owner, Amount: 200, DestroyOnHit: false})

			// --- LOGIC TẦM NHÌN VỤ NỔ ĐẤT ĐÁ ---
			addComponent(engine, hitbox, VisibilityMask{})
			addComponent(engine, hitbox, NetVisual{
				createRawEvent: func(tran Transform) RawEvent {
					return NewSpawnVFX(def.VFXBoulderCrash,e, tran.X, tran.Y, 0) // Client vẽ hiệu ứng nổ 0.6s
				},
			})
		},
	})
}