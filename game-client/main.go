package main

import (
	"fmt"
	def "game/pkg"
	"net"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)
const (
	MaxPacketSize = 8196
)
type GameState struct{
	conn *net.UDPConn
	receiver *ReliableReciever
	sendBuffer []byte 
	
	freePackets chan []byte
	readyPackets chan []byte

}
func NewGameState( conn *net.UDPConn) *GameState{
	gs := &GameState{
		conn: conn,
		sendBuffer: make([]byte, 2048),
		freePackets: make(chan []byte,32),
		readyPackets: make(chan []byte,32),
		receiver: NewReliableChannel(),
	}
	for i:= 0; i< 32;i++{
		packet := make([]byte,MaxPacketSize)
		gs.freePackets<-packet
	}
	return gs 
}
func main(){
	runtime.LockOSThread()
	serverAddr ,err:=net.ResolveUDPAddr("udp","127.0.0.1:9000");
	if err!=nil{
		panic(fmt.Errorf("lỗi phân giải IP: %w", err))
	}
	conn,err:= net.DialUDP("udp",nil,serverAddr)
	if err!=nil{
		panic(fmt.Errorf("lỗi kết nối UDP: %w", err))
	}
	
	gs := NewGameState(conn)
	conn.Write([]byte{0, 0, 0})
	go networkLisener(gs);
	ticker := time.NewTicker(16 *time.Millisecond)
	defer ticker.Stop()
	game := &ClientGame{
		netState: gs,
		Players:  make(map[uint32]*ClientPlayer),
		Projectiles: make(map[uint32]*ClientProjectile),
	}
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowTitle("Battle Royale - Go Client")
	
	// Lệnh này bắt đầu vòng lặp Game (nó sẽ block luồng chính)
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}

