package main

import (
	"fmt"
	def "game/pkg"
	"math"
	"math/bits"
	"sync/atomic"
)


func NetworkInputSystem(engine *ArchEngine,  inputs []atomic.Uint64){
	RunSystem2(engine, func(count int,entities[]Entity, ins []Intention, nets []NetSync) {
		for i:=0 ; i< count;i++{
			in := &ins[i]
			netID := nets[i].NetID
			// in.MoveX
			in.MoveX=0
			in.MoveY=0
			in.Casts=0

			input :=  inputs[netID].Swap(0)
			keys := def.Input(input>>32)
			// dist := 
			if  keys.IsSet(def.InputA) {in.MoveX=-1}
			if  keys.IsSet(def.InputD){in.MoveX=+1}
			if keys.IsSet(def.InputS) {in.MoveY=+1}
			if keys.IsSet(def.InputW) {in.MoveY=-1}
			if in.MoveX !=0 &&in.MoveY != 0{
				in.MoveX *= 0.707
				in.MoveY *= 0.707
			}
			in.AimAngle = uint16(input>>16)
			in.Dist=uint16(input)
			if keys.IsSet(def.InputLeftClick) {in.Casts |=def.CastLM}
			if keys.IsSet(def.InputRightClick)  {in.Casts |=def.CastRM}
			if keys.IsSet(def.InputSpace) {in.Casts |=def.CastDash}
		}
	})
}
type LifeSpanSystem struct{
	deads []Entity
}
func( s *LifeSpanSystem)process(engine *ArchEngine, dt float32){
	s.deads = s.deads[:0]
	RunSystem1(engine,func(count int, entities []Entity, tasks []ScheduledTask) {
		for i:=0; i < count;i++{
			t := &tasks[i]
			t.TimeLeft-=dt 
			if t.TimeLeft <=0{
				// switch t.TaskType{
				// case Task_DestroyEntity:
					s.deads = append(s.deads, entities[i])
				// }
			}
		}
	})
	for _,e :=range s.deads{
		addComponent(engine,e,TagDead{})
	}
}

func CooldownSystem(engine *ArchEngine, dt float32){
	RunSystem1(engine,func(count int, entities []Entity, skills []SkillCooldowns) {
		for i:=0; i < count;i++{
			s := &skills[i]
			s.LMB-=dt 
			if s.LMB <0 { s.LMB=0}
			s.RMB-=dt 
			if s.RMB<0{s.RMB = 0}
			s.Space -= dt
			if s.Space < 0 {
				s.Space = 0
			}

		}
	})
}

