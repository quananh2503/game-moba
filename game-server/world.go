package main

import (
	def "game/pkg"
	"math/rand"
	"sync/atomic"
)

//engine


type World struct{

	Engine *ArchEngine
	lifeSpanSystem *LifeSpanSystem
	skillCastSystem *SkillCastSystem
	physicCollisionSystem *PhysicsCollisionSystem
	// hitboxSystem *HitboxSystem
	damageApplySystem *DamageApplySystem
	statusEffectApplySystem *StatusEffectApplySystem
	statusEffectUpdateSystem *StatusEffectUpdateSystem
	cleanSystem *CleanSystem
	bounceSystem *BounceSystem
	cleanWallHitSystem *CleanWallHitSystem
	fragileSystem *FragileSystem

	TriggerOverlapSystem *TriggerOverlapSystem
	visions *CellsVisibilityMask

	hitEvents []HitEvent
	overlapEvents []OverlapEvent
	trajectorySyncSystem *TrajectorySyncSystem
}
func NewWord() *World{
	w:= &World{
		visions: &CellsVisibilityMask{
		
		},
		Engine:NewArchEngine(),
		lifeSpanSystem: &LifeSpanSystem{
			deads: make([]Entity, 0,256),

		},
		skillCastSystem: &SkillCastSystem{},
		physicCollisionSystem: &PhysicsCollisionSystem{
			hits: make([]EntitiHit, 0,256),
			walls: make([]WallCache, 0,256),
		},

		damageApplySystem: &DamageApplySystem{
			deads: make([]Entity, 0,256),
		},
		statusEffectApplySystem: &StatusEffectApplySystem{
			targets: make([]Entity, 0,256),
		},
		statusEffectUpdateSystem: &StatusEffectUpdateSystem{
			emptyEffects: make([]Entity, 0,256),
			masksToRemove: make([]MaskTask,0,256),

		},
		cleanSystem: &CleanSystem{
			deads: make([]Entity, 0,256),
		},
		bounceSystem: &BounceSystem{
			deads: make([]Entity, 0,256),
		},
		cleanWallHitSystem: &CleanWallHitSystem{
			toClean: make([]Entity, 0,256),
		},
		fragileSystem: &FragileSystem{
			deads: make([]Entity, 0,256),
		},
		TriggerOverlapSystem: &TriggerOverlapSystem{
			targetCache: make([]TargetCache, 0,256),
		},
		trajectorySyncSystem: &TrajectorySyncSystem{
			removes: make([]Entity, 0,256),
		},
	}
	for i:=range w.TriggerOverlapSystem.spatialGrid{
		w.TriggerOverlapSystem.spatialGrid[i] = make([]TargetCache, 0,100)
	}
	return w
	
}
func ( w *World)AcceptPendingclients(clients *[]uint16, accpets *[]MapNetEntity){

	for _,id := range *clients{
		e := w.Engine.CreateEntity()
		numCols := 32 // Giả sử xếp thành một hình vuông 32x32 = 1024 vị trí
		spacing := MapSize / float32(numCols) // 4000 / 32 = 125 đơn vị

		posX := float32(int(id) % numCols) * spacing
		posY := float32(int(id) / numCols) * spacing


		addComponent(w.Engine, e, Transform{X: posX, Y: posY}) 
		addComponent(w.Engine, e, Velocity{})
		addComponent(w.Engine, e, Collider{Radius: 25, ShapeType: def.ShapeCircle})
		addComponent(w.Engine, e, Intention{}) // Nhận phím bấm
		addComponent(w.Engine, e, Vitality{HP: 2000, MaxHP: 2000})		
		addComponent(w.Engine, e, StatSheet{BaseSpeed: def.StatBaseSpeed, CurrSpeed: def.StatBaseSpeed,Armor: 100000})
		addComponent(w.Engine, e, SkillCooldowns{})
		element := rand.Intn(5)+1

		addComponent(w.Engine, e, Equipment{PrimaryElement:uint8(element), ActiveSlot: 1})
		addComponent(w.Engine, e, Faction{TeamID: uint8(id)}) // Tạm thời TeamID = NetID (Đấu đơn)
		addComponent(w.Engine,e,NetSync{NetID: id})
		addComponent(w.Engine,e,ActiveStatusEffects{})
		addComponent(w.Engine,e,SolidBody{})
		addComponent(w.Engine,e,SightRange{TemplateID: 0})
		// ư.clients[id].Entity=e
		// w.sessions.SetEntityByID(id,e)
		(*accpets)=append((*accpets), MapNetEntity{
			NetID: id,
			Entity: e,
			TeamID: uint8(id),
		})
	}	
	(*clients)=(*clients)[:0]
}
func( w *World)RemoveEntities(dels *[]Entity){
	for _,e:= range *dels{
		w.Engine.RemoveEntity(e)
	}
	(*dels)=(*dels)[:0]
}
func( w *World)Tick( dt float32,inputs *[MaxPlayers]atomic.Uint64,outbox *NetworkOutbox){
		
		NetworkInputSystem(w.Engine,inputs[:])
		w.lifeSpanSystem.process(w.Engine,dt)
		CooldownSystem(w.Engine,dt)
		StatRecalculationSystem(w.Engine,dt)
		w.skillCastSystem.process(w.Engine,dt)
		VelocitySystem(w.Engine,dt)
		PullTriggerSystem(w.Engine,&w.overlapEvents)
		w.physicCollisionSystem.process(w.Engine,dt)
		w.bounceSystem.process(w.Engine)
		w.fragileSystem.process(w.Engine)
		SolidBodySystem(w.Engine)
		MovementSystem(w.Engine,dt)
		// SpatialMappingSystem(w.Engine)
		VisionCalculationSystem(w.Engine,w.visions)	
		VisionTriggerSystem(w.Engine,w.visions,outbox)
		TrailEmitterSystem(w.Engine,dt)
		w.TriggerOverlapSystem.process(w.Engine,&w.overlapEvents)
		
		DamageTriggerSystem(w.Engine,dt,&w.overlapEvents,&w.hitEvents)
		
		w.damageApplySystem.process(w.Engine,dt,&w.hitEvents)
		
		w.statusEffectApplySystem.process(w.Engine,&w.hitEvents)
		w.statusEffectUpdateSystem.process(w.Engine,dt,&w.hitEvents)
		PreDeadSystem(w.Engine)
		VisionTriggerVialitySystem(w.Engine,w.visions,outbox) 
		w.trajectorySyncSystem.process(w.Engine,outbox)
		w.cleanWallHitSystem.process(w.Engine)
		w.cleanSystem.process(w.Engine,outbox)

}
// Thêm struct này ở ngoài để lưu nháp






