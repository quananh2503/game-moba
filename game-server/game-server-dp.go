package main

import (
	"fmt"
	def "game/pkg"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)
const (

)
// data
type Client struct {
	NetID         uint16
	Addr          *unix.RawSockaddrAny
	Entity Entity
	TeamID uint8
	NextEventSeq  uint16 
	
	PendingEvents []PendingEvent
	PendingLarge  []PendingLargeEvent
	LastTickAck uint64
	IsDisconnected bool
}
//engine 
type SessionManager struct{
	clientAddrs map[uint64]uint16
	nextClientID uint16 
	clients [200]Client
}
func NewSessionManager()*SessionManager{
	return &SessionManager{
		clientAddrs: make(map[uint64]uint16),
		nextClientID: 0,
	}
}
func ( s *SessionManager)RegisterOrGet(addr *unix.RawSockaddrAny,pendingIds *[]uint16)(uint16,bool){
	// fmt.Println("vo register or get roi")
	addrHash := hashRawAddr(addr)


	if id, ok := s.clientAddrs[addrHash]; ok {
		return id, true
	}

	if s.nextClientID < 200 {
		newID := s.nextClientID
		
		s.clientAddrs[addrHash] = newID
		
		s.nextClientID++

		s.clients[newID] = Client{
			NetID: newID,
			Addr: copyRawAddr(addr),
			NextEventSeq: 1,
			PendingEvents: make([]PendingEvent, 0,256),
			PendingLarge: make([]PendingLargeEvent, 0,16),
			
		}
		(*pendingIds)=append((*pendingIds), newID)
		fmt.Printf("[GameServer] Người chơi mới. Gán ID: %d\n",newID )
		return newID, true
	}
	return 0, false	
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
func( s *SessionManager)ProcessRawPackets(buffer *PacketBuffer,inputs *[200]atomic.Uint64,pendingIds *[]uint16, state *MatchState){
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	// fmt.Println(" vo process packet ròi ",len(buffer.Packets))
	for _,packet := range buffer.Packets{
		netID,ok := s.RegisterOrGet(packet.Addr,pendingIds)
		if !ok  { continue }
		if s.clients[netID].IsDisconnected{
			s.clients[netID].IsDisconnected=false
		}
		s.clients[netID].LastTickAck=state.TickCount
		if len(packet.Data) < 5 {
			continue
		}
		key := uint64(packet.Data[0]) << 32
		angle := uint64(packet.Data[1])<<24 | uint64(packet.Data[2])<<16
		dist := uint64(packet.Data[3])<<8 | uint64(packet.Data[4])
		inputs[netID].Store(key | angle | dist)
		s.processAckClient(packet.Data[5:],netID)

		// fmt.Println("net id ",netID , " ",s.clients[netID].LastTickAck)
	}
	buffer.ResetLocked()
	// fmt.Println("thoat ra process roi")
}
func( s *SessionManager)SyncNetEntity(accpets  *[]MapNetEntity, cacheMapdata []byte){
	// fmt.Println("chay toii day ròi")
	for _,c:=range *accpets{
		// fmt.Println("sync client moi ", c.NetID, " enity ", c.Entity)
		s.clients[c.NetID].Entity=c.Entity
		s.clients[c.NetID].TeamID=c.TeamID
		ev :=RawEvent{
			Type: def.EventWelcome,
		}
		ev.WriteUint8(uint8(c.NetID))
		s.clients[c.NetID].Enqueue(ev)
		if cacheMapdata!=nil{
			s.clients[c.NetID].enqueueLargeBytes(def.EventSendMap,cacheMapdata)
		}
		
	}
	(*accpets)=(*accpets)[:0]
}
//data
type MatchState struct {
	MatchState uint8 
	TickCount uint64 
	Zone ZoneInfo
	WinnerID uint16 
	alivePlayer int
	TimeNow time.Time
}
//engine
type NetworkIO struct{
	engine *UdpEngine
	inputs *[200]atomic.Uint64	
	writer *PacketWriter
	largeEventWriter *PacketWriter
}
type RawPacket struct{
	Addr *unix.RawSockaddrAny
	Data []byte 
}
type PacketBuffer struct{
	mu sync.Mutex
	Packets []RawPacket
}
func ( p * PacketBuffer)ResetLocked(){
	// p.mu.Lock()
	// defer p.mu.Unlock()
	p.Packets=p.Packets[:0]
}
func NewNetworkIO(engine *UdpEngine, inputs *[200]atomic.Uint64)*NetworkIO{
	return &NetworkIO{
		engine: engine,
		inputs: inputs,
		writer: NewPacketWriter(8196),
		// largeEventWriter: NewPacketWriter(8196),
	}
} 
func (nio *NetworkIO)  ReadBatch( buffer *PacketBuffer) {
	n ,_:=nio.engine.ReadBatch()
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	for i := 0; i < n; i++ {
		packetLen := nio.engine.recvMsgs[i].Len
		dataCopy := make([]byte, packetLen)
		copy(dataCopy, nio.engine.recvBuffers[i][:packetLen])
		buffer.Packets = append(buffer.Packets, RawPacket{
			Addr: copyRawAddr(&nio.engine.recvAddrs[i]),
			Data: dataCopy,
		})
	}
}


func (s *SessionManager) processAckClient(data []byte, playerID uint16) {


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
	events := s.clients[playerID].PendingEvents
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
		// if isAcked{
		// 	fmt.Println("ack peding event ")
		// }
	}
	// Cắt bỏ phần đuôi thừa (Không tạo slice mới, chỉ đổi len)
	s.clients[playerID].PendingEvents = events[:keep]

	// ---------------------------------------------------------
	// 2. XỬ LÝ PENDING LARGE (Sự kiện lớn - Map)
	// ---------------------------------------------------------
	eventsLarge := s.clients[playerID].PendingLarge
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
		// if isAcked{
		// 	fmt.Println("ack peding large event ")
		// }
	}
	s.clients[playerID].PendingLarge = eventsLarge[:keepLarge]
}
func (s *SessionManager) CheckTimeouts(currentTick uint64, delEntitys *[]Entity) {

	for i := uint16(0); i < s.nextClientID; i++ {
		client := &s.clients[i]
		
		// Đã bị đánh dấu xóa trước đó, bỏ qua
		if client.Addr == nil || client.IsDisconnected {
			continue
		}

		timeSinceLastAck := currentTick - client.LastTickAck

		// MỨC 1: CẢNH BÁO LAG (Ví dụ 100 Ticks ~ 1.6s)
		// Ta sẽ dùng biến IsDisconnected để cấm NetworkIO gửi gói tin cho nó
		if timeSinceLastAck > 100 {
			client.IsDisconnected = true
			fmt.Printf("[GameServer] Client %d có dấu hiệu rớt mạng (Timeout 1.6s). Current Tick %d - Lasttick %d - netID %d\n", i,currentTick,client.LastTickAck,client.NetID)
		}

		// MỨC 2: TỬ HÌNH (Ví dụ 600 Ticks ~ 10s)
		if timeSinceLastAck > 600 {
			fmt.Printf("[GameServer] Client %d mất kết nối hoàn toàn! Xóa khỏi ECS.\n", i)
			
			// 1. Nhờ World xóa Entity trong ECS
			// world.RemovePlayer(client.Entity) 
			(*delEntitys)=append((*delEntitys), client.Entity)

			// 2. Dọn dẹp Network
			addrHash := hashRawAddr(client.Addr)
			delete(s.clientAddrs, addrHash) // Xóa khỏi danh bạ
			
			// Xóa sạch rác trong Queue (Zero-allocation)
			client.Addr = nil
			client.Entity = 0
			client.PendingEvents = client.PendingEvents[:0]
			client.PendingLarge = client.PendingLarge[:0]
		}
	}
}
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
		// numCols := 32 // Giả sử xếp thành một hình vuông 32x32 = 1024 vị trí
		// spacing := MapSize / float32(numCols) // 4000 / 32 = 125 đơn vị

		// posX := float32(int(id) % numCols) * spacing
		// posY := float32(int(id) / numCols) * spacing


		addComponent(w.Engine, e, Transform{X: 0, Y: 0}) 
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
		addComponent(w.Engine,e,SightRange{Radius: 500})
		// ư.clients[id].Entity=e
		// w.sessions.SetEntityByID(id,e)
		(*accpets)=append((*accpets), MapNetEntity{
			NetID: id,
			Entity: e,
			TeamID: uint8(id),
		})
		// ev:=NewEvent(def.EventWelcome)
		// ev.WriteUint8(uint8(id))
		// w.emit(id, ev)
		// if s.mapDataCache !=nil{
		// 	s.emitPrivateLargeEvent(id,s.mapDataCache)
		// }
		// s.alivePlayer++
		// fmt.Println(" gui tin cho client ", id)
	}	
	(*clients)=(*clients)[:0]
}
func( w *World)RemoveEntities(dels *[]Entity){
	for _,e:= range *dels{
		w.Engine.RemoveEntity(e)
	}
	(*dels)=(*dels)[:0]
}
func( w *World)Tick( dt float32,inputs *[200]atomic.Uint64,outbox *NetworkOutbox){
		
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
type PlayerSnapshot struct {
	NetID uint16
	X, Y  float32
	HP    uint16
}

func (n *NetworkIO) BroadcastState(engine *ArchEngine, state *MatchState, clients *[200]Client, outbox *NetworkOutbox) {


	// ==========================================
	// PHA 2: PACK & FILTER TỪNG CLIENT
	// ==========================================
	for i := range clients {
		client := &clients[i]
		if client.Addr == nil || client.IsDisconnected { continue }

		// 1. CHUẨN BỊ BUFFER CHO CLIENT NÀY
		n.writer.Reset() // ĐẶT TRONG VÒNG LẶP! Mỗi người 1 gói riêng
		n.writer.WriteUint8(0xAA)
		n.writer.WriteUint8(state.MatchState)
		n.writer.WriteFloat32(state.Zone.X)
		n.writer.WriteFloat32(state.Zone.Y)
		n.writer.WriteFloat32(state.Zone.Radius)
		
		countOffset := len(n.writer.Buf)
		n.writer.WriteUint8(0) // Giữ chỗ cho số lượng người chơi thấy được
		
		// 2. LỌC NGƯỜI CHƠI THEO TẦM NHÌN
		visibleCount := 0
		clientVision := outbox.Positions[client.TeamID]// Lấy mặt nạ tầm nhìn của Team này

		for _, p := range clientVision {
			// LUẬT HIỂN THỊ: 
			// 1. Luôn nhìn thấy bản thân mình (p.NetID == client.NetID)
			// 2. Hoặc ô lưới của mục tiêu nằm trong vùng sáng của Team
			// if p.NetID == client.NetID || clientVision.Has(p.Cell) {
				n.writer.WriteUint8(uint8(p.NetID))
				n.writer.WriteFloat32(p.X)
				n.writer.WriteFloat32(p.Y)
				n.writer.WriteUint16(p.HP)
				visibleCount++
			// }
		}

		// Điền lại số lượng thực tế
		n.writer.Buf[countOffset] = uint8(visibleCount)
		n.writer.WriteUint8(0xFF) // Header kết thúc Snapshot

		// ==========================================
		// PHA 3: ĐÍNH KÈM SỰ KIỆN (PENDING EVENTS)
		// ==========================================
		eventCountIdx := len(n.writer.Buf)
		n.writer.WriteUint8(0) // Giữ chỗ cho Event Count
		evCount := 0

		// Xử lý Event Nhỏ
		for j := range client.PendingEvents {
			ev := &client.PendingEvents[j]
			if ev.Sendtries > 0 && state.TickCount - ev.SentTick < 10 { continue }
			if ev.Sendtries > 10 { continue }

			ev.SentTick = state.TickCount
			ev.Sendtries++

			n.writer.WriteUint16(ev.Event.SeqID)
			n.writer.WriteUint8(ev.Event.Type)
			n.writer.WriteUint16(uint16(ev.Event.Len))
			n.writer.WriteBytes(ev.Event.Payload[:ev.Event.Len])
			evCount++
		}

		// Xử lý Event Lớn
		for j := range client.PendingLarge {
			ev := &client.PendingLarge[j]
			if ev.Sendtries > 0 && state.TickCount - ev.SentTick < 20 { continue }
			if ev.Sendtries > 10 { continue }

			ev.SentTick = state.TickCount
			ev.Sendtries++

			n.writer.WriteUint16(ev.Event.SeqID)
			n.writer.WriteUint8(ev.Event.Type)
			n.writer.WriteUint16(uint16(len(ev.Event.Payload)))
			n.writer.WriteBytes(ev.Event.Payload[:])
			evCount++
		}

		n.writer.Buf[eventCountIdx] = uint8(evCount)
		n.engine.QueueToSend(n.writer.Bytes(), client.Addr)
	}

	n.engine.FlushSend()
}
func(s *SessionManager ) FlushNetworkOutbox( outbox *NetworkOutbox){
	for _,ev:=range outbox.Globals{
		for i := range s.clients{
			client := &s.clients[i]
			client.Enqueue(ev)
		}
	}

	for _,ev:=range outbox.Teams{
		for i:=range s.nextClientID{
			client := &s.clients[i]
			// if 
			if ev.TeamID == client.TeamID{
				// fmt.Println("add team ", ev.Event)
				client.Enqueue(ev.Event)
			}	
		}
	}
	

	for _,ev := range outbox.Privates{
		client := &s.clients[ev.NetId]
		client.Enqueue(ev.Event)
	}

}
func (c *Client) Enqueue(raw RawEvent) {
	if c.Addr==nil{
		return
	}
	if c.IsDisconnected{
		return
	}
    if raw.Len <= MaxEventPayload {
        c.enqueueSmallEvent(raw)
    } 
	
}
func (c *Client) enqueueSmallEvent(raw RawEvent) {
    ev := GameEvent{Type: raw.Type, Len: raw.Len, Payload: raw.Payload}
    ev.SeqID = c.NextEventSeq
	// fmt.Println(" dong goi rawevent payload ", raw.Payload, " len ",raw.Len)
    c.NextEventSeq++
    c.PendingEvents = append(c.PendingEvents, PendingEvent{
		Event: ev,
	})
}

// Chỉ chịu trách nhiệm móc gói to vào đúng Client
func (c *Client)enqueueLargeBytes(evType uint8, data []byte){
	if c.Addr==nil|| c.IsDisconnected{
		return
	}
	ev :=GameLargeEvent{
		SeqID: c.NextEventSeq,
		Type: evType,
		Payload: data,
	}
	c.PendingLarge=append(c.PendingLarge, PendingLargeEvent{
		Event: ev,
		// SentTick: ,
	})
}
type GameEvent struct{
	SeqID uint16
	Type byte
	Len uint8
	Payload [MaxEventPayload]byte
}
type PendingEvent struct{
	Event GameEvent
	SentTick uint64
	Sendtries uint8
}
type GameLargeEvent struct{
	SeqID uint16
	Type byte
	Payload []byte
}
type PendingLargeEvent struct{
	Event GameLargeEvent
	SentTick uint64
	Sendtries uint8
}

const (
	MapSize      = 4000.0
	TickRate     = 60 // 60 Khung hình / giây

	CellSize = 100
	GridCols = int(MapSize / CellSize)
	GridRows = int(MapSize / CellSize)
	VisionCellSize = 50
	VisionGridCols = int(MapSize / VisionCellSize)
	VisionGridRows = int(MapSize / VisionCellSize)

	Element_Fire    uint8 = 1
	Element_Lightning   uint8 = 2
	Element_Ice     uint8 = 3
	Element_Wind    uint8 = 4
	Element_Stone   uint8 = 5
	Element_Poison uint8 = 6
)
type SpatialEvent struct {
	X, Y  float32
	Event RawEvent
}
type PrivateEvent struct{
	NetId uint16
	Event RawEvent
}
type TeamEvent struct{
	TeamID uint8 
	Event RawEvent
}
type NetworkOutbox struct {
	Globals  []RawEvent
	Privates []PrivateEvent
	Spatials []SpatialEvent
	Teams []TeamEvent
	Positions [256][]PlayerSnapshot
}
func (s *NetworkOutbox)Reset(){
	s.Globals=s.Globals[:0]
	s.Privates=s.Privates[:0]
	s.Spatials=s.Spatials[:0]
	s.Teams=s.Teams[:0]
	for i:= range s.Positions{
		s.Positions[i]=s.Positions[i][:0]
	}
}
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
	writer := NewPacketWriter(8196)

	// ==========================================
	// 1. GHI BỤI CỎ (Yêu cầu: TagBush, Position, Collider)
	// ==========================================
	// Vì Bụi cỏ cần 3 Component, ta dùng RunSystem3

	// A. Lưu nháp vị trí con trỏ để lát ghi Số Lượng (numBushes)
	bushCountOffset := len(writer.Buf)
	writer.WriteUint8(0) // Ghi tạm số 0, lát quay lại sửa

	totalBushes := 0

	RunSystem3(s.world.Engine, func(count int, entities []Entity, tags []TagBush, pos []Transform, cols []Collider) {
		totalBushes += count
		for i := 0; i < count; i++ {
			writer.WriteFloat32(pos[i].X)
			writer.WriteFloat32(pos[i].Y)
			writer.WriteFloat32(cols[i].Radius)
		}
	})

	// B. Quay lại sửa số 0 thành số lượng thật sự
	writer.Buf[bushCountOffset] = byte(totalBushes)

	// ==========================================
	// 2. GHI TƯỜNG (Yêu cầu: TagWall, Position, BoxCollider)
	// ==========================================
	wallCountOffset := len(writer.Buf)
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
	payload := make([]byte, len(writer.Buf))
	copy(payload,writer.Buf)
	s.mapDataCache = payload
}
func NewNetworkOutbox() *NetworkOutbox {
	nw:= &NetworkOutbox{
		Globals:  make([]RawEvent, 0, 100),
		Privates: make([]PrivateEvent,0,500),
		Spatials: make([]SpatialEvent, 0, 500),
		// Positions: ,
	}
	for i:=range nw.Positions{
		nw.Positions[i]=make([]PlayerSnapshot, 0,500)
	}
	return nw
}