func StatRecalculationSystem(engine *ArchEngine, dt float32){
	bitmask := GetMask[TagDead]()
	RunSystem1Ex(engine,bitmask,func(count int, entities []Entity, sheets []StatSheet) {
		for i:=0; i < count;i++{
			s :=&sheets[i]
			if s.Dirties == 0{
				continue
			}
			//fmt.println(" qua dc cai dirty  roi")
						

			if (s.Dirties & def.StatSpeed) !=0{
				sum:=float32(0)
				per := float32(0)
				for _,m := range s.Modifiers{
					if (m.Stat & def.StatSpeed ) != 0{
						sum += m.Flat
						per += m.Percent
					}
				}
				s.CurrSpeed = (s.BaseSpeed + sum) * ( 1.0 + per)
				// if per > 0 || sum>0{
				// 	//fmt.println("current speed ",s.CurrSpeed, " baseSpeed",s.BaseSpeed)
				// }
			}
			if (s.Dirties & def.StatAD) !=0{
				sum:=float32(0)
				per := float32(0)
				for _,m := range s.Modifiers{
					if (m.Stat & def.StatAD ) != 0{
						sum += m.Flat
						per += m.Percent
					}
				}
				s.CurrAD = (s.BaseAD + sum) * ( 1.0 + per)
			}
			s.Dirties = 0
			
		}
	})
}
func AddStatModidier(sheet *StatSheet, mod StatModifier){
	sheet.Modifiers = append(sheet.Modifiers, mod)
	sheet.Dirties |=mod.Stat
	// //fmt.println("add roi ",mod)
}
func RemoveStatModifier(sheet *StatSheet, sourceID Entity) {
	hasChanged := false
	dirtyFlags:= def.Stat(0)
    //fmt.println("remove roi")
	for i := 0; i < len(sheet.Modifiers); {
		if sheet.Modifiers[i].SourceID == sourceID {
			hasChanged = true
			dirtyFlags |= sheet.Modifiers[i].Stat

			lastIdx := len(sheet.Modifiers) - 1
			sheet.Modifiers[i] = sheet.Modifiers[lastIdx] 
			sheet.Modifiers = sheet.Modifiers[:lastIdx]   

			continue 
		}
		i++
	}

	if hasChanged {
		sheet.Dirties |= dirtyFlags
	}
}
type SkillCastSystem struct{

}
func (s *SkillCastSystem) process(engine *ArchEngine, dt float32) {
	exclude := GetMask[TagDead]() | GetMask[TagStunned]() | GetMask[TagSilenced]()

	RunSystem5Ex(engine, exclude, func(count int, entities []Entity, ins []Intention, trans []Transform, cools []SkillCooldowns, equips []Equipment, facs []Faction) {
		for i := 0; i < count; i++ {
			in := &ins[i]
			t := &trans[i]
			c := &cools[i]
			e := &equips[i]
			f := &facs[i]
			ownerID := entities[i]
			teamID := f.TeamID

			activeElement := e.GetActiveElement()

			// --- CHUỘT TRÁI (LMB) ---
			if  in.Casts.IsSet(def.CastLM) && c.LMB <= 0 {
				switch activeElement {
				case Element_Fire:
					SpawnFireball(engine, ownerID, teamID, t.X, t.Y, in.AimAngle)
					c.LMB = 0.8
				case Element_Poison:
					SpawnToxicSpray(engine, ownerID, teamID, t.X, t.Y, in.AimAngle)
					c.LMB = 1.0
				case Element_Ice:
					SpawnIceLance(engine, ownerID, teamID, t.X, t.Y, in.AimAngle)
					c.LMB = 0.7
				case Element_Wind:
					SpawnWindShear(engine, ownerID, teamID, t.X, t.Y, in.AimAngle)
					c.LMB = 0.4
				case Element_Stone:
					SpawnShockwave(engine, ownerID, teamID, t.X, t.Y, in.AimAngle)
					c.LMB = 0.9
				// case Element_Lightning:
				// 	SpawnLightningBolt(engine, ownerID, teamID, t.X, t.Y, in.AimAngle)
					// c.LMB = 1.2
				}
				in.Casts &= ^def.CastLM // Tắt cờ phím
			}

			// --- CHUỘT PHẢI (RMB) ---
			if in.Casts.IsSet(def.CastRM) && c.RMB <= 0 {
				
				switch activeElement {
				case Element_Fire:
                    // //fmt.println("toi day roi")
					SpawnFlamewall(engine, ownerID, teamID,t.X,t.Y,in.AimAngle,in.Dist)
					c.RMB = 15.0
				case Element_Poison:
                    // //fmt.println("toi day roi")
					SpawnToxicCloud(engine, ownerID, teamID,t.X,t.Y,in.AimAngle,in.Dist)
					c.RMB = 14.0
				case Element_Ice:
					SpawnFlashFreeze(engine,ownerID,teamID,t.X,t.Y,in.AimAngle,in.Dist)
				case Element_Wind:
					SpawnTornado(engine,ownerID,teamID,t.X,t.Y,in.AimAngle,in.Dist)
					c.RMB = 16.0
				case Element_Stone:
					SpawnBoulderfall(engine,ownerID,teamID,t.X,t.Y)
					c.RMB = 18.0
				// case Element_Lightning:
				// 	SpawnLightningStrike(engine, ownerID, teamID, in.AimX, in.AimY)
				// 	c.RMB = 10.0
				}
				in.Casts &= ^def.CastRM
			}

			// --- LƯỚT (SPACE) ---
			if  in.Casts.IsSet(def.CastDash)&& c.Space <= 0 {
				// Cấ-p gia tốc cực lớn và TagGhost trong 0.2s
				// SpawnDashEffect(engine, ownerID)
				c.Space = 8.0
				in.Casts &= ^def.CastDash
			}

			// --- ĐỔI GĂNG (Q) ---
			if in.Casts.IsSet(def.CastQ) {
				if e.ActiveSlot == 1 { e.ActiveSlot = 2 } else { e.ActiveSlot = 1 }
				in.Casts &= ^def.CastQ
			}
		}
	})
}

func VelocitySystem(engine *ArchEngine,dt float32){
	exclude := GetMask[TagDead]()|GetMask[TagStunned]()|GetMask[TagRooted]()
	RunSystem3Ex(engine,exclude,func(count int, entities []Entity, stats []StatSheet, vels []Velocity, ins []Intention) {
		for i:=0; i< count;i++{	
			v := &vels[i]
			in := &ins[i]
			speed := stats[i].CurrSpeed
			v.Dx = in.MoveX * speed
			v.Dy = in.MoveY * speed
		}
	})
}
type WallCache struct{
	ID Entity
	X,Y float32 
	C Collider 
}
type EntitiHit struct{
	WallHit
	Entity 
}
type PhysicsCollisionSystem struct{
	walls []WallCache
	hits [] EntitiHit
}