func networkLisener(gs *GameState){
	
	for{
		buf := <-gs.freePackets
		n,_,err:=gs.conn.ReadFromUDP(buf)

		if err!=nil{
			gs.freePackets <-buf[:cap(buf)]
			return
		}
		if n>0{
			select{
			case gs.readyPackets<-buf[:n]:
			default:
				gs.freePackets <-buf[:cap(buf)]
			}
		}else{
			gs.freePackets <-buf[:cap(buf)]
		}
	}
}
func parseServerUpdate(gs *GameState,g *ClientGame, payload []byte){
	if len(payload) < 9 || payload[0] != 0xAA {
		return 
	}
	reader := NewPacketReader(payload)
	reader.ReadUint8() // Bỏ qua Header 0xAA
	
	 reader.ReadUint8()
	zoneX := reader.ReadFloat32()
	zoneY := reader.ReadFloat32()
	zoneRad := reader.ReadFloat32()
	playerCount := reader.ReadUint8()
	newPlayers := make(map[uint32]* ClientPlayer)
		for i := 0; i < int(playerCount); i++ {
		id :=reader.ReadUint8()  // ID
		x :=reader.ReadFloat32() // X
		y := reader.ReadFloat32() // Y
		hp := reader.ReadUint16() // HP
		// //////fmt.println("ID ",id, "x ",x ,"y ",y ," hp ",hp)
		newPlayers[uint32(id)] = &ClientPlayer{
			X: x,
			Y: y,
			// TargetX: ,
			HP: hp,
		}
	}
	g.Players=newPlayers
	g.ZoneX=zoneX
	g.ZoneY = zoneY
	g.ZoneRad= zoneRad

	if reader.HasMore(){
		header := reader.ReadUint8()
		if header != 0xFF{
			return 
		}
		eventCount := reader.ReadUint8()
		for i := 0; i < int(eventCount) && reader.HasMore(); i++ {
			seqID := reader.ReadUint16()
			evType := reader.ReadUint8()
			payloadLen := reader.ReadUint16()

			// fmt.Println("type ",evType)
			payload := reader.ReadBytes(int(payloadLen))
			validEvents := gs.receiver.RecieveEvent(seqID,evType,payload)


			for _, ev := range validEvents {
				reader :=NewPacketReader(ev.Payload)
				switch ev.Type {
				case def.EventWelcome: // EventWelcome (Server trả về ID của mình)
					g.MyID =uint32(reader.ReadUint8())
					fmt.Println("Đã kết nối! ID của tôi là:", g.MyID)
					
				case def.EventSpawnProjectile: // EventSpawnBullet
				    
					id :=reader.ReadUint32()
					
					spellID := def.Spell(reader.ReadUint8())
					speed :=float32(0)
					switch spellID{
					case def.SpellToxicSpray:
						speed=800
					case def.SpellFireball:
						speed=1200
					case def.SpellIceLance:
						speed=500
					case def.SpellWindShear:
						speed = 1500
					}
					g.Projectiles[id]= &ClientProjectile{
						SpellID: spellID,
						X: reader.ReadFloat32() ,
						Y: reader.ReadFloat32(),
						Angle: reader.ReadUint16(),
						Speed: speed,
					}
					//fmt.println("co dan ",g.Projectiles[id])
				case def.EventUpdateProjectile:
					id := reader.ReadUint32()
					x:= reader.ReadFloat32()
					y := reader.ReadFloat32()
					angle := reader.ReadUint16()
					  p,ok:=g.Projectiles[id]
					  if ok{
						p.X=x
						p.Y=y
						p.Angle=angle
					  }

				case def.EventSpawnVFX:
					vfxType := def.VFXType(reader.ReadUint8())
					shape :=def.VFXShape( reader.ReadUint8())
					x := reader.ReadFloat32()
					y := reader.ReadFloat32()

					vfx := ClientVFX{
						Type:  vfxType,
						Shape: shape,
						X:     x,
						Y:     y,
					}

					// Đọc dữ liệu tùy theo Shape
					if shape == def.VFXShapeCircle {
						vfx.Radius = reader.ReadFloat32()
						duration := reader.ReadFloat32()
						vfx.TimeLeft =  duration
						vfx.MaxTime = duration
					} else if shape == def.VFXShapeBox {
						vfx.W = reader.ReadFloat32()
						vfx.H = reader.ReadFloat32()
						duration := reader.ReadFloat32()
						vfx.TimeLeft = duration
						vfx.MaxTime = duration
						vfx.Angle = reader.ReadUint16()
						
					} 
					if vfx.MaxTime < 0.5{
						vfx.MaxTime=0.5
						vfx.TimeLeft=0.5
					}
					//fmt.println("co vung no ", vfx )
					g.VFXs = append(g.VFXs, vfx)
				case def.EventRemoveEntity:
					entity :=reader.ReadUint32()
					//fmt.println("deads ",entity)
					if _,exists := g.Projectiles[entity];exists{
					   delete(g.Projectiles,entity)
					}
					// if _,exists := g.AoEs[entity];exists{
					//    delete(g.AoEs,entity)
					// }					
				
				case def.EventSendMap:
					numBushes := reader.ReadUint8()
					g.Bushes = make([]ClientBush, numBushes)
					for j := 0; j < int(numBushes); j++ {
						g.Bushes[j] = ClientBush{
							X: reader.ReadFloat32(),
							Y: reader.ReadFloat32(),
							Radius: reader.ReadFloat32(),
						}
					}
					numWalls := reader.ReadUint8()
					g.Walls = make([]ClientWall, numWalls)
					for j := 0; j < int(numWalls); j++ {
						g.Walls[j] = ClientWall{
							X: reader.ReadFloat32(), Y: reader.ReadFloat32(),
							W: reader.ReadFloat32(), H: reader.ReadFloat32(),
						}
					}
					//fmt.println("🌍 Đã tải xong Bản đồ!" , len(g.Walls), " ",len(g.Bushes))
				}

			}
		}
		// 
	}
}
func processNetworkEvents(gs *GameState, g *ClientGame) {
	for {
		select {
		case packet := <-gs.readyPackets:
			parseServerUpdate(gs,g,packet)
			gs.freePackets <- packet[:cap(packet)]
		default:
			// Hết thư, thoát vòng lặp để đi làm việc khác (Render, Input...)
			return
		}

	}
}
func sendAckEvents(gs *GameState, g *ClientGame){
	gs.sendBuffer = gs.sendBuffer[:0]
	writer := NewPacketWriter(gs.sendBuffer)
	writer.WriteUint8(g.input.keys)
	writer.WriteUint16(g.input.angle)
	writer.WriteUint16(g.input.rangeToMouse)
	writer.WriteUint8(0xFF)
	writer.WriteUint8(uint8(len(gs.receiver.ackQueue)))
	// //fmt.println("len ack ",len(gs.receiver.ackQueue))
	for _,seq :=range gs.receiver.ackQueue{	
		writer.WriteUint16(seq)
	}
	if len(writer.Bytes()) == 0{
		return
	}
	gs.conn.Write(writer.Bytes())
	gs.receiver.ackQueue = gs.receiver.ackQueue[:0]
}