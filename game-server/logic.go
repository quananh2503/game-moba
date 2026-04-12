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


// Server gửi tọa độ và Loại hiệu ứng (VD: 1 = Nổ Lửa, 2 = Nổ Độc)

func getVelocity(angle uint16, speed float32) (float32, float32) {
	rad := float64(angle) * (math.Pi / 180.0)
	return float32(math.Cos(rad)) * speed, float32(math.Sin(rad)) * speed
}
func SpawnFireball(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	e := engine.CreateEntity()
	vx, vy := getVelocity(angle, 1200.0)
	addComponent(engine,e,TagProjectile{})
	addComponent(engine, e, TagFire{})
	addComponent(engine, e, Transform{X: x, Y: y, Angle: angle})
	addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 20}) // Đạn hình tròn
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 0.40})
	addComponent(engine,e,Fragile{})
	addComponent(engine, e, DamageDealer{
		SourceID: owner, Amount: 0, DestroyOnHit: true, // Chạm là nổ
	})
	
	addComponent(engine,e,SpawnOnDead{
		Action: func( x ,y float32 ) {
			e := engine.CreateEntity()
			addComponent(engine,e,Transform{X: x,Y: y})
			addComponent(engine,e,Collider{ShapeType: 1,Radius: 100})
			addComponent(engine,e,Faction{TeamID: team})
			addComponent(engine,e,DamageDealer{Effects: nil,Amount: 200,SourceID: owner,DestroyOnHit: false})
			addComponent(engine,e,ScheduledTask{TimeLeft: 0.020})
			ev :=NewSpawnVFXCircleEvent(def.VFXFireExplosion,x,y,100,0.020)
			engine.FrameEvents = append(engine.FrameEvents, ev)
		},
	})
	ev := NewSpawnProjectEvent(e,def.SpellFireball,x,y,angle)
	engine.FrameEvents=append(engine.FrameEvents, ev)


	// Chú ý: Việc nổ AoE 100 units sẽ được xử lý trong HitboxSystem khi viên đạn này bị DestroyOnHit.
}

func SpawnFlamewall(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16, dist uint16) {
	e := engine.CreateEntity()
	
	// Tính tọa độ xuất hiện của tường lửa (Cách người chơi 100 units)
	if dist > 400{
		dist = 400
	}
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*float32(dist)
	spawnY := y + float32(math.Sin(rad))*float32(dist)

	addComponent(engine, e, TagFire{})
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY, Angle: angle})
	addComponent(engine, e, Collider{ShapeType: def.ShapeOBB, Width: 150, Height: 200}) // Tường hình chữ nhật (xoay ngang)
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 4.0})
	

	igniteEffect := EffectPayload{
		EffectType: def.EffectFire,
		Value:      50,   // 15 Sát thương
		TickRate:   1.0,  // Mỗi 1 giây
		Duration:   3.0,  // Kéo dài 2 giây
	}
	// Tường lửa không bay, nó nằm trên đất và gây TickEffect (Đốt cháy)
	addComponent(engine, e, DamageDealer{
		SourceID: owner, Amount: 70, DestroyOnHit: false, // Đi xuyên qua,
		Effects: []EffectPayload{igniteEffect},
		TickRate: 0.5,

		// Ty

	})
	ev :=NewSpawnVFXBoxEvent(def.VFXFireExplosion,spawnX,spawnY,150	,200,4,angle)
	engine.FrameEvents=append(engine.FrameEvents, ev)
	// Gắn thêm component buff Ignite cho ai đi ngang qua (Xử lý ở hệ thống va chạm)
}
func SpawnToxicSpray(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	// Bắn 5 viên shotgun, lệch nhau -20, -10, 0, 10, 20 độ
	offsets := []int{-20, -10, 0, 10, 20}
	for _, offset := range offsets {
		e := engine.CreateEntity()
		finalAngle := uint16((int(angle) + offset + 360) % 360)
		vx, vy := getVelocity(finalAngle, 800.0) // Bay chậm hơn
		addComponent(engine,e,TagProjectile{})
		addComponent(engine, e, TagToxic{})
		addComponent(engine, e, Transform{X: x, Y: y, Angle: finalAngle})
		addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
		addComponent(engine, e, Collider{ShapeType: 1, Radius: 10})
		addComponent(engine, e, Faction{TeamID: team})
		addComponent(engine, e, ScheduledTask{TimeLeft: 0.5}) // Bay 400 units (800 * 0.5)
		addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 100, DestroyOnHit: true})
		addComponent(engine,e, Fragile{})

		ev := NewSpawnProjectEvent(e,def.SpellToxicSpray,x,y,finalAngle)
		engine.FrameEvents=append(engine.FrameEvents, ev)
	}
}