func (s *PhysicsCollisionSystem) process(engine *ArchEngine, dt float32){
	s.walls = s.walls[:0] 
	s.hits=s.hits[:0]
	RunSystem3Ex(engine,GetMask[TagDead](),func(count int, entities []Entity, trans []Transform, cols []Collider, compC []TagStatic) {
			for i := 0; i< count;i++{
				s.walls = append(s.walls, WallCache{
					ID: entities[i],
					X: trans[i].X,
					Y: trans[i].Y,
					C: cols[i],
				})
			}
	})
	exclude:=GetMask[TagGhost]()|GetMask[TagStatic]()|GetMask[TagDead]()
	RunSystem3Ex(engine,exclude,func(count int, entities []Entity, vels []Velocity, trans []Transform, cols []Collider) {
		for i:=0; i < count;i++{
			v := &vels[i]
			t := trans[i]
			c := cols[i]
			if v.Dx == 0 && v.Dy == 0 {
				continue
			}
			nextX := t.X + v.Dx * dt
			nextY := t.Y + v.Dy * dt
			hitX,hitY :=false,false
			for _, w := range s.walls {
				// Truyền nextX và t.Y hiện tại
				if checkInteract2Collider(nextX, t.Y, c,t.Angle, w.X, w.Y, w.C,0) {
					hitX=true
					break
				}

			}
			for _, w := range s.walls {
				if checkInteract2Collider(t.X, nextY, c,t.Angle, w.X, w.Y, w.C,0) {
						hitY=true
						break
					}
			}
			if hitX ||hitY{
				//  _,isPlayer := GetComponentByEntity[TagPlayer{}]()
				// fmt.Println("hit ",entities[i])
				s.hits=append(s.hits, EntitiHit{
					WallHit: WallHit{
						HitX: hitX,
						HitY: hitY,
					},
					Entity: entities[i],
				})
			}
	}})
	for _,e := range s.hits{
		addComponent(engine,e.Entity,e.WallHit)
	}
}
func MovementSystem(engine *ArchEngine , dt float32){
	exclude := GetMask[TagDead]()
	RunSystem2Ex(engine,exclude,func(count int, entities []Entity,  trans []Transform, vels []Velocity) {
		for i:=0 ;i < count;i++{
			v := vels[i]
			t := &trans[i]
			t.X += v.Dx * dt 
			t.Y += v.Dy * dt
			if t.X > MapSize{
				t.X = MapSize
			}
			if t.X < 0{
				t.X = 0
			}
			if t.Y<0{
				t.Y =0
			}
			if t.Y>MapSize{
				t.Y=MapSize
			}
		}
	})
}
type TargetCache struct {
	ID     Entity
	TeamID uint8
	X, Y   float32
	C      Collider
}

// // Khởi tạo HitboxSystem
// type HitboxSystem struct {
// 	targetCache []TargetCache
// 	walls []WallCache
	
// 	deads []Entity
// }

// func ( s *HitboxSystem) process(engine *ArchEngine,dt float32,events  *[]HitEvent){
// 	exclude := GetMask[TagDead]()|GetMask[TagInvincible]()
// 	s.targetCache = s.targetCache[:0]
// 	s.deads= s.deads[:0]
// 	s.walls=s.walls[:0]
// 	RunSystem4Ex(engine,exclude,func(count int, entities []Entity, trans []Transform, cols []Collider, facs []Faction, vis []Vitality) {
// 		for i:=0;i< count;i++{
// 			s.targetCache = append(s.targetCache, TargetCache{
// 				ID: entities[i],
// 				TeamID: facs[i].TeamID,
// 				X: trans[i].X,
// 				Y: trans[i].Y,
// 				C: cols[i],
// 			})
// 		}
// 	})

// 	// //fmt.println("len wall ",len(s.walls) )
// 	exclude = GetMask[TagDead]()
// 	RunSystem4Ex(engine,exclude,func(count int, entities []Entity, damages []DamageDealer, cols []Collider, trans []Transform, facs []Faction) {
// 		for i:=0;i<count;i++{
// 			d := &damages[i]
// 			c := cols[i]
// 			t := trans[i]
// 			if d.TickRate > 0 && d.targets != nil {
// 				for targetID, timeLeft := range d.targets {
// 					if timeLeft > 0 {
// 						d.targets[targetID] = timeLeft - dt
// 					}
// 				}
// 			}
// 			for _,target :=range s.targetCache{
// 				if facs[i].TeamID==target.TeamID{
// 					continue
// 				}

// 				if checkInteract2Collider(target.X,target.Y,target.C,0,t.X,t.Y,c,t.Angle){
// 					if d.TickRate>0{
// 						if d.targets == nil{
// 							d.targets = make(map[Entity]float32)
// 						}
// 						if d.targets[target.ID] >0{
// 							continue
// 						}
// 						d.targets[target.ID] = d.TickRate
// 					}
// 					//fmt.println("trung roi")
// 					*events=append(*events, HitEvent{
// 						Effects: d.Effects,
// 						SourceID: d.SourceID,
// 						TargetID: target.ID,
// 						Damage: d.Amount,
// 						DamageType: d.Type,
// 					})

// 					if d.DestroyOnHit{
// 						s.deads=append(s.deads, entities[i])
// 						break
// 					}
// 				}
// 			}
// 		}
// 	})
// 	// //fmt.println("len dead ",len(s.deads))
// 	for _,e :=range s.deads{
// 		addComponent(engine,e,TagDead{})
// 	}

