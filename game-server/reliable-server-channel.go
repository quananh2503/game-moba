package main

import "math/bits"


type InFlightBuffer struct{
	Masks [4]uint64
	PacketSeqs [256]uint16 
	Senticks [256]uint64 
	EventCounts [256]uint8 
}
func (i *InFlightBuffer)AddPacket( seq uint16, tick uint64, count uint8 )  {
	idx := seq &255 
	i.Masks[idx>>6]|= (1 <<(idx&63))
	i.PacketSeqs[idx]=seq 
	i.Senticks[idx]=tick
	i.EventCounts[idx]=count
}
func ( b *InFlightBuffer)ProcessAck(highest uint16, mask uint32 , pendingEvents *EventQueue){

	for i  := range b.Masks{
		maskLoop:=b.Masks[i]
		if maskLoop == 0 {
			continue
		}
		newMask :=maskLoop
		
		for maskLoop!=0{
			t := uint16(bits.TrailingZeros64(maskLoop))
			bitToRemove := uint64(1) << t
			maskLoop&=^bitToRemove
			id := uint16(i<<6)|t
			if b.PacketSeqs[id] == highest  {
				newMask&=^bitToRemove
				continue
			}
			dist := highest-b.PacketSeqs[id]
			if dist > 32768 { continue }
			if(dist<=MaxEventsPerPkt ){
				if ( (mask&(1<<(dist-1))) != 0){
					newMask&=^bitToRemove
				}	
				continue
			}

			for j:=uint8(0);j<b.EventCounts[id];j++{
				// pendingEvents.Pu(b.Events[id][j])
			}
			newMask&=^bitToRemove
		}
		b.Masks[i]=newMask

	}
}