func SpawnToxicCloud(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16, dist uint16) {
	e := engine.CreateEntity()
	if dist > 200{
		dist = 200
	}
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*float32(dist)
	spawnY := y + float32(math.Sin(rad))*float32(dist)
	addComponent(engine, e, TagToxic{})
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 300}) // Đám mây tròn to
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 8.0})
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 50, DestroyOnHit: false,TickRate: 0.5})
	// ev :=NewSpawnVFXBoxEvent(VfX,spawnX,spawnY,150	,200,4,angle)
	ev :=NewSpawnVFXCircleEvent(def.VFXToxicCloud,spawnX,spawnY,300,8)
	engine.FrameEvents=append(engine.FrameEvents, ev)
}

// ==========================================
// 3. HỆ BĂNG (ICE)
// ==========================================
func SpawnIceLance(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	e := engine.CreateEntity()
	vx, vy := getVelocity(angle, 500) // Bắn cực nhanh
	addComponent(engine,e,TagProjectile{})
	addComponent(engine, e, TagIce{})
	addComponent(engine, e, Transform{X: x, Y: y, Angle: angle})
	addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 15})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 1.0})
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 70, DestroyOnHit: false})
    // ev :=	

	addComponent(engine, e, TrailEmitter{Interval: 0.05, Timer: 0,
		Action: func(x, y float32) {
			 e :=engine.CreateEntity()
			//  addComponent(engine, e, TagStatic{})
			 addComponent(engine,e,Transform{X: x,Y:y})
			 addComponent(engine,e,Collider{ShapeType: 1,Radius: 15})
			 addComponent(engine,e,Faction{TeamID: 255})
			 addComponent(engine,e,DamageDealer{
				Amount: 0,
				DestroyOnHit: false,
				TickRate: 0.2,
				SourceID: owner,
				Effects: []EffectPayload{
					{
						EffectType: def.EffectStatBuff,
						Value: 0.5,
						Duration: 0.3,
						Stat: def.StatSpeed,
					},
				},
			 })
			 addComponent(engine,e,ScheduledTask{TimeLeft: 2.0})
            ev := NewSpawnVFXCircleEvent(def.VFXIceTrail,x,y,15,2 )
	        engine.FrameEvents = append(engine.FrameEvents, ev)
		},}) 
	 ev := NewSpawnProjectEvent( e, def.SpellIceLance, x, y, angle)
	engine.FrameEvents = append(engine.FrameEvents, ev)
}

func SpawnFlashFreeze(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16, dist uint16) {
		if dist > 300{
		dist = 300
	}
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*float32(dist)
	spawnY := y + float32(math.Sin(rad))*float32(dist)
	e := engine.CreateEntity()
	addComponent(engine, e, ScheduledTask{TimeLeft: 0.75})
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY})
	addComponent(engine,e,Faction{TeamID: team})

	addComponent(engine,e,SpawnOnDead{
		Action: func(x, y float32) {
			//fmt.println(" tao dong bang roi ")
			e := engine.CreateEntity()
			addComponent(engine, e, TagIce{})
			addComponent(engine, e, Transform{X: spawnX, Y: spawnY})
			addComponent(engine,e,Collider{ShapeType:1,Radius: 250.0})
			addComponent(engine,e,Faction{TeamID: team})
			addComponent(engine, e, ScheduledTask{TimeLeft: 0.05})
			
			addComponent(engine,e,DamageDealer{
				Effects: []EffectPayload{
					EffectPayload{
						EffectType: def.EffectStun,
						Duration: 2.0,
						RemoveMask: GetMask[TagStunned](),
					},
				},
			})
			burstEv := NewSpawnVFXCircleEvent(def.VFXIceExplosion, x, y, 250, 0.5)
			engine.FrameEvents = append(engine.FrameEvents, burstEv)
		},
	})
	ev := NewSpawnVFXCircleEvent(def.VFXIceWarning, spawnX, spawnY, 250, 0.75)
	engine.FrameEvents = append(engine.FrameEvents, ev)
	// addComponent(engine, e, DelayedAction{Timer: 0.75, ActionID: 1}) // 0.75s sau kích hoạt nổ Stun
}