// }
type DamageApplySystem struct {
	deads []Entity // Cache những kẻ bị đánh chết
}
func (sys *DamageApplySystem) process(engine *ArchEngine, dt float32, events *[]HitEvent) {
	if len(*events) == 0 { return }

	for _, ev := range *events {
		//fmt.println("hit event ", ev)

		if ev.Damage <= 0 { continue }
		
		// Tìm thẳng nạn nhân (O(1)), không cần lặp qua tất cả quái vật!
		vis, ok1 := GetComponentByEntity[Vitality](engine, ev.TargetID)
		stats, ok2 := GetComponentByEntity[StatSheet](engine, ev.TargetID)
		//fmt.println("ok ",ok1 ," ",ok2)
		if ok1 && ok2 {
			multiplier := 100.0 / (100.0 + stats.Armor)
			actualDamage := ev.Damage * multiplier
			
			// Trừ Shield ...
			if vis.Shield >= actualDamage {
				vis.Shield -= actualDamage
				actualDamage = 0
			} else {
				actualDamage -= vis.Shield
				vis.Shield = 0
			}
			if actualDamage > 0 {
				// cur:=vis.HP
				vis.HP -= actualDamage
				// //fmt.println("tru mau ",cur,"- ", vis.HP)
            
				if vis.HP <= 0 {
					vis.HP = 0
					sys.deads = append(sys.deads, ev.TargetID) 
				}
			}
		} 

	}
	
	for _, e := range sys.deads {
		addComponent(engine, e, TagDead{})
	}
	// events=events[:]
}
func TrailEmitterSystem(engine *ArchEngine, dt float32) {
    // Chạy qua tất cả những Entity có Transform và TrailEmitter
    RunSystem2(engine, func(count int,entities[]Entity, transforms []Transform, emitters []TrailEmitter) {
        for i := 0; i < count; i++ {
            emitters[i].Timer -= dt
            if emitters[i].Timer <=0 {
                emitters[i].Timer += emitters[i].Interval
                // Gọi hàm callback để đẻ ra Trail
                if emitters[i].Action != nil {
                    emitters[i].Action(transforms[i].X, transforms[i].Y)
                }
            }
        }
    })
}
type StatusEffectApplySystem struct{
	targets []Entity
}

func (sys *StatusEffectApplySystem) process(engine *ArchEngine, events *[]HitEvent) {
    sys.targets=sys.targets[:0]
	for _,ev:=range *events{
		if len(ev.Effects)==0 {
			continue
		}
		targetID := ev.TargetID
		activeEffects ,ok:=GetComponentByEntity[ActiveStatusEffects](engine,targetID)
		if !ok{
			continue
		}
	
		for _,effect := range ev.Effects{
			Instance := StatusEffectInstance{
				SourceID: ev.SourceID,
				TimeLeft: effect.Duration,
				TickTimer: effect.TickRate,
				Payload: effect,
			}
			idx :=effect.EffectType
			mask := uint32( 1 <<idx)
			if (activeEffects.ActiveMask & mask) != 0{
				if activeEffects.Effects[idx].TimeLeft < effect.Duration {
					//fmt.println("gan lai hieu ung")
					activeEffects.Effects[idx].TimeLeft = effect.Duration
				}
			}else{
				activeEffects.ActiveMask|=mask
				activeEffects.Effects[idx] = Instance
				switch effect.EffectType{
					case def.EffectFire,def.EffectPoision,def.EffectHeal:
					case def.EffectStun:
						sys.targets=append(sys.targets, targetID)

					case def.EffectStatBuff:
						sheet,ok :=GetComponentByEntity[StatSheet](engine,targetID)
						// 
						if ok{
							//fmt.println("add speed ",effect.Stat, " ",effect.Value)
							AddStatModidier(sheet,StatModifier{
								SourceID: ev.SourceID,
								Stat: effect.Stat,
								Percent: effect.Value,
							})
							// s ,_:=GetComponentByEntity[StatSheet](engine,targetID)
							// //fmt.println(" dirties ",s.Dirties)
						}

				}
			}
	


		}

	}
	(*events) =(*events)[:0]
	for _,e := range sys.targets{
		addComponent(engine,e,TagStunned{})
		//fmt.println("taget stun ",e)
	}
}

// ==========================================
// 2. STATUS EFFECT UPDATE SYSTEM
// ==========================================
// Struct dùng để lưu nhiệm vụ xóa Mask, thay thế cho func() gây lỗi
type MaskTask struct {
	EntityID Entity
	Mask     ComponentMask
}

type StatusEffectUpdateSystem struct {
	emptyEffects  []Entity
	masksToRemove []MaskTask // Mảng Struct cực kỳ an toàn
}

