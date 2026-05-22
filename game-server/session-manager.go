package main

import (
	"fmt"
	def "game/pkg"
	"math/bits"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
)


type NetAckPacket struct{
	HighestSeq uint16 
	Mask uint32
	NetID uint16
}
type NetworkState struct{
	LastTick uint64
	Entity Entity
	Cursor uint32
	NextPacketSeq uint16
	IsDisconnected bool
}
type NetworkEndpoint struct{
	Addr *unix.RawSockaddrAny
	NetID uint16	
	TeamID uint16
}
type EventHeader struct{
	Head,Tail uint32
	Cursor uint32
	TeamID uint16
	IsActive bool
}
type EventData struct{
	EventID uint32
	IsReceived bool
	PacketSeq uint16
	LastSentTick uint64
	// Payload []byte
}
type ClientsSoA struct{
	States [MaxPlayers]NetworkState
	Endpoints [MaxPlayers]NetworkEndpoint
	Events_Header [MaxPlayers]	EventHeader
	Events_Data [MaxPlayers][MaxEvents]EventData
}
const MaxEvents = 1<<12
// type Events struct{

// }
// func (e *Events)Push(eventID uint32){
// 	// fmt.Println("them event id ", eventID, " vao queue ", e.Head)
// 	idx := e.Head & (MaxEvents - 1)
// 	e.EventIDs[idx] = eventID
// 	e.Head++
// }
type SessionManager struct{
	clientAddrs map[uint64]uint16
	nextClientID uint16
	clientSOA *ClientsSoA
	acks []NetAckPacket

}
func NewSessionManager()*SessionManager{
	return &SessionManager{
		clientAddrs: make(map[uint64]uint16),
		nextClientID: 0,
		clientSOA: &ClientsSoA{
		},
	}
}
// func (s *SessionManager) GetClient(id uint16) ClientRef {
//    	return s.clientSOA.GetClient(id)
// }
func ( s *SessionManager)RegisterOrGet(addr *unix.RawSockaddrAny,pendingIds *[]uint16,currentTick uint64, head uint32)(uint16,bool){
	// fmt.Println("vo register or get roi")
	addrHash := hashRawAddr(addr)


	if id, ok := s.clientAddrs[addrHash]; ok {
		return id, true
	}

	if s.nextClientID < MaxPlayers {
		newID := s.nextClientID
		
		s.clientAddrs[addrHash] = newID
		
		s.nextClientID++

		// s.clientSOA.NewClient(newID,addr)
		s.clientSOA.Endpoints[newID]=NetworkEndpoint{
			Addr: addr,
			NetID: newID,
			TeamID: newID,
		}
		s.clientSOA.States[newID]=NetworkState{
			LastTick: currentTick,
			Entity: 0,
			IsDisconnected: false,
			NextPacketSeq: 1,
			Cursor: head,
		}
		s.clientSOA.Events_Header[newID] = EventHeader{
			Head: 0,
			Tail: 0,
			Cursor: head,
			TeamID: newID,
			IsActive: true,
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
	dst := new(unix.RawSockaddrAny) 
	*dst = *src
	return dst
}
func( s *SessionManager)ProcessRawPackets(buffer *PacketBuffer,inputs *[MaxPlayers]atomic.Uint64,pendingIds *[]uint16, state *MatchState, currentTick uint64 , head uint32) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	for _,packet := range buffer.Packets{
		netID,ok := s.RegisterOrGet(packet.Addr,pendingIds,currentTick, head)
		if !ok  { continue }
		if netID >= MaxPlayers { continue } 
		// client := s.GetClient(netID)
		if !s.clientSOA.States[netID].IsDisconnected{
			s.clientSOA.States[netID].IsDisconnected=false
		}
		s.clientSOA.States[netID].LastTick=state.TickCount
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
		s.acks=append(s.acks, NetAckPacket{
			HighestSeq: highest,
			Mask: mask,
			NetID: netID,
		})
		// client.processAckClient(highest, mask)
		
	}
	buffer.ResetLocked()
}

func( s *SessionManager)SyncNetEntity(accpets  *[]MapNetEntity, globalEvent *GlobalEvent){

	for _,c:=range *accpets{
		ev :=RawEvent{
			Type: def.EventWelcome,
		}
		ev.WriteUint16(c.NetID)
		mask := VisibilityMask{}
		mask.Set(c.TeamID)
		if c.NetID==200{
			fmt.Println("sync net entity ", c.NetID, " team ", c.TeamID, " event type ", ev.Type, " payload ", ev.Payload[:ev.Len]," head ",globalEvent.Head)
		}
		// evID :=
		globalEvent.Push(ev, mask)
		// s.clientSOA.Events[c.NetID].Push(evID)
		// fmt.Printf("Đồng bộ entity NetID %d vào client team %d\n", c.NetID, c.TeamID)
	}
	(*accpets)=(*accpets)[:0]
}

func (s *SessionManager) CheckTimeouts(currentTick uint64, delEntitys *[]Entity) {

	for i := uint16(0); i < s.nextClientID; i++ {
		// client := s.GetClient(i)
		if s.clientSOA.Endpoints[i].Addr == nil || s.clientSOA.States[i].IsDisconnected {
			continue
		}

		timeSinceLastAck := currentTick - s.clientSOA.States[i].LastTick

		if timeSinceLastAck > 100 {
			s.clientSOA.States[i].IsDisconnected = true
			fmt.Printf("[GameServer] Client %d có dấu hiệu rớt mạng (Timeout 1.6s). Current Tick %d - Lasttick %d - netID %d\n", i,currentTick,s.clientSOA.States[i].LastTick,s.clientSOA.Endpoints[i].NetID)
		}

		if timeSinceLastAck > 600 {
			fmt.Printf("[GameServer] Client %d mất kết nối hoàn toàn! Xóa khỏi ECS.\n", i)
			
			(*delEntitys)=append((*delEntitys), s.clientSOA.States[i].Entity)


			addrHash := hashRawAddr(s.clientSOA.Endpoints[i].Addr)
			delete(s.clientAddrs, addrHash) 
			
			s.clientSOA.Endpoints[i].Addr = nil
			s.clientSOA.States[i].Entity = 0
			s.clientSOA.States[i].IsDisconnected = true		
			s.clientSOA.Events_Header[i].IsActive = false														
		}
	}
}
// func (s *SessionManager) FlushToQueue(globalEvent *GlobalEvent) {
// 	if s.nextClientID > 0 {
// 		// Assert ngoài loop để xóa Bounds Check của Events_Header[i] và Events_EventIDs[i]
// 		_ = s.clientSOA.Events_Header[s.nextClientID-1]
// 		_ = s.clientSOA.Events_Data[s.nextClientID-1]
// 	}

// 	currentGlobalHead := globalEvent.Head
	
// 	// Cắt lát Masks để ép Go Compiler xóa bỏ hoàn toàn Bounds Check của masks[idx]
// 	masks := globalEvent.Masks[:GlobalEventMask+1]

// 	for i := uint16(0); i < s.nextClientID; i++ {
// 		if s.clientSOA.Events_Header[i].IsActive == false {
// 			continue
// 		}

// 		// Biến thành Slice và Assert ngoài loop j để xóa Bounds Check của eventIDs[...]
// 		eventIDs := s.clientSOA.Events_Data[i][:]
// 		_ = eventIDs[MaxEvents-1]

// 		teamID := s.clientSOA.Events_Header[i].TeamID
// 		maskTeamID := teamID >> 6
		
// 		// Assert maskTeamID một lần duy nhất ngoài loop j để xóa Bounds Check của KnownByTeams
// 		_ = masks[0].KnownByTeams[maskTeamID]

// 		maskAndTeamID := uint64(1) << (teamID & 63)

// 		head := s.clientSOA.Events_Header[i].Head
// 		cursor := s.clientSOA.Events_Header[i].Cursor

// 		for j := cursor; j < currentGlobalHead; j++ {
// 			idx := j & (GlobalEventMask)
			
// 			// ĐÃ SẠCH BÓNG BOUNDS CHECK!
// 			if masks[idx].KnownByTeams[maskTeamID]&maskAndTeamID == 0 {
// 				continue
// 			}
			
// 			// ĐÃ SẠCH BÓNG BOUNDS CHECK!
// 			eventIDs[head&(MaxEvents-1)].EventID = j
// 			head++
// 		}
// 		sumEvent+=int(head - s.clientSOA.Events_Header[i].Head)
// 		s.clientSOA.Events_Header[i].Head = head
// 		s.clientSOA.Events_Header[i].Cursor = currentGlobalHead
// 	}
// }


func (s *SessionManager) FlushToQueue(globalEvent *GlobalEvent) {
	currentHead := globalEvent.Head

	// BƯỚC 1: Tìm cursor nhỏ nhất trong các Player đang hoạt động
	minCursor := currentHead
	for i := uint16(0); i < s.nextClientID; i++ {
		if s.clientSOA.Events_Header[i].IsActive && s.clientSOA.Events_Header[i].Cursor < minCursor {
			minCursor = s.clientSOA.Events_Header[i].Cursor
		}
	}

	// Xác định số Block 64-bit cần quét dựa trên số Client thực tế (Tối ưu cho 1000 players)
	// Ví dụ: nextClientID = 1000 -> maskBlocks = 16
	maskBlocks := uint16(s.nextClientID + 63) / 64

	// BƯỚC 2: QUÉT TUẦN TỰ EVENTS (Perfect Sequential Read - L1 Hit 100% cho Masks)
	for j := minCursor; j < currentHead; j++ {
		idx := j & GlobalEventMask
		mask := &globalEvent.Masks[idx]

		// BƯỚC 3: GIẢI NÉN MASK THEO KHỐI 64-BIT
		for b := uint16(0); b < maskBlocks; b++ {
			bitsVal := mask.KnownByTeams[b]
			
			// NẾU KHỐI NÀY BẰNG 0 -> SKIP NGAY 64 PLAYERS TRONG 1 CYCLE
			// Vì mỗi player chỉ nhận 20/1600 events, 98% các block này sẽ bằng 0!
			if bitsVal == 0 {
				continue
			}

			// Chỉ giải nén những bit thực sự có giá trị 1 (Có người nhìn thấy)
			for bitsVal != 0 {
				tz := bits.TrailingZeros64(bitsVal)
				playerID := (b * 64) + uint16(tz)

				// Chỉ ghi nếu người chơi này thực sự đang hoạt động và chưa nhận event j
				header := &s.clientSOA.Events_Header[playerID]
				if header.IsActive && header.Cursor <= j {
					eventIDs := &s.clientSOA.Events_Data[playerID]
					head := header.Head
					
					// Ghi thẳng vào mảng đích của Player
					eventIDs[head & (MaxEvents - 1)].EventID = j
					header.Head++
				}

				// Xóa bit đã xử lý
				bitsVal &= ^(uint64(1) << tz)
			}
		}
	}

	// ĐỒNG BỘ CURSOR CHO TOÀN BỘ PLAYERS HOẠT ĐỘNG Ở CUỐI PHIÊN BATCH
	for i := uint16(0); i < s.nextClientID; i++ {
		if s.clientSOA.Events_Header[i].IsActive {
			s.clientSOA.Events_Header[i].Cursor = currentHead
		}
	}
}
// func (s *SessionManager) FlushToQueue(globalEvent *GlobalEvent) {
//     currentHead := globalEvent.Head
    
//     // TRƯỚC TIÊN: Cần biết cursor nhỏ nhất để xác định phạm vi Block
//     minCursor := currentHead
//     for i := uint16(0); i < s.nextClientID; i++ {
//         if s.clientSOA.Events_Header[i].IsActive && s.clientSOA.Events_Header[i].Cursor < minCursor {
//             minCursor = s.clientSOA.Events_Header[i].Cursor
//         }
//     }

//     // BLOCK_SIZE tính toán để mảng globalEvent.Masks lọt thỏm trong L1
//     // 256 events * 16 block masks (8 byte) = 32 KB (Vừa khít L1)
//     const BLOCK_SIZE = 256 

//     // CHIA KHỐI 1600 EVENTS RA (Xử lý theo Lô - Batching)
//     for blockStart := minCursor; blockStart < currentHead; blockStart += BLOCK_SIZE {
//         blockEnd := blockStart + BLOCK_SIZE
//         if blockEnd > currentHead {
//             blockEnd = currentHead
//         }

//         // TẠI ĐÂY: Dữ liệu của globalEvent.Masks[blockStart : blockEnd] ĐÃ NẰM Ở L1
        
//         // VÒNG TRONG: DUYỆT TỪNG PLAYER XỬ LÝ KHỐI NÀY
//         for i := uint16(0); i < s.nextClientID; i++ {
//             header := &s.clientSOA.Events_Header[i]
//             if !header.IsActive { continue }

//             cursor := header.Cursor
//             if cursor >= blockEnd { continue }

//             startJ := cursor
//             if startJ < blockStart { startJ = blockStart }

//             // Lấy thông tin Team của Player i
//             teamID := header.TeamID
//             maskBlockIdx := teamID / 64
//             maskBitFlag := uint64(1) << (teamID % 64)

//             // Cache các con trỏ cục bộ (Rất quan trọng để tối ưu L1 Ghi)
//             eventIDs := &s.clientSOA.Events_Data[i]
//             head := header.Head

//             // LẶP QUA CÁC EVENT TRONG KHỐI 
//             for j := startJ; j < blockEnd; j++ {
//                 idx := j & GlobalEventMask
                
//                 // Cú AND bit kinh điển (Đọc từ L1 vì Cache Blocking)
//                 if (globalEvent.Masks[idx].KnownByTeams[maskBlockIdx] & maskBitFlag) != 0 {
//                     // CÓ THẤY! Ghi vào Queue của Player i
//                     eventIDs[head & (MaxEvents - 1)].EventID = j
//                     head++
//                 }
//             }

//             // Ghi trạng thái trở lại SOA sau khi xong 1 khối
//             header.Head = head
//             header.Cursor = blockEnd
//         }
//     }
// }
func (s *SessionManager)ProcessBatchAck(acks []NetAckPacket){
	// fmt.Println("process ack")
	for _,ack := range acks{
		netID := ack.NetID
		// ack.
		mask := ack.Mask
		// fmt.Println("netID ", netID , "ack ",ack)
		// queue := &s.clientSOA.Events[netID]
		header := &s.clientSOA.Events_Header[netID]
		head := header.Head
		tail := header.Tail
		data := &s.clientSOA.Events_Data[netID]

		for j := tail; j < head; j++{
			idx := j & (MaxEvents - 1)
			seq := data[idx].PacketSeq
			
			if data[idx].IsReceived{
				continue
			}
			// fmt.Println("toi day roi A ")
			if seq == ack.HighestSeq {
				data[idx].IsReceived = true
				continue
			}
			
			dist := ack.HighestSeq - seq
			// fmt.Println("toi day roi B ","dist ", dist, " seq ", seq, " highest ", ack.HighestSeq)
			if dist > 32{
				continue 
			}
			// fmt.Println("toi day roi C" , dist)
			if dist == 0 {
				data[idx].IsReceived = true
				// fmt.Println("ack event id ", queue.EventIDs[idx], " seq ", seq, " mask ", mask)
				continue
			}
			if mask &(1<<(dist-1)) != 0{
				// fmt.Println("ack event id ", queue.EventIDs[idx], " seq ", seq, " mask ", mask)
				data[idx].IsReceived = true
			}
		}

		for j := tail; j < head; j++{
			idx := j & (MaxEvents - 1)
			if !data[idx].IsReceived{
				break
			}
			data[idx].IsReceived = false
			tail++
		}
		header.Tail = tail
	}
	s.acks= s.acks[:0]
}


