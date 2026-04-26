package main

import (
	"fmt"
	def "game/pkg"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
)



type SessionManager struct{
	clientAddrs map[uint64]uint16
	nextClientID uint16
	// clients [MaxPlayers]Client
	clientSOA *ClientsSoA
}
func NewSessionManager()*SessionManager{
	return &SessionManager{
		clientAddrs: make(map[uint64]uint16),
		nextClientID: 0,
		clientSOA: &ClientsSoA{
		},
	}
}
func (s *SessionManager) GetClient(id uint16) ClientRef {
   	return s.clientSOA.GetClient(id)
}
func ( s *SessionManager)RegisterOrGet(addr *unix.RawSockaddrAny,pendingIds *[]uint16)(uint16,bool){
	// fmt.Println("vo register or get roi")
	addrHash := hashRawAddr(addr)


	if id, ok := s.clientAddrs[addrHash]; ok {
		return id, true
	}

	if s.nextClientID < MaxPlayers {
		newID := s.nextClientID
		
		s.clientAddrs[addrHash] = newID
		
		s.nextClientID++

		s.clientSOA.NewClient(newID,addr)

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
	dst := new(unix.RawSockaddrAny) 
	*dst = *src
	return dst
}
func( s *SessionManager)ProcessRawPackets(buffer *PacketBuffer,inputs *[MaxPlayers]atomic.Uint64,pendingIds *[]uint16, state *MatchState){
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	for _,packet := range buffer.Packets{
		netID,ok := s.RegisterOrGet(packet.Addr,pendingIds)
		if !ok  { continue }
		if netID >= MaxPlayers { continue } 
		client := s.GetClient(netID)
		if client.IsDisconnected(){
			client.SetIsDisconnected(false)
		}
		client.SetLastTick(state.TickCount)
		if len(packet.Data) < 6 {
			continue
		}
		reader :=NewPacketReader(packet.Data)
		key := uint64(reader.ReadUint8()) << 32
		angle := uint64(reader.ReadUint16())<<16
		dist := uint64(reader.ReadUint16())
		inputs[netID].Store(key | angle | dist)

		header :=reader.ReadUint8()
		if header!=0xFF{
			continue
		}
		highest := reader.ReadUint16()
		mask := reader.ReadUInt32()
		client.processAckClient(highest, mask)
		
	}
	buffer.ResetLocked()
}
func( s *SessionManager)SyncNetEntity(accpets  *[]MapNetEntity, cacheMapdata []byte){

	for _,c:=range *accpets{
		client := s.GetClient(c.NetID)
		client.SetEntity(c.Entity)
		client.SetTeamID(c.TeamID)
		ev :=RawEvent{
			Type: def.EventWelcome,
		}
		ev.WriteUint16(c.NetID)
		frontEvent :=client.PendingQueue().Claim()
		*frontEvent =ev
		client.PendingQueue().Commit()

	}
	(*accpets)=(*accpets)[:0]
}

func (s *SessionManager) CheckTimeouts(currentTick uint64, delEntitys *[]Entity) {

	for i := uint16(0); i < s.nextClientID; i++ {
		client := s.GetClient(i)
		if client.Addr() == nil || client.IsDisconnected() {
			continue
		}

		timeSinceLastAck := currentTick - client.LastTick()

		if timeSinceLastAck > 100 {
			client.SetIsDisconnected(true)
			fmt.Printf("[GameServer] Client %d có dấu hiệu rớt mạng (Timeout 1.6s). Current Tick %d - Lasttick %d - netID %d\n", i,currentTick,client.LastTick(),client.NetID())
		}

		if timeSinceLastAck > 600 {
			fmt.Printf("[GameServer] Client %d mất kết nối hoàn toàn! Xóa khỏi ECS.\n", i)
			
			(*delEntitys)=append((*delEntitys), client.Entity())


			addrHash := hashRawAddr(client.Addr())
			delete(s.clientAddrs, addrHash) 
			
			client.ClearAddr()
			client.SetEntity(0)
			client.PendingQueue().Clear()
		}
	}
}
func(s *SessionManager ) FlushNetworkOutbox( clientSoA *ClientsSoA,outbox *NetworkOutbox){

	for i:=0;i<int(s.nextClientID);i++{
		client :=s.GetClient(uint16(i))
		if client.Addr() == nil || client.IsDisconnected() {
            continue
        }
		netID:=client.NetID()
		teamID := client.TeamID()
		client.PendingQueue().PushBatch(outbox.Teams[teamID])
		client.PendingQueue().PushBatch(outbox.Globals)
		client.PendingQueue().PushBatch(outbox.Privates[netID])
	}
}
