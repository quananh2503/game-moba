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
func( s *SessionManager)ProcessRawPackets(buffer *PacketBuffer,inputs *[200]atomic.Uint64,pendingIds *[]uint16){
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	for _,packet := range buffer.Packets{
		netID,ok := s.RegisterOrGet(packet.Addr,pendingIds)
		if !ok  { continue }
		key := uint64(packet.Data[0]) << 32
		angle := uint64(packet.Data[1])<<24 | uint64(packet.Data[2])<<16
		dist := uint64(packet.Data[3])<<8 | uint64(packet.Data[4])
		inputs[netID].Store(key | angle | dist)
	}
	buffer.Reset()
}
func( s *SessionManager)SetEntityByID(id uint16 , e Entity){
	s.clients[id].Entity=e
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
func ( p * PacketBuffer)Reset(){
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Packets=p.Packets[:0]
}
func NewNetworkIO(engine *UdpEngine, inputs *[200]atomic.Uint64)*NetworkIO{
	return &NetworkIO{
		engine: engine,
		inputs: inputs,
		writer: NewPacketWriter(8196),
		largeEventWriter: NewPacketWriter(8196),
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
	}
	s.clients[playerID].PendingLarge = eventsLarge[:keepLarge]
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
	spatialGrid *SpatialGrid

	hitEvents []HitEvent
	overlapEvents []OverlapEvent
}
func NewWord() *World{
	w:= &World{
		spatialGrid: &SpatialGrid{

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
	}
	for i:=range w.TriggerOverlapSystem.spatialGrid{
		w.TriggerOverlapSystem.spatialGrid[i] = make([]TargetCache, 0,100)
	}
	return w
	
}
func ( w *World)AcceptPendingclients(clients *[]uint16){

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
		// element := rand.Intn(5)

		addComponent(w.Engine, e, Equipment{PrimaryElement:uint8(rand.Int()%5+1), ActiveSlot: 1})
		addComponent(w.Engine, e, Faction{TeamID: uint8(id)}) // Tạm thời TeamID = NetID (Đấu đơn)
		addComponent(w.Engine,e,NetSync{NetID: id})
		addComponent(w.Engine,e,ActiveStatusEffects{})
		addComponent(w.Engine,e,SolidBody{})
		// ư.clients[id].Entity=e
		// w.sessions.SetEntityByID(id,e)
		ev:=NewEvent(def.EventWelcome)
		ev.WriteUint8(uint8(id))
		// s.emitPrivateEvent(id, ev)
		// if s.mapDataCache !=nil{
		// 	s.emitPrivateLargeEvent(id,s.mapDataCache)
		// }
		// s.alivePlayer++
		// fmt.Println(" gui tin cho client ", id)
	}	
	(*clients)=(*clients)[:0]
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
		w.bounceSystem.process(w.Engine,outbox)
		w.fragileSystem.process(w.Engine)
		SolidBodySystem(w.Engine)
		MovementSystem(w.Engine,dt)
		TrailEmitterSystem(w.Engine,dt)
		w.TriggerOverlapSystem.process(w.Engine,&w.overlapEvents)
		
		DamageTriggerSystem(w.Engine,dt,&w.overlapEvents,&w.hitEvents)
		
		w.damageApplySystem.process(w.Engine,dt,&w.hitEvents)
		
		w.statusEffectApplySystem.process(w.Engine,&w.hitEvents)
		w.statusEffectUpdateSystem.process(w.Engine,dt,&w.hitEvents)
		PreDeadSystem(w.Engine)
		w.cleanWallHitSystem.process(w.Engine)
		w.cleanSystem.process(w.Engine,outbox)

}
func (n *NetworkIO) BroadcastState(engine *ArchEngine, state *MatchState) {

	n.writer.Reset()
	
	n.writer.WriteUint8(0xAA)
	n.writer.WriteUint8(state.MatchState)
	n.writer.WriteFloat32(state.Zone.X)
	n.writer.WriteFloat32(state.Zone.Y)
	n.writer.WriteFloat32(state.Zone.Radius)
	countOffset := len(n.writer.Buf)
	n.writer.WriteUint8(0)
	activeCount := 0
	RunSystem3(engine,func(count int, entities []Entity, nets []NetSync, pos []Transform, hps []Vitality) {
		for i:=0; i < count;i++{
			if hps[i].HP <= 0 {
				continue
			}
			net := nets[i].NetID
			n.writer.WriteUint8(uint8(net))  //snapshotBuf[idx] = uint8(net
			n.writer.WriteFloat32(pos[i].X)
			n.writer.WriteFloat32(pos[i].Y)
			n.writer.WriteUint16(uint16(hps[i].HP))
			activeCount++
		}
	})


	if activeCount == 0 { return }

	n.writer.Buf[countOffset] = uint8(activeCount)
	n.writer.WriteUint8(0xFF)

	n.engine.FlushSend()
}
func(s *SessionManager ) FlushNetworkOutbox(grid *SpatialGrid, outbox *NetworkOutbox){
	for _,ev:=range outbox.Globals{
		for i := range s.clients{
			client := &s.clients[i]
			client.Enqueue(ev)
		}
	}
	outbox.Globals=outbox.Globals[:0]
	for _,ev:=range outbox.Spatials{
		cell := getGridIndex(ev.X,ev.Y)
		for i:=range s.clients{
			client := &s.clients[i]
			if grid.TeamVisions[client.TeamID].Has(cell){
				client.Enqueue(ev.Event)
			}
		}
	}
	outbox.Spatials=outbox.Spatials[:0]
	for _,ev := range outbox.Privates{
		client := &s.clients[ev.NetId]
		client.Enqueue(ev.Event)
	}
	outbox.Privates=outbox.Privates[:0]


}
func (c *Client) Enqueue(raw RawEvent) {
	if c.Addr==nil{
		return
	}
    if raw.Len <= MaxEventPayload {
        c.enqueueSmallEvent(raw)
    } else {
        c.enqueueLargeEvent(raw)
    }
}
func (c *Client) enqueueSmallEvent(raw RawEvent) {
    ev := GameEvent{Type: raw.Type, Len: raw.Len, Payload: raw.Payload}
    ev.SeqID = c.NextEventSeq
    c.NextEventSeq++
    c.PendingEvents = append(c.PendingEvents, PendingEvent{Event: ev})
}

// Chỉ chịu trách nhiệm móc gói to vào đúng Client
func (c *Client) enqueueLargeEvent(raw RawEvent) {
    payloadCopy := make([]byte, raw.Len) // Chỉ copy khi là gói lớn
    copy(payloadCopy, raw.Payload[:raw.Len])

    ev := GameLargeEvent{Type: raw.Type, Payload: payloadCopy}
    ev.SeqID = c.NextEventSeq
    c.NextEventSeq++
    c.PendingLarge = append(c.PendingLarge, PendingLargeEvent{Event: ev})
}