func (sys *StatusEffectUpdateSystem) process(engine *ArchEngine, dt float32, events *[]HitEvent) {
	sys.emptyEffects = sys.emptyEffects[:0]
	sys.masksToRemove = sys.masksToRemove[:0]
	
	exclude := GetMask[TagDead]()

	RunSystem1Ex(engine, exclude, func(count int, entities []Entity, actEffs []ActiveStatusEffects) {
		for i:=0; i <count;i++{
			targetID := entities[i]
			if actEffs[i].ActiveMask ==0{
				continue
			}
			for j:=0 ; j< int(def.EffectCount);j++{
				mask := uint32(1 << j)
				if actEffs[i].ActiveMask & mask !=0{
					ef := &actEffs[i].Effects[j]
					ef.TimeLeft-=dt
					ef.TickTimer-=dt 
					if ef.Payload.TickRate>0{
						if ef.TickTimer<=0{
							evHit := HitEvent{
								Effects: nil,
								SourceID: ef.SourceID,
								TargetID: targetID,
								Damage: ef.Payload.Value,
							}
							(*events)=append((*events), evHit)
							ef.TickTimer+=ef.Payload.TickRate
						}
					}
					if ef.TimeLeft <=0{
						if ef.Payload.RemoveMask>0{
							sys.masksToRemove=append(sys.masksToRemove, MaskTask{
								EntityID: targetID,
								Mask: ef.Payload.RemoveMask,
							})
						}
						if ef.Payload.Stat>0{
							s,ok := GetComponentByEntity[StatSheet](engine,targetID)
							if ok{
								RemoveStatModifier(s,ef.SourceID)
							}
						}
						actEffs[i].ActiveMask &^=mask
					}


				}
				
			}
			
		}
	})
	
	for _, task := range sys.masksToRemove {
		idx := bits.TrailingZeros64(uint64(task.Mask))
		f := compOpts[idx];
			f.Remove(engine, task.EntityID)
	}
	

}
func PreDeadSystem(engine *ArchEngine){
	RunSystem3(engine,func(count int, entities []Entity, compA []TagDead, trans []Transform, onDeads []SpawnOnDead) {
		for i:=0; i < count;i++{
			d := onDeads[i]
			t := trans[i]
				//fmt.println("predead 1 ",entities[i])
			d.Action(t.X,t.Y)
			//fmt.println("predead 2",entities[i])
		}
	}) 
}
func NewKillEvent(entity Entity)RawEvent{
	ev := RawEvent{}
	ev.WriteUint32(uint32(entity))
	return ev
}



type BounceSystem struct{
	deads []Entity
	updates []Entity
}
func (s *BounceSystem)process(engine *ArchEngine){
	s.deads = s.deads[:0]
	s.updates=s.updates[:0]
	exclude := GetMask[TagDead]()
	RunSystem4Ex(engine,exclude,func(count int, entities []Entity, bounces []Bounce, hits []WallHit , vecs []Velocity, trans []Transform) {
		for i:=0 ; i< count;i++{
			v := &vecs[i]
			h := hits[i]
			b := &bounces[i]
			// visual := visuals[i]
			if b.Remaining<= 0{
				s.deads=append(s.deads, entities[i])
				continue
			}
			b.Remaining--
			if h.HitX{
				v.Dx=-v.Dx
			}
			if h.HitY{
				v.Dy=-v.Dy
			}
			rad := math.Atan2(float64(v.Dy), float64(v.Dx))
			trans[i].Angle = uint16((rad * 180.0 / math.Pi) + 360.0) % 360	
			s.updates = append(s.updates, entities[i])
		}
	})
	for _,e:=range s.deads{
		addComponent(engine,e,TagDead{})
	}
	for _,e:=range s.updates{
		addComponent(engine,e,TrajectoryChanged{})
	}
}

func SolidBodySystem(engine *ArchEngine) {
	exclude := GetMask[TagDead]()
	RunSystem3Ex(engine,exclude,func(count int, entities []Entity, vels []Velocity, hits []WallHit, tag []SolidBody){	 
		for i := 0; i < count; i++ {
			if hits[i].HitX { vels[i].Dx = 0 }
			if hits[i].HitY { vels[i].Dy = 0 }
		}
	})
}
type FragileSystem struct{
	deads []Entity
}

func (s *FragileSystem)process(engine *ArchEngine) {
	s.deads = s.deads[:0]
	exclude := GetMask[TagDead]()
	// Tìm đạn MỎNG MANH (Fragile) và ĐANG ĐẬP TƯỜNG
	RunSystem2Ex(engine, exclude, func(count int, entities []Entity, hits []WallHit, frags []Fragile) {
		for i := 0; i < count; i++ {
			s.deads = append(s.deads, entities[i])			
		}
	})
	for _, e:=range s.deads{
		// fmt.Println("entirty dead ", e)
		addComponent(engine,e,TagDead{})
	}
}
type CleanWallHitSystem struct{
	toClean []Entity
}
func (s *CleanWallHitSystem)process(engine *ArchEngine) {
	s.toClean =s.toClean[:0]
	// exclude := GetMask[TagDead]()
	RunSystem1(engine,func(count int, entities []Entity, hits []WallHit) {
		for i := 0; i < count; i++ {
			s.toClean = append(s.toClean, entities[i])
		}
	})
	for _, e := range s.toClean {
		removeComponent[WallHit](engine, e)
	}
}
type Cell struct {
	Targets []TargetCache
}

func getGridIndex(x , y float32)uint16{
	col := int(x)/CellSize
	row := int(y)/CellSize
	if col<0{
		col = 0
	}
	if col >= GridCols{
		col = GridCols - 1
	}
	if row <0{
		row = 0
	}
	if row >=GridRows{
		row = GridRows -1
	}
	return uint16(row * GridCols + col)

}