// // ==========================================
// // 4. HỆ GIÓ (WIND)
// // ==========================================
func SpawnWindShear(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	e := engine.CreateEntity()
	vx, vy := getVelocity(angle, 1500.0)
	// addComponent(engine,e,TagProjectile{})
	addComponent(engine, e, TagWind{})
	addComponent(engine, e, Transform{X: x, Y: y, Angle: angle})
	addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
	addComponent(engine, e, Collider{ShapeType: 1, Radius: 15}) // Hình cung/chữ nhật
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e, ScheduledTask{TimeLeft: 0.4})
	addComponent(engine,e,Bounce{Remaining: 1})
	// Đạn xuyên thấu (DestroyOnHit: false)
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 0, DestroyOnHit: false})
	ev :=NewSpawnProjectEvent(e,def.SpellWindShear,x,y,angle)
	engine.FrameEvents=append(engine.FrameEvents, ev)
}

func SpawnTornado(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16, dist uint16)  {
	if dist > 400{
		dist = 300
	}
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*float32(dist)
	spawnY := y + float32(math.Sin(rad))*float32(dist)
	e := engine.CreateEntity()

	
	addComponent(engine, e, TagWind{})
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY})
	addComponent(engine, e, Faction{TeamID: team})
	addComponent(engine, e,ScheduledTask{TimeLeft: 6.0,})
	addComponent(engine,e,Collider{ShapeType: 1,Radius: 350})
	addComponent(engine, e, PullForce{Force: 200.0})

	ev := NewSpawnVFXCircleEvent(def.VFXTornado,spawnX,spawnY,350,6)
	engine.FrameEvents=append(engine.FrameEvents, ev)
}
func SpawnShockwave(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
	e := engine.CreateEntity()
	
	// Vì hình chữ nhật dài tới 800, nếu spawn ở tâm người chơi thì 400 units sẽ nằm ở... sau lưng.
	// Ta dời tâm của sóng đất lên phía trước 400 units để nó quét từ người chơi quét ra.
	rad := float64(angle) * (math.Pi / 180.0)
	spawnX := x + float32(math.Cos(rad))*200.0
	spawnY := y + float32(math.Sin(rad))*200.0
	
	// Sóng đất trượt đi với tốc độ vừa phải
	vx, vy := getVelocity(angle, 750.0)

	addComponent(engine, e, TagProjectile{})
	// Thêm TagStone nếu bạn có
	addComponent(engine, e, Transform{X: spawnX, Y: spawnY, Angle: angle})
	addComponent(engine, e, Velocity{Dx: vx, Dy: vy})
	
	// Hình chữ nhật: Rộng 100, Dài 800 (Xoay theo hướng bắn)
	addComponent(engine, e, Collider{ShapeType: def.ShapeOBB, Width: 50, Height: 400})
	addComponent(engine, e, Faction{TeamID: team})
	
	// Xuyên người (DestroyOnHit: false), nhưng Đập tường là vỡ (Fragile)
	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 75, DestroyOnHit: false})
	addComponent(engine, e, Fragile{}) 
	
	// Tồn tại 0.6s (Trượt được thêm 600 units rồi tự tan biến)
	addComponent(engine, e, ScheduledTask{TimeLeft: 1})

	ev := NewSpawnProjectEvent(e, def.SpellShockwave, spawnX, spawnY, angle)
	engine.FrameEvents = append(engine.FrameEvents, ev)
}
func SpawnBoulderfall(engine *ArchEngine, owner Entity, team uint8, aimX, aimY float32) {
	// 1. TẠO BÓNG TÂM ĐIỂM (Cảnh báo 1.2s)
	e := engine.CreateEntity()
	addComponent(engine, e, Transform{X: aimX, Y: aimY})
	addComponent(engine, e, ScheduledTask{TimeLeft: 1.2})

	// Báo Client vẽ vòng tròn cảnh báo màu Đất
	warnEv := NewSpawnVFXCircleEvent(def.VFXBoulderWarning, aimX, aimY, 180.0, 1.2)
	engine.FrameEvents = append(engine.FrameEvents, warnEv)

	// 2. ĐÁ RƠI XUỐNG VÀ NỔ BÙM
	addComponent(engine, e, SpawnOnDead{
		Action: func(x, y float32) {
			hitbox := engine.CreateEntity()
			addComponent(engine, hitbox, Transform{X: x, Y: y})
			addComponent(engine, hitbox, Collider{ShapeType: def.ShapeCircle, Radius: 180.0})
			addComponent(engine, hitbox, Faction{TeamID: team})
			addComponent(engine, hitbox, TagArea{}) // Vùng sát thương tĩnh
			
			// Tồn tại 1 frame để quét sát thương
			addComponent(engine, hitbox, ScheduledTask{TimeLeft: 0.020})

			// Sát thương CỰC MẠNH: 200
			addComponent(engine, hitbox, DamageDealer{
				SourceID: owner, Amount: 200, DestroyOnHit: false,
			})

			// Báo Client vẽ vụ nổ Đất Đá
			crashEv := NewSpawnVFXCircleEvent(def.VFXBoulderCrash, x, y, 180.0, 0.6)
			engine.FrameEvents = append(engine.FrameEvents, crashEv)
		},
	})
}
// // ==========================================
// // 5. HỆ ĐẤT VÀ HỆ SÉT
// // ==========================================
// // Bạn có thể tự viết hàm SpawnShockwave (Tương tự IceLance nhưng ShapeType = 2 (Box) và có TagGroundProjectile).
// // SpawnBoulderfall và SpawnLightningStrike sử dụng y hệt logic của FlashFreeze (Dùng DelayedAction).
// func SpawnSockWave(engine *ArchEngine, owner Entity, team uint8, x,y float32, angle uint16){
// 	e := engine.CreateEntity()
// 	vx,vy :=getVelocity(angle,500)


