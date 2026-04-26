package main

import "time"

const (
	SafeMTU         = 1200
	MaxInFlight     = 256
	MaxEventsPerPkt = 32
	MaxPlayers =500
	EventQueueSize=1024
	EventQueueMask = EventQueueSize - 1 
)
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


type MatchState struct {
	MatchState uint8 
	TickCount uint64 
	Zone ZoneInfo
	WinnerID uint16 
	alivePlayer int
	TimeNow time.Time
}
type SpatialEvent struct {
	X, Y  float32
	Event RawEvent
}
type PrivateEvent struct{
	NetId uint16
	Event RawEvent
}
type PlayerSnapshot struct {
	NetID uint16
	X, Y  float32
	HP    uint16
}
type NetworkOutbox struct {
	Globals  []RawEvent
	Privates [MaxPlayers][]RawEvent
	Spatials []SpatialEvent
	Teams 	[MaxPlayers][]RawEvent
	Positions [MaxPlayers][]PlayerSnapshot
}
func (s *NetworkOutbox)Reset(){
	s.Globals=s.Globals[:0]
	s.Spatials=s.Spatials[:0]
	for i:= range s.Privates{
		s.Privates[i]=s.Privates[i][:0]
	}
	for i:= range s.Teams{
		s.Teams[i]=s.Teams[i][:0]
	}
	for i:= range s.Positions{
		s.Positions[i]=s.Positions[i][:0]
	}
}
func NewNetworkOutbox() *NetworkOutbox {
	nw:= &NetworkOutbox{
		Globals:  make([]RawEvent, 0, 100),
		// Privates: make([]PrivateEvent,0,500),
		Spatials: make([]SpatialEvent, 0, 500),
	}
	for i:=range nw.Privates{
		nw.Privates[i]=make([]RawEvent, 0,500)
	}
	for i:=range nw.Teams{
		nw.Teams[i]=make([]RawEvent, 0,500)
	}
	for i:=range nw.Positions{
		nw.Positions[i]=make([]PlayerSnapshot, 0,500)
	}
	return nw
}