type TriggerOverlapSystem struct{
	targetCache []TargetCache
	spatialGrid[GridCols * GridRows][]TargetCache 
}
 
func (s *TriggerOverlapSystem)process(engine *ArchEngine, overlapEvents *[]OverlapEvent) {
    // Xóa sạch sổ báo cáo của frame trước (Zero-allocation)
    *overlapEvents = (*overlapEvents)[:0]
	for i := 0; i < len(s.spatialGrid); i++ {
		s.spatialGrid[i] = s.spatialGrid[i][:0]
	}


    exclude := GetMask[TagDead]()|GetMask[TagInvincible]()
	RunSystem4Ex(engine,exclude,func(count int, entities []Entity, trans []Transform, cols []Collider, facs []Faction, vis []Vitality) {
		for i:=0;i< count;i++{
			// s.targetCache = append(s.targetCache, TargetCache{
			// 	ID: entities[i],
			// 	TeamID: facs[i].TeamID,
			// 	X: trans[i].X,
			// 	Y: trans[i].Y,
			// 	C: cols[i],
			// })
			target := TargetCache{
				ID: entities[i],
				TeamID: facs[i].TeamID,
				X: trans[i].X,
				Y: trans[i].Y,
				C: cols[i],
			}
			idx := getGridIndex(target.X,target.Y)
			s.spatialGrid[idx] = append(s.spatialGrid[idx], target)
		}
	})

    exclude = GetMask[TagDead]()
    RunSystem3Ex(engine, exclude, func(count int, entities []Entity, cols []Collider, trans []Transform, facs []Faction) {
        for i := 0; i < count; i++ {
            areaID := entities[i]
            c, t, team := cols[i], trans[i], facs[i].TeamID
			
			col := int(t.X)/CellSize
			row := int(t.Y)/CellSize

			for dx :=-1 ;dx<=1;dx++{
				for dy :=-1;dy<=1;dy++{
					newCol := col +dx 
					newRol := row + dy 
					if newCol <0 || newCol >=GridCols || newRol<0||newRol>=GridRows{
						continue
					}
					idx := newRol * GridCols + newCol

 					for _, target := range s.spatialGrid[idx] {
						if areaID == target.ID || team == target.TeamID {
							continue
						}
						if checkInteract2Collider(target.X, target.Y, target.C, 0, t.X, t.Y, c, t.Angle) {
							*overlapEvents = append(*overlapEvents, OverlapEvent{
								SourceID: areaID,
								TargetID:  target.ID,
							})
							//fmt.println("trung roi ",areaID,target.ID)
						}
					}
				}
			}
			
            for _, target := range s.targetCache {
                if areaID == target.ID || team == target.TeamID {
                    continue
                }
                if checkInteract2Collider(target.X, target.Y, target.C, 0, t.X, t.Y, c, t.Angle) {
                    *overlapEvents = append(*overlapEvents, OverlapEvent{
                        SourceID: areaID,
                        TargetID:  target.ID,
                    })
					//fmt.println("trung roi ",areaID,target.ID)
                }
            }
        }
    })
}
func DamageTriggerSystem(engine *ArchEngine,dt float32, overlapEvents *[]OverlapEvent, hitEvents *[]HitEvent) {
	damageMask := GetMask[DamageDealer]()
	vitalityMask := GetMask[Vitality]()
    for _, ev := range *overlapEvents {
        // Có phải Area này là thứ gây sát thương không?
		// //fmt.println(" ev 0",ev)
        dmg, isDamage := GetComponentByEntityAndMask[DamageDealer](engine, ev.SourceID,damageMask)
        if !isDamage { continue }
		// //fmt.println(" ev 1",ev)
        _, hasVitality := GetComponentByEntityAndMask[Vitality](engine, ev.TargetID,vitalityMask)
        if !hasVitality { continue }
		// //fmt.println(" ev 2",ev)


		if dmg.TickRate>0{
			foundIdx :=-1
			for i:= 0; i < int(dmg.TargetCount);i++{
				if dmg.Targets[i] == ev.TargetID{
					foundIdx=i
					break
				}
			}
			if foundIdx!=-1{
				dmg.TimeLefts[foundIdx]-=dt 
				if dmg.TimeLefts[foundIdx] >0{
					continue
				}
				dmg.TimeLefts[foundIdx] = dmg.TickRate
			}else{
				if dmg.TargetCount<50{
					idx := dmg.TargetCount
					dmg.TargetCount++
					dmg.Targets[idx]=ev.TargetID
					dmg.TimeLefts[idx]=dmg.TickRate
				}else{
					continue
				}
			}

		}
		

		

        *hitEvents = append(*hitEvents, HitEvent{
            Effects:    dmg.Effects,
            SourceID:   dmg.SourceID,
            TargetID:   ev.TargetID,
            Damage:     dmg.Amount,
            DamageType: dmg.Type,
        })

        // Xóa đạn nếu DestroyOnHit
        if dmg.DestroyOnHit {
            addComponent(engine, ev.SourceID, TagDead{})
        }
    }
}
func PullTriggerSystem(engine *ArchEngine, overlapEvents *[]OverlapEvent) {
	pullForceMask := GetMask[PullForce]()
	velocMask :=GetMask[Velocity]()
	TransformMask :=GetMask[Transform]()
    for _, ev := range *overlapEvents {

        pull, isPull := GetComponentByEntityAndMask[PullForce](engine, ev.SourceID,pullForceMask)
        if !isPull { continue }

        // Có phải nạn nhân là thứ di chuyển được không?
        vicVel, hasVel := GetComponentByEntityAndMask[Velocity](engine, ev.TargetID,velocMask)
        vicTrans, hasTrans := GetComponentByEntityAndMask[Transform](engine, ev.TargetID,TransformMask)
        areaTrans, _ := GetComponentByEntityAndMask[Transform](engine, ev.SourceID,TransformMask)
        // fmt.Println("hut ",hasVel, " - ",hasTrans)
		// fmt.Println()
        if hasVel && hasTrans {
            // Tính hướng hút và cộng Vận tốc
            dx := areaTrans.X - vicTrans.X
            dy := areaTrans.Y - vicTrans.Y
            dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

            if dist > 0.1 {
                vicVel.Dx += (dx / dist) * pull.Force
                vicVel.Dy += (dy / dist) * pull.Force
            }
        }
    }
}
func getVisionGridIndex(x , y float32)uint16{
	col := int(x)/VisionCellSize
	row := int(y)/VisionCellSize
	if col<0{
		col = 0
	}
	if col >= GridCols{
		col = GridCols - 1
	}
	if row <0{
		row = 0
	}
	if row >=GridRows{
		row = GridRows -1
	}
	return uint16(row * GridCols + col)

}

