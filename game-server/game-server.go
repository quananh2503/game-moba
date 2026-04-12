package main

import (
	"fmt"
	def "game/pkg"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)
const (
	MapSize      = 4000.0
	TickRate     = 60 // 60 Khung hình / giây

	CellSize = 100
	GridCols = int(MapSize / CellSize)
	GridRows = int(MapSize / CellSize)

	Element_Fire    uint8 = 1
	Element_Lightning   uint8 = 2
	Element_Ice     uint8 = 3
	Element_Wind    uint8 = 4
	Element_Stone   uint8 = 5
	Element_Poison uint8 = 6
)
type EventForTeams struct{
	teamID uint8
	rawEvent RawEvent
}
type SpatialGrid struct{
	Cells [GridCols*GridRows][]Entity
	TeamVisions [256]VisionMask
}
type GameEvent struct{
	SeqID uint16 
	Type byte 
	Len uint8
	Payload [MaxEventPayload]byte 
}
type PendingEvent struct{
	Event GameEvent
	SentAt time.Time
	Sendtries uint8
}
type GameLargeEvent struct{
	SeqID uint16 
	Type byte 
	Payload []byte 
}
type PendingLargeEvent struct{
	Event GameLargeEvent
	SentAt time.Time
	Sendtries uint8	
}
type ClientConnection struct {
	NetID         uint16
	Addr          *unix.RawSockaddrAny
	Entity Entity
	
	NextEventSeq  uint16 
	
	PendingEvents []PendingEvent
	PendingLarge  []PendingLargeEvent
	PendingInputs atomic.Uint32
}

type GameServer struct{
	Engine *ArchEngine
	// NetToEntity map[uint16]Entity
	playerAddrs map[uint64]uint16
	addrMutex sync.Mutex
	nextPlayerID uint16


	Clients [200]ClientConnection
	ClientInputs [200]atomic.Uint64

	PendingPlayers []uint16


	MatchState uint8       // Trạng thái hiện tại
	TickCount  uint64      // Đếm số lượng Tick đã trôi qua
	Zone       ZoneInfo    // Vòng Bo
	WinnerID   uint16      // Lưu ID của kẻ sống sót


	writer *PacketWriter
	largeEventWriter *PacketWriter

	mapDataCache []byte 

	//system
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

	alivePlayer int
	TimeNow time.Time

	spatialGrid *SpatialGrid
	
	


}
func NewGameServer() *GameServer {
	writer:= NewPacketWriter(8192)
	engine := NewArchEngine()
	s:= &GameServer{
		Engine: engine,

		nextPlayerID: 0,

		writer: writer,
		largeEventWriter: NewPacketWriter(8096),
		playerAddrs: make(map[uint64]uint16),

		lifeSpanSystem: &LifeSpanSystem{
			deads: make([]Entity, 0,256),

		},
		skillCastSystem: &SkillCastSystem{},
		physicCollisionSystem: &PhysicsCollisionSystem{
			hits: make([]EntitiHit, 0,256),
			walls: make([]WallCache, 0,256),
		},
		// hitboxSystem: &HitboxSystem{
		// 	targetCache: make([]TargetCache, 0,256),
		// 	deads: make([]Entity, 0,256),
		// 	walls: make([]WallCache, 0,256),
		// },
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
		PendingPlayers: make([]uint16, 0,256),


		


	}
	
	return s
}

