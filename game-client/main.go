package main

import (
	"fmt"
	def "game/pkg"
	"io"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)
const (
	MaxPacketSize = 8196
)
type GameState struct{
	conn *net.UDPConn
	receiver *NetworkChannel
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
		receiver: NewNetworkChannel(),
	}
	for i:= 0; i< 32;i++{
		packet := make([]byte,MaxPacketSize)
		gs.freePackets<-packet
	}
	return gs 
}
func loadMap(data []byte, g *ClientGame){
	r:=NewPacketReader(data)
	numBushes := r.ReadUint8()
		g.Bushes = make([]ClientBush, numBushes)
		for j := 0; j < int(numBushes); j++ {
			g.Bushes[j] = ClientBush{
				X: r.ReadFloat32(),
				Y: r.ReadFloat32(),
				Radius: r.ReadFloat32(),
			}
		}
		numWalls := r.ReadUint8()
		g.Walls = make([]ClientWall, numWalls)
		for j := 0; j < int(numWalls); j++ {
			g.Walls[j] = ClientWall{
				X: r.ReadFloat32(), Y: r.ReadFloat32(),
				W: r.ReadFloat32(), H: r.ReadFloat32(),
			}
		}
		fmt.Println(" numBush ",numBushes , " numWall ",numWalls)	
}
func main(){
	runtime.LockOSThread()

	resp, err := http.Get("http://127.0.0.1:8080/join")
	if err != nil { return }
	
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	
	// Giải mã body lấy SessionID và MapData
	// sessionID, mapData := parseJoinResponse(body)
	// loadMap(body) // Bot đã có map!



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
		VFXs: make(map[uint32]*ClientVFX),
	}
	loadMap(body,game)
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

func parseServerUpdate(gs *GameState, g *ClientGame, payload []byte) {
	if len(payload) < 6 || payload[0] != 0xAA { return }
	reader := NewPacketReader(payload)
	reader.ReadUint8()
	packetSeq := reader.ReadUint16()
	if gs.receiver.RecievedPackets[packetSeq] {
		return 
	}
	isOutdatedState := !SeqMoreRecent(packetSeq, gs.receiver.HighestRecieved) && packetSeq != gs.receiver.HighestRecieved
	hasSnapshot:=reader.ReadUint8() 
	if hasSnapshot == 1 {
		reader.ReadUint8() // MatchState
		zX := reader.ReadFloat32()
		zY := reader.ReadFloat32()
		zRad := reader.ReadFloat32()
		
		playerCount := reader.ReadUint16()
		
		// CHỈ LƯU TỌA ĐỘ VÀO GAME NẾU GÓI NÀY LÀ MỚI NHẤT
		if !isOutdatedState {
			g.ZoneX = zX
			g.ZoneY = zY
			g.ZoneRad = zRad
			
			newPlayers := make(map[uint32]*ClientPlayer)
			for i := 0; i < int(playerCount); i++ {
				id := reader.ReadUint16()
				newPlayers[uint32(id)] = &ClientPlayer{
					X: reader.ReadFloat32(),
					Y: reader.ReadFloat32(),
					HP: reader.ReadUint16(),
				}
			}
			g.Players = newPlayers
		} else {
			// NẾU GÓI CŨ: Vẫn phải đọc lướt qua để Offset chạy tới phần Event bên dưới
			for i := 0; i < int(playerCount); i++ {
				reader.ReadUint16()
				reader.ReadFloat32()
				reader.ReadFloat32()
				reader.ReadUint16()
			}
		}
	}

	if reader.HasMore() {
		header := reader.ReadUint8()
		if header != 0xFF { return }

		eventCount := reader.ReadUint8()
		for i := 0; i < int(eventCount) && reader.HasMore(); i++ {
			evType := reader.ReadUint8()
			payloadLen := reader.ReadUint16()

			eventPayload := reader.ReadBytes(int(payloadLen))
			
			// validEvents := 

			
				r := NewPacketReader(eventPayload)
				switch evType {
				
				case def.EventWelcome:
					g.MyID = uint32(r.ReadUint16())
					fmt.Println("id :",g.MyID)
					
				case def.EventSpawnProjectile:
					id := r.ReadUint32()
					spellID := def.Spell(r.ReadUint8())
					spellData := def.GetSpellData(spellID) // Tra cứu từ điển

					g.Projectiles[id] = &ClientProjectile{
						SpellID:  spellID,
						X:        r.ReadFloat32(),
						Y:        r.ReadFloat32(),
						Angle:    r.ReadUint16(),
						TimeLeft: spellData.MaxTime, // Khóa tầm nội suy
					}

				case def.EventUpdateProjectile:
					id := r.ReadUint32()
					if p, ok := g.Projectiles[id]; ok {
						p.X = r.ReadFloat32()
						p.Y = r.ReadFloat32()
						p.Angle = r.ReadUint16()
					}

				case def.EventSpawnVFX:
					id := r.ReadUint32() // BẮT BUỘC PHẢI CÓ ID ĐỂ SAU NÀY XÓA
					vfxType := def.VFXType(r.ReadUint8())
					// fmt.Println("co VFX ",id)
					vfxData := def.GetVFXData(vfxType) // Tra cứu từ điển

					g.VFXs[id] = &ClientVFX{
						Type:     vfxType,
						X:        r.ReadFloat32(),
						Y:        r.ReadFloat32(),
						Angle:    r.ReadUint16(),
						TimeLeft: vfxData.MaxTime,
					}

				case def.EventRemoveEntity:
					entityID := r.ReadUint32()
					// fmt.Println("remove VFX ",entityID)
					// Vô tư xóa, thằng nào chứa ID này thì thằng đó bay màu (Đạn hoặc VFX/Vùng độc)
					delete(g.Projectiles, entityID)
					delete(g.VFXs, entityID) // FIX LỖI VÙNG ĐỘC TRÔI LẠI TRÊN MÀN HÌNH

			}
		}
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
	highestSeq, mask:=gs.receiver.GetAckData()
	writer.WriteUint16(highestSeq)
	writer.WriteUint32(mask)
	// writer.WriteUint8(uint8(len(gs.receiver.ackQueue)))
	// // //fmt.println("len ack ",len(gs.receiver.ackQueue))
	// for _,seq :=range gs.receiver.ackQueue{	
	// 	writer.WriteUint16(seq)
	// }
	// if len(writer.Bytes()) == 0{
	// 	return
	// }
	gs.conn.Write(writer.Bytes())
	// gs.receiver.ackQueue = gs.receiver.ackQueue[:0]
}