func VisionCalculationSystem(engine *ArchEngine, visions *CellsVisibilityMask) {
	for i := range *visions {
		(*visions)[i].Clear()
	}

	RunSystem3Ex(engine, GetMask[TagDead](), func(count int, entities []Entity, trans []Transform, facs []Faction, sights []SightRange) {
			for i := 0; i < count; i++ {
			t := trans[i]
			teamID := facs[i].TeamID

			colCurrent := int(t.X)/VisionCellSize
			rowCurrent := int(t.Y)/VisionCellSize
			for _,cell:=range visionTemplates[sights[i].TemplateID]{
				col:= colCurrent+cell.dCol
				row := rowCurrent + cell.dRow
			 	if col >= 0 && col < VisionGridCols && row >= 0 && row < VisionGridRows {
                    idx := uint16(row*VisionGridCols + col)
                    visions[idx].Set(teamID)
                }
			}
		}
			
	})
}

func VisionTriggerSystem(engine *ArchEngine, visions *CellsVisibilityMask, outbox *NetworkOutbox){
	RunSystem4Ex(engine,GetMask[TagDead]()|GetMask[NetSync](), func(count int, entities []Entity, masks []VisibilityMask, visuals []NetVisual,trans []Transform, bounds []BoundingBox) {
		for i:=0; i < count;i++{
			mask := &masks[i]
			visual := visuals[i]
			t := trans[i]
			bound := bounds[i]
			// hatf := bounds[i]

			minCol := int(t.X-bound.HalfW) / VisionCellSize
			maxCol := int(t.X+bound.HalfW) / VisionCellSize
			minRow := int(t.Y-bound.HalfH) / VisionCellSize
			maxRow := int(t.Y+bound.HalfH) / VisionCellSize

			if minCol < 0 { minCol = 0 }
			if maxCol >= VisionGridCols { maxCol = VisionGridCols - 1 }
			if minRow < 0 { minRow = 0 }
			if maxRow >= VisionGridRows { maxRow = VisionGridRows - 1 }
			resMask := &VisibilityMask{}
			for row := minRow; row <= maxRow;row++{
				for col := minCol;col<=maxCol;col++{
					mask := visions[row * VisionGridCols+col]
					resMask.Or(mask)					 
				}
			}
			rawEv := visual.createRawEvent(t)
			gainedVisionMask := resMask.AndNot(*mask)
			gainedVisionMask.ForAll(func(teamID uint8) {
				outbox.Teams[teamID]=append(outbox.Teams[teamID], 
				rawEv)
			})
			lostVisionMask := mask.AndNot(*resMask)
			lostVisionMask.ForAll(func(teamID uint8) {
				// TẠO EVENT ẨN/XÓA ENTITY GỬI CHO CLIENT (Bạn cần hàm tạo event này)
				hideEv := NewRemoveEntityEvent(entities[i]) 
				// fmt.Println("vien dan ra khoi tam nhin E: ",entities[i] )
				outbox.Teams[teamID]=append(outbox.Teams[teamID], hideEv)
			})
			*mask=*resMask
		}
	})
}