func (s *GameServer) getPlayerID(addr *unix.RawSockaddrAny) (uint16, bool) {
	addrHash := hashRawAddr(addr)

	s.addrMutex.Lock()
	defer s.addrMutex.Unlock()

	if id, ok := s.playerAddrs[addrHash]; ok {
		return id, true
	}

	if s.nextPlayerID < 200 {
		newID := s.nextPlayerID
		
		s.playerAddrs[addrHash] = newID
		
		s.nextPlayerID++

		s.Clients[newID] = ClientConnection{
			NetID: newID,
			Addr: copyRawAddr(addr),
			NextEventSeq: 1,
			PendingEvents: make([]PendingEvent, 0,256),
			PendingLarge: make([]PendingLargeEvent, 0,16),
			// Entity: e,

		}
		s.PendingPlayers = append(s.PendingPlayers, newID)
		fmt.Printf("[GameServer] Người chơi mới. Gán ID: %d\n",newID )
		return newID, true
	}
	return 0, false
}
func (s *GameServer)processPendingPlayers(){
	if len(s.PendingPlayers)==0{
		return
	}
	s.addrMutex.Lock()

	for _,id := range s.PendingPlayers{
		e := s.Engine.CreateEntity()
		numCols := 32 // Giả sử xếp thành một hình vuông 32x32 = 1024 vị trí
		spacing := MapSize / float32(numCols) // 4000 / 32 = 125 đơn vị

		posX := float32(int(id) % numCols) * spacing
		posY := float32(int(id) / numCols) * spacing


		addComponent(s.Engine, e, Transform{X: posX, Y: posY}) 
		addComponent(s.Engine, e, Velocity{})
		addComponent(s.Engine, e, Collider{Radius: 25, ShapeType: def.ShapeCircle})
		addComponent(s.Engine, e, Intention{}) // Nhận phím bấm
		addComponent(s.Engine, e, Vitality{HP: 2000, MaxHP: 2000})		
		addComponent(s.Engine, e, StatSheet{BaseSpeed: def.StatBaseSpeed, CurrSpeed: def.StatBaseSpeed,Armor: 100000})
		addComponent(s.Engine, e, SkillCooldowns{})
		// element := rand.Intn(5)

		addComponent(s.Engine, e, Equipment{PrimaryElement:uint8(rand.Int()%5+1), ActiveSlot: 1})
		addComponent(s.Engine, e, Faction{TeamID: uint8(id)}) // Tạm thời TeamID = NetID (Đấu đơn)
		addComponent(s.Engine,e,NetSync{NetID: id})
		addComponent(s.Engine,e,ActiveStatusEffects{})
		addComponent(s.Engine,e,SolidBody{})
		s.Clients[id].Entity=e
		ev:=NewEvent(def.EventWelcome)
		ev.WriteUint8(uint8(id))
		s.emitPrivateEvent(id, def.EventWelcome, ev)
		if s.mapDataCache !=nil{
			s.emitPrivateLargeEvent(id,def.EventSendMap,s.mapDataCache)
		}
		s.alivePlayer++
		// fmt.Println(" gui tin cho client ", id)
	}
	s.PendingPlayers=s.PendingPlayers[:0]
	s.addrMutex.Unlock()
}
func (s *GameServer) HandlePacket( data[]byte, addr *unix.RawSockaddrAny){
	if len(data)<5 {
		return 
	}
	playerNetId ,ok:= s.getPlayerID(addr)
	if !ok{
		return 
	}
	key := uint64(data[0])<<32
	angle := uint64(data[1])<<24|uint64(data[2])<<16
	dist:=uint64(data[3])<<8| uint64(data[4]) 

	// packetInput:= key|angle
	s.ClientInputs[playerNetId].Store(key|angle|dist)
	if len(data) > 5{
		s.processAckClient(data[5:],playerNetId)
	}
}
func (s *GameServer) emitGlobalEvent(evType byte, rawEvent RawEvent) {
	s.addrMutex.Lock()
	defer s.addrMutex.Unlock()
	// now :=time.Now()
	ev := GameEvent{ 
		// SeqID: , 
		Type: evType, 
		Len: rawEvent.Len,
		Payload: rawEvent.Payload,
	}
	

	for i := uint16(0); i < s.nextPlayerID; i++ {
		if s.Clients[i].Addr != nil {
			ev.SeqID = s.Clients[i].NextEventSeq
			s.Clients[i].NextEventSeq++
			s.Clients[i].PendingEvents = append(s.Clients[i].PendingEvents, PendingEvent{
				Event: ev,
				SentAt: s.TimeNow,
			})
		}
	}
}
func (s *GameServer) emitGlobalLargeEvent(evType byte,payload []byte) {
	s.addrMutex.Lock()
	defer s.addrMutex.Unlock()
	// now :=time.Now()
	payloadCopy := make([]byte,len(payload))
	copy(payloadCopy,payload)
	ev := GameLargeEvent{ 
		Type: evType, 
		Payload: payload,
	}

	for i := uint16(0); i < s.nextPlayerID; i++ {
		if s.Clients[i].Addr != nil {
			ev.SeqID=s.Clients[i].NextEventSeq
			s.Clients[i].NextEventSeq++
			s.Clients[i].PendingLarge = append(s.Clients[i].PendingLarge , PendingLargeEvent{
				Event:ev,
				SentAt: s.TimeNow,

			})
		}
	}
}
func (s *GameServer) EmitToTeam(teamID uint8, rawEvent RawEvent) {
	event :=GameEvent{
		Type: rawEvent.Type,
		Len: rawEvent.Len,
		Payload: rawEvent.Payload,
	}
	// Duyệt tất cả Client đang online
	for i := uint16(0); i < s.nextPlayerID; i++{
		client := &s.Clients[i]
		if client.Addr == nil { continue }

		// Lấy Faction của Entity tương ứng với Client này
		fac, exists := GetComponentByEntity[Faction](s.Engine, client.Entity)
		if exists && fac.TeamID == teamID {
			// Tái sử dụng logic của EmitPrivate (Viết inline để Zero-Allocation)
			// ev := PendingEvent{
			// 	SeqID:  client.NextSeqID,
			// 	Event:  rawEvent,
			// 	SentAt: s.TimeNow,
			// }
			// client.NextSeqID++
			// client.PendingEvents = append(client.PendingEvents, ev)
				event.SeqID=client.NextEventSeq
				client.NextEventSeq++
				client.PendingEvents=append(client.PendingEvents, PendingEvent{
					Event: event,
					SentAt: s.TimeNow,
				})
		}
	}
}
func(s *GameServer)emitPrivateEvent(targetId uint16, evType byte, rawEvent RawEvent){
	// now :=time.Now()
	ev := GameEvent{ 
		Type: evType, 
		Len: rawEvent.Len,
		Payload: rawEvent.Payload,
	 }

	if s.Clients[targetId].Addr != nil {
		ev.SeqID=s.Clients[targetId].NextEventSeq
		s.Clients[targetId].NextEventSeq++
		s.Clients[targetId].PendingEvents = append(s.Clients[targetId].PendingEvents , PendingEvent{
			Event: ev,
			SentAt: s.TimeNow,
		})
	}	

}
func(s *GameServer)emitPrivateLargeEvent(targetId uint16, evType byte, payload []byte){
	// now :=time.Now()
	ev := GameLargeEvent{ 
		Type: evType, 
		Payload:payload ,
	 }
	if s.Clients[targetId].Addr != nil {
		ev.SeqID=s.Clients[targetId].NextEventSeq
		s.Clients[targetId].NextEventSeq++
		s.Clients[targetId].PendingLarge = append(s.Clients[targetId].PendingLarge, PendingLargeEvent{
			Event: ev,
			SentAt: s.TimeNow,
		})
	}	

}
func (s *GameServer) processAckClient(data []byte, playerID uint16) {
	s.addrMutex.Lock()
	defer s.addrMutex.Unlock()

	if data[0] != 0xFF {
		return
	}
	
	n := data[1] // Số lượng gói tin client ACK
	if n == 0 {
		return
	}

	// ---------------------------------------------------------
	// 1. XỬ LÝ PENDING EVENTS (Sự kiện nhỏ)
	// ---------------------------------------------------------
	events := s.Clients[playerID].PendingEvents
	keep := 0 // Con trỏ lưu những sự kiện CHƯA được ACK

	for j := 0; j < len(events); j++ {
		seqToTest := events[j].Event.SeqID
		isAcked := false

		// Đọc trực tiếp các ACK Seq từ byte data để so sánh (Không cấp phát RAM)
		idx := 2
		for i := uint8(0); i < n; i++ {
			ackSeq := uint16(data[idx])<<8 | uint16(data[idx+1])
			if ackSeq == seqToTest {
				isAcked = true
				break
			}
			idx += 2
		}

		// Nếu Client CHƯA nhận được sự kiện này, ta giữ nó lại
		if !isAcked {
			events[keep] = events[j]
			keep++
		}
	}
	// Cắt bỏ phần đuôi thừa (Không tạo slice mới, chỉ đổi len)
	s.Clients[playerID].PendingEvents = events[:keep]

	// ---------------------------------------------------------
	// 2. XỬ LÝ PENDING LARGE (Sự kiện lớn - Map)
	// ---------------------------------------------------------
	eventsLarge := s.Clients[playerID].PendingLarge
	keepLarge := 0

	for j := 0; j < len(eventsLarge); j++ {
		seqToTest := eventsLarge[j].Event.SeqID
		isAcked := false

		// Lại đọc từ byte data
		idx := 2
		for i := uint8(0); i < n; i++ {
			ackSeq := uint16(data[idx])<<8 | uint16(data[idx+1])
			if ackSeq == seqToTest {
				isAcked = true
				break
			}
			idx += 2
		}

		if !isAcked {
			eventsLarge[keepLarge] = eventsLarge[j]
			keepLarge++
		}
	}
	s.Clients[playerID].PendingLarge = eventsLarge[:keepLarge]
}
func hashRawAddr(addr *unix.RawSockaddrAny) uint64{
	if addr.Addr.Family == unix.AF_INET{
		add4 := (*unix.RawSockaddrInet4)(unsafe.Pointer(addr))
		port := add4.Port
		ip:=add4.Addr
		return (uint64(port)<<32) | (uint64(ip[3])<<24) | (uint64(ip[2])<<16) | (uint64(ip[1])<<8) | (uint64(ip[0]))
	}
	return 0
}
func copyRawAddr(src *unix.RawSockaddrAny) *unix.RawSockaddrAny {
	dst := new(unix.RawSockaddrAny) // Cấp phát vùng nhớ mới an toàn
	*dst = *src
	return dst
}
func (s *GameServer) StartLoop(engine *UdpEngine) {
	// Sử dụng TickRate chung với client: 1 / TickRate giây mỗi tick
	ticker := time.NewTicker(time.Second / TickRate)
	dt := float32(0.016)
	s.Zone = ZoneInfo{
		X: MapSize / 2, 
		Y: MapSize / 2, 
		Radius: MapSize, 
		TargetRad: MapSize / 2, 
		Damage: 5,               // 5 máu/giây
		ShrinkTimer: 60 * 60,    // Chờ 1 phút trước khi thu đợt 1
	}
	overlapEvents := make([]OverlapEvent,0,2048)
	hitEvent := make([]HitEvent,0,2048)
	eventsToTeam := make([]EventForTeams, 0,2048)
	// for 
	SpawnMapObjects(s.Engine)
	s.CacheMapData()
	var totalTickTime time.Duration
	var maxTickTime time.Duration
	budget := time.Second / TickRate 
	tickCount := 0
	InitGrid()
	for {
		<-ticker.C
		   tickCount++ 
		s.TimeNow=time.Now()
		s.processPendingPlayers()
		NetworkInputSystem(s.Engine,s.ClientInputs[:])
		s.lifeSpanSystem.process(s.Engine,dt)
		CooldownSystem(s.Engine,dt)
		StatRecalculationSystem(s.Engine,dt)
		s.skillCastSystem.process(s.Engine,dt)
		VelocitySystem(s.Engine,dt)
		PullTriggerSystem(s.Engine,&overlapEvents)
		s.physicCollisionSystem.process(s.Engine,dt)
		s.bounceSystem.process(s.Engine)
		s.fragileSystem.process(s.Engine)
		SolidBodySystem(s.Engine)
		MovementSystem(s.Engine,dt)
		TrailEmitterSystem(s.Engine,dt)
		// s.hitboxSystem.process(s.Engine,dt,&hitEvent)
		s.TriggerOverlapSystem.process(s.Engine,&overlapEvents)
		
		DamageTriggerSystem(s.Engine,dt,&overlapEvents,&hitEvent)
		
		s.damageApplySystem.process(s.Engine,dt,&hitEvent)
		
		s.statusEffectApplySystem.process(s.Engine,&hitEvent)
		s.statusEffectUpdateSystem.process(s.Engine,dt,&hitEvent)
		PreDeadSystem(s.Engine)
		s.cleanWallHitSystem.process(s.Engine)
		s.cleanSystem.process(s.Engine)
		// for _,ev :=range s.Engine.FrameEvents{
			// s.emitGlobalEvent(ev.Type,ev)
		// }
		for _,ev := range eventsToTeam{
			s.EmitToTeam(ev.teamID,ev.rawEvent)
		}
		eventsToTeam=eventsToTeam[:0]
		// s.Engine.FrameEvents = s.Engine.FrameEvents[:0]
		s.broadcastState(engine)
		elapsed := time.Since(s.TimeNow)

		// ==========================================
		// THỐNG KÊ VÀ IN KẾT QUẢ RA MÀN HÌNH
		// ==========================================
		totalTickTime += elapsed
		if elapsed > maxTickTime {
			maxTickTime = elapsed
		}
		if elapsed > budget {
			fmt.Printf("⚠️ [LAG SPIKE] Tick bị chậm! Thời gian: %v (Vượt ngân sách 16.66ms)\n", elapsed)
		}
		if tickCount >= TickRate {
			avgTime := totalTickTime / time.Duration(tickCount)
			
		// 	// Tính % CPU mà Game Loop chiếm dụng (Ví dụ: Avg = 8ms thì chiếm 50% của 16ms)
			cpuUsage := float64(avgTime) / float64(budget) * 100.0

			fmt.Printf("[Performance 1s] Avg: %-8v | Max: %-8v | CPU Load: %.1f%%| Alive %d\n", avgTime, maxTickTime, cpuUsage,s.alivePlayer)

		// 	// Reset bộ đếm cho giây tiếp theo
			totalTickTime = 0
			maxTickTime = 0
			tickCount = 0
		}

	}

}
func (s *GameServer) spawnBotsToFill() {
	// //////fmt.println("🌍 Khởi tạo Thế giới: Đẻ Bot!")
	for i := s.nextPlayerID; i < 50; i++ {
		botNetID := i
		
		e := s.Engine.CreateEntity()
// addComponent(s.Engine, e, TagBot{}) // Để AI System nhận diện
addComponent(s.Engine, e, Transform{X: float32(rand.Intn(int(MapSize))), Y: float32(rand.Intn(int(MapSize)))})
addComponent(s.Engine, e, Velocity{})
addComponent(s.Engine, e, Collider{Radius: 25, ShapeType: def.ShapeCircle})
addComponent(s.Engine, e, Intention{}) // AI sẽ điều khiển Intention, thay vì người chơi!
addComponent(s.Engine, e, Vitality{HP: 1000, MaxHP: 1000})
addComponent(s.Engine, e, StatSheet{BaseSpeed: def.StatBaseSpeed, CurrSpeed: def.StatBaseSpeed})
addComponent(s.Engine, e, SkillCooldowns{})
addComponent(s.Engine, e, Equipment{PrimaryElement: Element_Lightning, ActiveSlot: 1})
addComponent(s.Engine, e, Faction{TeamID: uint8(botNetID)})

		
		// s.NetToEntity[botNetID] = e
		s.nextPlayerID++
	}
}
func (s *GameServer) broadcastState(engine *UdpEngine) {
	now:=time.Now()
	s.writer.Reset()
	
	s.writer.WriteUint8(0xAA)
	s.writer.WriteUint8(s.MatchState)
	s.writer.WriteFloat32(s.Zone.X)
	s.writer.WriteFloat32(s.Zone.Y)
	s.writer.WriteFloat32(s.Zone.Radius)
	countOffset := len(s.writer.Buf)
	s.writer.WriteUint8(0)
	activeCount := 0
	RunSystem3(s.Engine,func(count int, entities []Entity, nets []NetSync, pos []Transform, hps []Vitality) {
		for i:=0; i < count;i++{
			if hps[i].HP <= 0 {
				continue
			}
			net := nets[i].NetID
			s.writer.WriteUint8(uint8(net))  //snapshotBuf[idx] = uint8(net
			s.writer.WriteFloat32(pos[i].X)
			s.writer.WriteFloat32(pos[i].Y)
			s.writer.WriteUint16(uint16(hps[i].HP))
			activeCount++
		}
	})


	if activeCount == 0 { return }

	s.writer.Buf[countOffset] = uint8(activeCount)
	s.writer.WriteUint8(0xFF)

	s.addrMutex.Lock()
	countEventOffet :=len(s.writer.Buf)
	s.writer.WriteUint8(0)


	for i := uint16(0); i < s.nextPlayerID;i++{
		if s.Clients[i].Addr!=nil{
			count:=0
			isKicked :=false
			s.writer.Buf=s.writer.Buf[:countEventOffet+1]
			for j:= 0 ; j < len(s.Clients[i].PendingEvents);j++{
				if now.Sub( s.Clients[i].PendingEvents[j].SentAt) > 3*time.Second {
					fmt.Printf("[Server] Client %d lag quá, Kick!\n", i)
					s.disconnectPlayer(i) // Hàm tự định nghĩa để xóa player
					isKicked=true
					
					break 
				}
			}
			if isKicked{
				continue
			}
			for j := 0;j<len(s.Clients[i].PendingEvents);j++{
				evt :=&s.Clients[i].PendingEvents[j] 
				if (evt.Sendtries>0&&now.Sub(evt.SentAt)<200 *time.Millisecond) || evt.Sendtries>5{
					continue 
				}
				evt.SentAt=now
				evt.Sendtries++
				count++
				s.writer.WriteUint16(evt.Event.SeqID)
				s.writer.WriteUint8( evt.Event.Type)
				s.writer.WriteUint16(uint16(evt.Event.Len))
				s.writer.WriteBytes(evt.Event.Payload[:evt.Event.Len])
				// if evt.Event.Type == EventWelcome{
				// 	//fmt.println("ID cua nguoi choi ", evt.Event.Payload," time ",evt.SentAt)
				// }
			}
			for j:=0;j<len(s.Clients[i].PendingLarge);j++{
				evt :=&s.Clients[i].PendingLarge[j] 
				if (evt.Sendtries>0&&time.Since(evt.SentAt)<200 *time.Millisecond) || evt.Sendtries>15{
					continue 
				}
				evt.SentAt=now
				evt.Sendtries++
				count++
				s.writer.WriteUint16(evt.Event.SeqID)
				s.writer.WriteUint8( evt.Event.Type)
				s.writer.WriteUint16(uint16(len(evt.Event.Payload)))
				s.writer.WriteBytes(evt.Event.Payload[:])				
			}
			s.writer.Buf[countEventOffet]=byte(count)
			//////fmt.println("send ",len(s.writer.Bytes()), " for client ",i)
			isFull := engine.QueueToSend(s.writer.Bytes(), s.Clients[i].Addr)
			if isFull {
				engine.FlushSend()
			}

		}

	}
	
	s.addrMutex.Unlock()
	engine.FlushSend()
}
func (s *GameServer) disconnectPlayer(netID uint16) {
	// 1. TÌM VÀ XÓA THỰC THỂ TRONG ECS
	// Khóa Mutex nếu hàm này được gọi từ nhiều luồng (ví dụ: luồng đọc mạng và luồng Update)
	// (Nếu chỉ gọi trong luồng StartLoop thì không cần khóa Engine)
	 client := &s.Clients[netID]

	 if client.Addr != nil {
		s.Engine.RemoveEntity(client.Entity) 
		fmt.Printf("[Server] Đã xóa Entity %d của NetID %d khỏi ECS.\n", client.Entity, netID)
	}

	// 2. DỌN DẸP TÀI NGUYÊN MẠNG (ĐẢM BẢO THREAD-SAFE BẰNG MUTEX)
	// s.addrMutex.Lock()
	// defer s.addrMutex.Unlock()              

	addr := s.Clients[netID].Addr
	if addr != nil {
		arrHash := hashRawAddr(addr)
		delete(s.playerAddrs, arrHash) // Gỡ địa chỉ IP khỏi sổ
		s.Clients[netID].Addr = nil       // Hủy Endpoint
	}

	// 3. DỌN SẠCH HÀNG ĐỢI SỰ KIỆN VÀ INPUT (ZERO-ALLOCATION)
	s.Clients[netID].PendingEvents =s.Clients[netID].PendingEvents [:0] 
	s.ClientInputs[netID].Store(0)

	s.alivePlayer--
}
func (s *GameServer) logicGameOver() {
	// Ở trạng thái GameOver, CHÚNG TA KHÔNG GỌI CÁC SYSTEM VẬT LÝ, BẮN SÚNG NỮA
	// (Game sẽ bị đóng băng hoàn toàn trên màn hình Client)

	// Đếm ngược 10 giây (TickRate * 10 = 600 ticks)
	if s.TickCount > TickRate * 10 {
		//////fmt.println("🛑 KẾT THÚC ĐẾM NGƯỢC. RESET SERVER CHO VÁN MỚI...")
		os.Exit(0)
		// s.resetServer()
	}
}
func SpawnMapObjects(e *ArchEngine) {
	//////fmt.println("🌳 Bắt đầu rải Môi trường lên bản đồ...")

	// 1. Sinh 20 Bụi Cỏ (Tàng hình)
	for i := 0; i < 20; i++ {
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
	for i := 0; i < 15; i++ {
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
func (s *GameServer) CacheMapData() {
	s.writer.Reset()

	// ==========================================
	// 1. GHI BỤI CỎ (Yêu cầu: TagBush, Position, Collider)
	// ==========================================
	// Vì Bụi cỏ cần 3 Component, ta dùng RunSystem3
	
	// A. Lưu nháp vị trí con trỏ để lát ghi Số Lượng (numBushes)
	bushCountOffset := len(s.writer.Buf) 
	s.writer.WriteUint8(0) // Ghi tạm số 0, lát quay lại sửa
	
	totalBushes := 0

	RunSystem3(s.Engine, func(count int, entities []Entity, tags []TagBush, pos []Transform, cols []Collider) {
		totalBushes += count
		for i := 0; i < count; i++ {
			s.writer.WriteFloat32(pos[i].X)
			s.writer.WriteFloat32(pos[i].Y)
			s.writer.WriteFloat32(cols[i].Radius)
		}
	})
	
	// B. Quay lại sửa số 0 thành số lượng thật sự
	s.writer.Buf[bushCountOffset] = byte(totalBushes)


	// ==========================================
	// 2. GHI TƯỜNG (Yêu cầu: TagWall, Position, BoxCollider)
	// ==========================================
	wallCountOffset := len(s.writer.Buf)
	s.writer.WriteUint8(0) 
	totalWalls := uint8(0)

	RunSystem3(s.Engine, func(count int, entities []Entity, tags []TagWall, pos []Transform, boxes []Collider) {
		totalWalls += uint8(count)
		for i := 0; i < count; i++ {
			s.writer.WriteFloat32(pos[i].X)
			s.writer.WriteFloat32(pos[i].Y)
			s.writer.WriteFloat32(boxes[i].Width)
			s.writer.WriteFloat32(boxes[i].Height)
		}
	})

	s.writer.Buf[wallCountOffset] = totalWalls 


	// // ==========================================
	// // 3. GHI ITEM TRÊN ĐẤT (Yêu cầu: TagItem, Position, ItemData)
	// // ==========================================
	// itemCountOffset := len(s.writer.Buf)
	// s.writer.WriteUint16(0)
	// totalItems := uint16(0)

	// RunSystem3(s.Engine, func(count int, entities []Entity, tags []TagItem, pos []Transform, items []ItemData) {
	// 	totalItems += uint16(count)
	// 	for i := 0; i < count; i++ {
	// 		s.writer.WriteUint32(uint32(entities[i])) // ID Thực thể để Client xóa khi có Event Nhặt
	// 		s.writer.WriteFloat32(pos[i].X)
	// 		s.writer.WriteFloat32(pos[i].Y)
	// 		s.writer.WriteUint8(items[i].ItemType)
	// 		s.writer.WriteUint16(items[i].Value)
	// 	}
	// })

	// s.writer.Buf[itemCountOffset] = byte(totalItems >> 8)
	// s.writer.Buf[itemCountOffset+1] = byte(totalItems)

	// ==========================================
	// CUỐI CÙNG: GỬI GÓI HÀNG
	// ==========================================
	payload := make([]byte, len(s.writer.Buf))
	copy(payload,s.writer.Buf)
	s.mapDataCache = payload
}