func SpawnMapObjects(e *ArchEngine) {
	//////fmt.println("🌳 Bắt đầu rải Môi trường lên bản đồ...")

	// 1. Sinh 20 Bụi Cỏ (Tàng hình)
	for i := 0; i < 30; i++ {
		bush := e.CreateEntity()
		// addComponent(e, bush, TagStatic{})
		addComponent(e,bush,TagBush{})
		addComponent(e, bush, Transform{
			X: float32(rand.Intn(int(MapSize))),
			Y: float32(rand.Intn(int(MapSize))),
		})
		addComponent(e, bush, Collider{Radius: 100.0}) // Bụi cỏ to 100px
	}

	// 2. Sinh 15 Bức Tường
	for i := 0; i < 40; i++ {
		wall := e.CreateEntity()
		addComponent(e, wall, TagStatic{})
		addComponent(e,wall,TagWall{})

		// Tránh sinh tường quá sát mép map
		x := float32(rand.Intn(int(MapSize-400)) + 200)
		y := float32(rand.Intn(int(MapSize-400)) + 200)

		// Tường có hình dạng ngẫu nhiên (Dọc hoặc Ngang)
		w, h := float32(300.0), float32(50.0)
		if rand.Float32() > 0.5 { w, h = h, w } // Đảo ngược 50% cơ hội

		addComponent(e, wall, Transform{X: x, Y: y})
		addComponent(e, wall, Collider{ShapeType:2,Width: w, Height: h})
	}
}
func (s *Server) CacheMapData() {
	writer := NewPacketWriter(4*8196)
	writer.Reset()

	bushCountOffset := writer.Pos
	writer.WriteUint8(0) 

	totalBushes := 0

	RunSystem3(s.world.Engine, func(count int, entities []Entity, tags []TagBush, pos []Transform, cols []Collider) {
		totalBushes += count
		for i := 0; i < count; i++ {
			writer.WriteFloat32(pos[i].X)
			writer.WriteFloat32(pos[i].Y)
			writer.WriteFloat32(cols[i].Radius)
		}
	})


	writer.Buf[bushCountOffset] = byte(totalBushes)


	wallCountOffset := writer.Pos
	writer.WriteUint8(0)
	totalWalls := uint8(0)

	RunSystem3(s.world.Engine, func(count int, entities []Entity, tags []TagWall, pos []Transform, boxes []Collider) {
		totalWalls += uint8(count)
		for i := 0; i < count; i++ {
			writer.WriteFloat32(pos[i].X)
			writer.WriteFloat32(pos[i].Y)
			writer.WriteFloat32(boxes[i].Width)
			writer.WriteFloat32(boxes[i].Height)
		}
	})
	writer.Buf[wallCountOffset] = totalWalls
	payload := make([]byte, writer.Pos)
	copy(payload,writer.Buf)
	s.mapDataCache = payload
}