func VisionTriggerVialitySystem(engine *ArchEngine, visions *CellsVisibilityMask, outbox *NetworkOutbox){
	RunSystem4Ex(engine,GetMask[TagDead](), func(count int, entities []Entity,trans []Transform, cols[]Collider, vialities []Vitality, nets[]NetSync) {
		for i:=0; i < count;i++{
		
			t := trans[i]
			v := vialities[i]
			visionRadius := float32(cols[i].Radius) 

			minCol := int(t.X-visionRadius) / VisionCellSize
			maxCol := int(t.X+visionRadius) / VisionCellSize
			minRow := int(t.Y-visionRadius) / VisionCellSize
			maxRow := int(t.Y+visionRadius) / VisionCellSize

			if minCol < 0 { minCol = 0 }
			if maxCol >= VisionGridCols { maxCol = VisionGridCols - 1 }
			if minRow < 0 { minRow = 0 }
			if maxRow >= VisionGridRows { maxRow = VisionGridRows - 1 }
			resMask := &VisibilityMask{}
			for row := minRow; row <= maxRow;row++{
				for col := minCol;col<=maxCol;col++{
					mask := visions[row * VisionGridCols+col]
					resMask.Or(mask)					 
				}
			}
			
			resMask.ForAll(func(teamID uint8) {
				position := PlayerSnapshot{
					NetID: nets[i].NetID,
					X: t.X,
					Y: t.Y,
					HP: uint16(v.HP),	
				}

				outbox.Positions[teamID]=append(outbox.Positions[teamID], position)
			})
		}
	})
}
type CleanSystem struct{
	deads []Entity
}
func ( s *CleanSystem)process(engine *ArchEngine, outbox *NetworkOutbox){
	s.deads=s.deads[:0]
	RunSystem2Ex(engine,0, func(count int, entities []Entity, deads []TagDead, masks []VisibilityMask) {
		for i := 0; i < count; i++ {
			s.deads = append(s.deads, entities[i])
			
			ev := NewRemoveEntityEvent(entities[i]) 
			
			masks[i].ForAll(func(teamID uint8) {
					outbox.Teams[teamID]=append(outbox.Teams[teamID], ev)
			})
		}
	})
	RunSystem2(engine, func(count int, entities []Entity, deads []TagDead, players []TagPlayer) {
		for i := 0; i < count; i++ {
			ev := NewKillEvent(entities[i])
			outbox.Globals = append(outbox.Globals, ev)
		}
	})
	if len(s.deads)==0{
		return
	}
	// //fmt.println(" len deads ", len(s.deads))
	for _, e := range s.deads {
		
		engine.RemoveEntity(e)
	}
}
// Bạn nên đặt System này ở file network_systems.go
type TrajectorySyncSystem struct{
	removes []Entity
}
func (s *TrajectorySyncSystem)process(engine *ArchEngine, outbox *NetworkOutbox) {
	s.removes=s.removes[:0]
	// Lặp qua tất cả những Entity vừa bị thay đổi quỹ đạo trong frame này
	RunSystem3(engine, func(count int, entities []Entity, changes []TrajectoryChanged, trans []Transform, masks []VisibilityMask) {
		for i := 0; i < count; i++ {
			updateEv := NewUpdateProjectileEvent(entities[i], trans[i].X, trans[i].Y, trans[i].Angle)
			
			// Phát gói tin Update cho các Team đang nhìn thấy
			masks[i].ForAll(func(teamID uint8) {
				outbox.Teams[teamID]=append(outbox.Teams[teamID], updateEv)
			})
			s.removes=append(s.removes, entities[i])
		}
	})
	for _,e := range s.removes{
		removeComponent[TrajectoryChanged](engine, e)
	}
	
	// // DỌN DẸP: Xóa cờ `TrajectoryChanged` của tất cả Entity đã xử lý
	// // (Để frame sau không gửi lại gói tin Update thừa)
	// RunSystem1(engine, func(count int, entities []Entity, changes []TrajectoryChanged) {
	// 	for i := 0; i < count; i++ {
	// 		removeComponent[TrajectoryChanged](engine, entities[i])
	// 	}
	// })
}
type VisionOffset struct {
    dCol int
    dRow int
}
var visionTemplates [3][]VisionOffset

func InitVisionTemplates() {
    
    
    // Giả sử game bạn có 2 loại tầm nhìn: 500 (Ngắn) và 800 (Dài)
    radiuses := []float32{500.0, 800.0} 

    for i, r := range radiuses {
        var template []VisionOffset
        
        // Giả sử tâm của vòng tròn nằm ở CHÍNH GIỮA ô lưới tọa độ (0,0)
        maxCells := int(r / float32(VisionCellSize)) + 1
        
        for dRow := -maxCells; dRow <= maxCells; dRow++ {
            for dCol := -maxCells; dCol <= maxCells; dCol++ {
                // Tọa độ float của ô lưới tương đối
                cellCenterX := float32(dCol)*float32(VisionCellSize) 
                cellCenterY := float32(dRow)*float32(VisionCellSize)
                
                // Dùng hàm check của bạn 1 lần duy nhất ở đây
                if checkCircleVsAABB(0, 0, r, cellCenterX, cellCenterY, float32(VisionCellSize), float32(VisionCellSize)) {
                    template = append(template, VisionOffset{dCol: dCol, dRow: dRow})
                }
            }
        }
        visionTemplates[i] = template
    }
    fmt.Printf("Đã build xong Vision Template. Bán kính 500 chiếm %d ô lưới.\n")
}
