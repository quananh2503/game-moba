package main

import "golang.org/x/sys/unix"

type ClientsSoA struct{
	NetIDs [MaxPlayers]uint16 
	Addrs [MaxPlayers] *unix.RawSockaddrAny
	Entities [MaxPlayers]Entity
	TeamIDs [MaxPlayers]uint8 
	
	LastTicks [MaxPlayers]uint64
	IsDisconnected [MaxPlayers]bool 
	NextPacketSeqs [MaxPlayers]uint16 
	
	Inflights [MaxPlayers]InFlightBuffer
	EventQueues [MaxPlayers]EventQueue 
	InFlightsEvents [MaxPlayers][256][32]RawEvent
}
func (c * ClientsSoA )NewClient(NetID uint16, addr *unix.RawSockaddrAny){
	c.NetIDs[NetID]=NetID
	c.Addrs[NetID]=addr
	c.TeamIDs[NetID]=uint8(NetID)
	// c.EventQueues[NetID] = EventQueue{head: 0,tail: 0,count: 0}
	c.EventQueues[NetID].Clear()
	c.LastTicks[NetID]=0
	c.IsDisconnected[NetID]=false
	c.NextPacketSeqs[NetID]=0
	// c.Inflights[NetID]=InFlightBuffer{}
}
func ( c *ClientsSoA)GetClient(id uint16 ) ClientRef{
	return ClientRef{
		id: id,
		data: c,
	}
}
type ClientRef struct{
	id uint16 
	data *ClientsSoA
}
func (c ClientRef) NextPacketSeq() uint16 {
    seq := c.data.NextPacketSeqs[c.id] // Lấy giá trị hiện tại
    c.data.NextPacketSeqs[c.id]++      // Tăng lên cho lần sau
    return seq                         // Trả về giá trị trước khi tăng
}
func (c ClientRef)NetID()uint16{
	return c.id
}
func (c ClientRef) TeamID() uint8 {
    return c.data.TeamIDs[c.id]
}

func (c ClientRef) SetTeamID(team uint8) {
    c.data.TeamIDs[c.id] = team
}

func (c ClientRef) Entity() Entity {
    return c.data.Entities[c.id]
}
func(c ClientRef)SetEntity(e Entity){
	c.data.Entities[c.id]=e
}
func (c ClientRef) IsDisconnected() bool {
    return c.data.IsDisconnected[c.id]
}
func (c ClientRef) SetIsDisconnected(b bool)  {
     c.data.IsDisconnected[c.id]=b
}
func( c ClientRef)LastTick()uint64{
	return  c.data.LastTicks[c.id]
}
func (c ClientRef) SetLastTick(tick uint64)  {
     c.data.LastTicks[c.id]=tick
}

func (c ClientRef) Addr() *unix.RawSockaddrAny {
    return c.data.Addrs[c.id]
}
func (c ClientRef) ClearAddr()  {
     c.data.Addrs[c.id] = nil
} 
func (c ClientRef) PendingQueue() *EventQueue {
    return &c.data.EventQueues[c.id]
}
func ( c ClientRef)InFlight()*InFlightBuffer{
	return &c.data.Inflights[c.id]
}
func ( c ClientRef)InflightEvent()*[256][32]RawEvent{
	return &c.data.InFlightsEvents[c.id]
}


func ( c ClientRef) processAckClient(highest uint16, mask uint32){
	c.data.Inflights[c.id].ProcessAck(highest,mask,&c.data.EventQueues[c.id])
	
}