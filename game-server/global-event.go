package main

const(
    GlobalEventCapacity = (1<<21)
    GlobalEventMask     = GlobalEventCapacity - 1
	// GlobalMask = VisibilityMask{}
)
var GlobalMask = VisibilityMask{
	KnownByTeams: [16]uint64{(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1,(1<<64)-1},
}
type GlobalEvent struct{
	Head uint32 
	Masks [GlobalEventCapacity] VisibilityMask
	Events [GlobalEventCapacity] RawEvent
}
func( s *GlobalEvent)Push(ev RawEvent, mask VisibilityMask)uint32{
	id := s.Head
	idx:=s.Head&GlobalEventMask
	s.Events[idx]=ev 
	s.Masks[idx] = mask
	s.Head++
	// fmt.Println("them event id ", id, " vao queue global ", s.Head, " mask ", mask, " event type ", ev.Type, " payload ", ev.Payload[:ev.Len])
	return id
}

type SnapShotData struct{
	NetID uint16 
	X,Y float32 
	HP uint16 
	Mask VisibilityMask
}
type FrameSnapshot [2000]SnapShotData