// 	addComponent(engine,e,Transform{X: x,Y: y,Angle: angle})
// 	addComponent(engine,e,Collider{ShapeType: 2,Height: 800,Width: 100})
// 	addComponent(engine,e,Velocity{Dx: vx,Dy: vy})
// 	addComponent(engine,e,Faction{TeamID: team})
// 	addComponent(engine,e,ScheduledTask{TimeLeft: 1.0,TaskType: Task_DestroyEntity})
// 	addComponent(engine,e,DamageDealer{
// 		Amount: 75,
// 		SourceID: owner,
// 		// Type: ,
// 		DestroyOnHit: false,

// 	})
// }
// func SpawnBoulderfall(engine *ArchEngine, owner Entity, team uint8, x,y float32){
// 	e := engine.CreateEntity()


// 	addComponent(engine,e,Transform{X: x,Y: y})
// 	addComponent(engine,e,Collider{ShapeType: 1,Radius: 180})
// 	addComponent(engine,e,Faction{TeamID: team})
// 	addComponent(engine,e,ScheduledTask{TimeLeft: 1.2,TaskType: Task_Explode})
// 	addComponent(engine,e,DamageDealer{
// 		Amount: 200,
// 		SourceID: owner,
// 		// Type: ,
// 		DestroyOnHit: false,

// 	})	
// }
// func SpawnLightningBolt(engine *ArchEngine, owner Entity, team uint8, x, y float32, angle uint16) {
// 	// HITSCAN: Không sinh Entity bay. Bắn 1 tia rọi thẳng từ X, Y tới AimAngle độ dài 1000 units.
// 	// Check va chạm ngay lập tức trong hàm này, hoặc đẻ ra 1 Entity tĩnh dài 1000 units tồn tại 0.1s.
	
// 	e := engine.CreateEntity()
// 	addComponent(engine, e, TagLightning{})
	
// 	// Tính điểm cuối của tia sét
// 	rad := float64(angle) * (math.Pi / 180.0)
// 	endX := x + float32(math.Cos(rad))*1000.0
// 	endY := y + float32(math.Sin(rad))*1000.0
	
// 	// Tạo 1 BoxCollider bao phủ từ điểm đầu đến điểm cuối
// 	addComponent(engine, e, Transform{X: (x+endX)/2, Y: (y+endY)/2, Angle: angle})
// 	addComponent(engine, e, Collider{ShapeType: 2, Width: 1000, Height: 20})
// 	addComponent(engine, e, Faction{TeamID: team})
// 	addComponent(engine, e, ScheduledTask{TimeLeft: 0.1,TaskType: Task_DestroyEntity})
// 	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 30, DestroyOnHit: false}) // Xuyên thấu
// }
// func SpawnLightningStrike(engine *ArchEngine, owner Entity,team uint8, x,y float32){
// 	e := engine.CreateEntity()
// 	addComponent(engine, e, Transform{X: x, Y: y})
// 	addComponent(engine, e, Collider{ShapeType: 1, Radius: 200})
// 	addComponent(engine, e, Faction{TeamID: team})
// 	// addComponent(engine, e, Lifespan{TimeLeft: 0.1}) // Biến mất sau 1 tick để vẽ UI
// 	addComponent(engine,e,ScheduledTask{TimeLeft: 0.6,TaskType: Task_DestroyEntity})
// 	addComponent(engine, e, DamageDealer{SourceID: owner, Amount: 120, DestroyOnHit: false}) // Xuyên thấu	
// 	addComponent(engine,e, TagSilenced{})
// }