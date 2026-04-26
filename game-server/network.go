package main

import (
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)
type NetworkIO struct{
	engine *UdpEngine
	inputs *[MaxPlayers]atomic.Uint64	
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
func NewNetworkIO(engine *UdpEngine, inputs *[MaxPlayers]atomic.Uint64)*NetworkIO{
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
// TẠI FILE: network.go

func (n *NetworkIO) BroadcastState(state *MatchState, clientSOA *ClientsSoA, outbox *NetworkOutbox) {
	
	// Khai báo sẵn các biến dùng chung ngoài vòng lặp để tránh cấp phát lại
	tickCount := state.TickCount
	matchState := state.MatchState
	zoneX, zoneY, zoneRadius := state.Zone.X, state.Zone.Y, state.Zone.Radius

	// DUYỆT QUA TẤT CẢ CLIENT
	for i := uint16(0); i < MaxPlayers; i++ {
		
		// 1. KIỂM TRA ĐIỀU KIỆN (Dữ liệu Nóng - truy cập cực nhanh)
		addr := clientSOA.Addrs[i]
		if addr == nil || clientSOA.IsDisconnected[i] {
			continue
		}

		// 2. CHUẨN BỊ CON TRỎ & SLICE (Lấy 1 lần duy nhất cho toàn bộ quy trình)
		teamID := clientSOA.TeamIDs[i]
		queue := &clientSOA.EventQueues[i]
		inflightHeader := &clientSOA.Inflights[i]
		inflightDataArray := &clientSOA.InFlightsEvents[i] // Trỏ tới cục Lạnh 125MB
		clientVision := outbox.Positions[teamID]       // Trỏ tới cục Snapshot

		isFirstPacketOfTick := true

		// VÒNG LẶP GỬI GÓI TIN (Cho đến khi hết Queue)
		for queue.count > 0 || isFirstPacketOfTick {
			
			// --- A. LẤY SEQUENCE (Dữ liệu Nóng) ---
			packetSeq := clientSOA.NextPacketSeqs[i]
			clientSOA.NextPacketSeqs[i]++
			idx255 := packetSeq & 255

			// --- B. KHỞI TẠO BỘ ĐỆM (Writer) ---
			writer := n.writer
			writer.Reset()
			writer.WriteUint8(0xAA)
			writer.WriteUint16(packetSeq)

			// ==========================================
			// --- C. GHI SNAPSHOT (Đọc tuần tự từ mảng Positions) ---
			// ==========================================
			if isFirstPacketOfTick {
				writer.WriteUint8(1)
				writer.WriteUint8(matchState)
				writer.WriteFloat32(zoneX)
				writer.WriteFloat32(zoneY)
				writer.WriteFloat32(zoneRadius)

				countOffset := writer.Pos
				writer.WriteUint16(0) 

				visibleCount := uint16(0)
				for _, p := range clientVision {
					if writer.Pos + 12 > SafeMTU - 60 { break } // Bảo vệ MTU
					
					writer.WriteUint16(p.NetID)
					writer.WriteFloat32(p.X)
					writer.WriteFloat32(p.Y)
					writer.WriteUint16(p.HP)
					visibleCount++
				}
				writer.Buf[countOffset] = byte(visibleCount >> 8)
				writer.Buf[countOffset+1] = byte(visibleCount)
				
				isFirstPacketOfTick = false
			} else {
				writer.WriteUint8(0) 
			}
			writer.WriteUint8(0xFF) // Phân cách Snapshot và Event

			// ==========================================
			// --- D. GHI EVENTS (Đọc từ Queue -> Ghi vào Writer & Inflight) ---
			// ==========================================
			eventCountIdx := writer.Pos
			writer.WriteUint8(0)

			eventsSlice := queue.PeekBatch(MaxEventsPerPkt)
			eventCount := uint8(0)
			
			if len(eventsSlice) > 0 {
				// CẮM MỤC TIÊU VÀO ĐÚNG MẢNG ĐÍCH (Kích hoạt BCE & Prefetch)
				targetInflight := &inflightDataArray[idx255]
				_ = targetInflight[31] // BCE Hint

				currentBufLen := writer.Pos
				
				for evIdx := range eventsSlice {
					ev := &eventsSlice[evIdx] 
					evLen := int(ev.Len) 
					
					if currentBufLen + evLen + 5 > SafeMTU {
						break 
					}
					currentBufLen += evLen
					
					// 1. Backup vào Inflight (Ghi vào mảng Lạnh)
					targetInflight[eventCount] = *ev 
					
					// 2. Đóng gói vào UDP (Ghi vào mảng Nóng của bộ nhớ đệm mạng)
					writer.WriteUint8(ev.Type)
					writer.WriteUint16(uint16(evLen))
					writer.WriteBytes(ev.Payload[:evLen])
					
					eventCount++
				}
				// Tiêu thụ Event khỏi Queue
				queue.ConsumeBatch(int(eventCount))
			}

			// --- E. CẬP NHẬT HEADER & CHỐT SỔ GÓI TIN ---
			writer.Buf[eventCountIdx] = uint8(eventCount)
			
			// Cập nhật Inflight Header (Dữ liệu Nóng)
			inflightHeader.Masks[idx255>>6] |= (1 << (idx255 & 63))
			inflightHeader.PacketSeqs[idx255] = packetSeq
			inflightHeader.Senticks[idx255] = tickCount
			inflightHeader.EventCounts[idx255] = eventCount

			// Quăng xuống card mạng (Tạm lưu vào OS buffer)
			n.engine.QueueToSend(writer.Bytes(), addr)
		}
	}

	// 3. FLUSH TẤT CẢ XUỐNG CARD MẠNG MỘT LẦN CHÓT
	n.engine.FlushSend()
}
type EventQueue struct{
	data [EventQueueSize]RawEvent
	head uint16 
	tail uint16 
	count int
}
func (s *EventQueue) Claim() *RawEvent {
    if s.count < EventQueueSize {
        return &s.data[s.tail]
    }
    return nil
}
func (s *EventQueue) Commit() {
    s.tail = (s.tail + 1) & EventQueueMask
    s.count++
}
func (s *EventQueue) PushBatch(events []RawEvent) {
    n := len(events)
    if n == 0 || s.count+n > EventQueueSize {
        return // Xử lý lỗi đầy hàng đợi tùy bạn
    }
	spaceAtEnd := EventQueueSize-s.tail
	if(n <= int(spaceAtEnd)){
		copy(s.data[s.tail:],events)
	}else{
		copy(s.data[s.tail:],events[:spaceAtEnd])
		copy(s.data[0:],events[spaceAtEnd:])
	}
	s.tail=(s.tail+uint16(n)) & EventQueueMask
	s.count+=n
}
func (s *EventQueue) PeekBatch(max int) []RawEvent {
    if s.count == 0 {
        return nil
    }
    
    n := max
    if n > s.count {
        n = s.count
    }

    spaceAtEnd := EventQueueSize - int(s.head)
    if n > spaceAtEnd {
        n = spaceAtEnd 
    }
    return s.data[s.head : int(s.head)+n]
}

// Bỏ đi N phần tử cùng lúc
func (s *EventQueue) ConsumeBatch(n int) {
    s.head = uint16((int(s.head) + n) & EventQueueMask)
    s.count -= n
}
func (s *EventQueue)Clear(){
	s.count=0
	s.head=0
	s.tail=0
}
