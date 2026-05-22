package main

import (
	"math/bits"
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
type FinalSnapshot struct{
	idxs [MaxPlayers][MaxPlayers]uint16
	counts [MaxPlayers]uint16
}
func (nio *NetworkIO) GatherVisibleSnapshots(countPlayer uint16 ,frameShapshot []SnapShotData, snapshots []SnapShotData, clientsSOA *ClientsSoA, finalSnapshot *FinalSnapshot){
	
	for i := range finalSnapshot.counts {
		finalSnapshot.counts[i] = 0
	}
	sz := (countPlayer+63)>>6

	for i := uint16(0); i < uint16(len(frameShapshot)); i++ {
		mask := frameShapshot[i].Mask
		for j := uint16(0); j < sz; j++ {
			bitsVal := mask.KnownByTeams[j]
			if bitsVal == 0 {
				continue
			}
			for bitsVal!= 0{
				id := (j << 6) + uint16(bits.TrailingZeros64(bitsVal))
				finalSnapshot.idxs[id][finalSnapshot.counts[id]] = i 
				finalSnapshot.counts[id]++
				bitsVal &= bitsVal - 1
			}
		}
		
	}
}
func (nio *NetworkIO) WriteBatch(globalEvent *GlobalEvent, frameShapshot []SnapShotData, clientsSOA *ClientsSoA, currentTick uint64, snapshots *FinalSnapshot) {
	// GatherVisibleSnapshots()
	for i :=0 ; i< MaxPlayers; i++{
		if clientsSOA.States[i].IsDisconnected || clientsSOA.Endpoints[i].Addr == nil{
			continue		 						
		}

		packetSeq := clientsSOA.States[i].NextPacketSeq
		clientsSOA.States[i].NextPacketSeq++
		// eventQueue := &clientsSOA.Events[i]
		nio.writer.Reset()
		nio.writer.WriteUint8(0xAA)   // Magic byte bắt buộc
		// nio.writer.WriteUint8(0x00)  
		nio.writer.WriteUint16(packetSeq)
		nio.writer.WriteUint8(1) // hasSnapshot = 1 (Luôn có)

		nio.writer.WriteUint8(0) // MatchState (Byte bỏ qua)
		
		// Ghi Tọa độ vòng bo (Bạn thay thế biến tương ứng trong MatchState của bạn)
		nio.writer.WriteFloat32(0.0) // zX (Ví dụ: matchState.ZoneX)
		nio.writer.WriteFloat32(0.0) // zY (Ví dụ: matchState.ZoneY)
		nio.writer.WriteFloat32(0.0) // zRad (Ví dụ: matchState.ZoneRad)
		
		playerCountIDx := nio.writer.Pos// Placeholder cho số lượng player
		nio.writer.WriteUint16(0)
		count := uint16(0)

		for j:= uint16(0); j< snapshots.counts[i]; j++{
			idx := snapshots.idxs[i][j]
			snap := &(frameShapshot)[idx]
			nio.writer.WriteUint16(snap.NetID)
			nio.writer.WriteFloat32(snap.X)
			nio.writer.WriteFloat32(snap.Y)
			nio.writer.WriteUint16(snap.HP)
			count++
		}
		nio.writer.Buf[playerCountIDx] = byte(count>>8)
		nio.writer.Buf[playerCountIDx+1] = byte(count )
		// fmt.Println("playerCOunt ",count)
		// fmt.Println("frame ", frameShapshot)
		
		nio.writer.WriteUint8(0xFF)
		eventCountIDx := nio.writer.Pos
		nio.writer.WriteUint8(0) // Placeholder cho số lượng event
		eventCount := uint8(0)
		head := clientsSOA.Events_Header[i].Head
		tail := clientsSOA.Events_Header[i].Tail
		data := &clientsSOA.Events_Data[i]
		for j := tail; j < head; j++{

			idx := j & (MaxEvents - 1)
			if data[idx].IsReceived {
				continue
			}
			lastSent := data[idx].LastSentTick
			if data[idx].PacketSeq == 0 || (currentTick - lastSent > 10) {
				// fmt.Printf("[DEBUG] currentTick %d - lastSentTick %d Client %d, Checking queue slot j=%d (idx=%d), EventID=%d\n", currentTick, lastSent, i, j, idx, eventQueue.EventIDs[idx])				
				// fmt.Println("Dist ", currentTick - lastSent, " -" , (currentTick - lastSent > 10),"or ",eventQueue.PacketSequences[idx] == 0 )
				
				data[idx].LastSentTick = currentTick
				data[idx].PacketSeq = packetSeq
				
				evID := data[idx].EventID & GlobalEventMask

				ev := &globalEvent.Events[evID]
		
				nio.writer.WriteUint8(ev.Type)
				nio.writer.WriteUint16(uint16(ev.Len)) 
				nio.writer.WriteBytes(ev.Payload[:ev.Len])
				// fmt.Println("gui event id ", evID, " len ", ev.Len, " type ", ev.Type, " payload ", ev.Payload[:ev.Len])
				eventCount++
			}
			if eventCount == 255 {
				break 
			}
			// fmt.Println("data ", nio.writer.Buf[:nio.writer.Pos])
		}
		nio.writer.Buf[eventCountIDx] = byte(eventCount)
		// fmt.Println("event count ",eventCount)
		// nio.writer.Buf[eventCountIDx+1] = byte(eventCount >> 8)
		
		// DEBUG: In toàn bộ bytes
		payload := nio.writer.Buf[:nio.writer.Pos]
		// fmt.Printf("PACKET HEX: ")
		// for i, b := range payload {
		// 	fmt.Printf("[%d]=%02x ", i, b)
		// }
		// fmt.Printf("\n")
		
		nio.engine.QueueToSend(payload, clientsSOA.Endpoints[i].Addr)  
		
	}
	nio.engine.FlushSend()

}
