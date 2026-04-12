package main

import (
	"fmt"
	def "game/pkg" // Đảm bảo bạn đã import package chứa các hằng số Input
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	ServerAddr = "127.0.0.1:9000"
	NumBots    = 100 
	MapCenter  = 2000.0
	Boundary   = 2000.0 // Bot sẽ quay đầu nếu cách tâm quá 600 đơn vị
)

// Cấu trúc để lưu trạng thái của mỗi Bot
type BotState struct {
	MyID uint32
	X, Y float32
	mu   sync.RWMutex
}

func main() {
	fmt.Printf("🚀 Khởi động %d Headless Bots (Có logic quay về tâm)...\n", NumBots)
	serverIP, _ := net.ResolveUDPAddr("udp", ServerAddr)
	var wg sync.WaitGroup

	for i := 0; i < NumBots; i++ {
		wg.Add(1)
		go runBot(i, serverIP, &wg)
		time.Sleep(10 * time.Millisecond) 
	}
	wg.Wait()
}

func runBot(botID int, serverIP *net.UDPAddr, wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.DialUDP("udp", nil, serverIP)
	if err != nil { return }
	defer conn.Close()

	seed := time.Now().UnixNano() + int64(botID)
	localRand := rand.New(rand.NewSource(seed))
	
	state := &BotState{}
	receiver := NewReliableChannel()
	readBuf := make([]byte, 8196)
	writeBuf := make([]byte, 2048)

	// Gửi gói tin đầu tiên để Server nhận diện
	conn.Write([]byte{0, 0, 0, 0, 0})

	// LUỒNG NHẬN DATA
	go func() {
		for {
			n, _, err := conn.ReadFromUDP(readBuf)
			if err != nil { return }
			if n > 0 {
				extractACKs(readBuf[:n], receiver, state)
			}
		}
	}()

	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		state.mu.RLock()
		curX, curY := state.X, state.Y
		state.mu.RUnlock()

		var fakeKeys uint8

		// LOGIC DI CHUYỂN:
		// Nếu chưa có tọa độ (mới vào game), di chuyển ngẫu nhiên hoàn toàn
		if curX == 0 && curY == 0 {
			fakeKeys = uint8(localRand.Intn(16)) // Random WASD
		} else {
			// Kiểm tra vị trí so với tâm
			distToCenterX := curX - MapCenter
			distToCenterY := curY - MapCenter

			// Nếu quá xa tâm về bên phải -> Ấn 'A' (Sang trái)
			if distToCenterX > Boundary {
				fakeKeys |= uint8(def.InputA)
			} else if distToCenterX < -Boundary { // Quá xa về bên trái -> Ấn 'D'
				fakeKeys |= uint8(def.InputD)
			}

			// Nếu quá xa tâm về phía dưới -> Ấn 'W' (Lên trên)
			if distToCenterY > Boundary {
				fakeKeys |= uint8(def.InputW)
			} else if distToCenterY < -Boundary { // Quá xa về phía trên -> Ấn 'S'
				fakeKeys |= uint8(def.InputS)
			}

			// Nếu vẫn ở trong vùng an toàn, thi thoảng nhấn phím ngẫu nhiên cho tự nhiên
			if fakeKeys == 0 {
				fakeKeys = uint8(localRand.Intn(16))
			}
		}

		// Thêm một chút tỉ lệ bắn chiêu ngẫu nhiên (LMB/RMB)
		if localRand.Intn(100) < 5 { // 5% mỗi tick sẽ click chuột
			fakeKeys |= uint8(def.InputLeftClick)
		}

		fakeAngle := uint16(localRand.Intn(360))
		fakeDist := uint16(localRand.Intn(200))

		writer := NewPacketWriter(writeBuf[:0])
		writer.WriteUint8(fakeKeys)
		writer.WriteUint16(fakeAngle)
		writer.WriteUint16(fakeDist)

		writer.WriteUint8(0xFF)
		writer.WriteUint8(uint8(len(receiver.ackQueue)))
		for _, seq := range receiver.ackQueue {
			writer.WriteUint16(seq)
		}

		conn.Write(writer.Bytes())
		receiver.ackQueue = receiver.ackQueue[:0]
	}
}

func extractACKs(payload []byte, receiver *ReliableReciever, state *BotState) {
	if len(payload) < 9 || payload[0] != 0xAA {
		return
	}
	
	reader := NewPacketReader(payload)
	reader.ReadUint8() // Header
	reader.ReadUint8() // MatchState
	reader.ReadFloat32() // ZoneX
	reader.ReadFloat32() // ZoneY
	reader.ReadFloat32() // ZoneRad
	
	playerCount := reader.ReadUint8()
	
	state.mu.Lock()
	for i := 0; i < int(playerCount); i++ {
		id := reader.ReadUint8()  
		x := reader.ReadFloat32() 
		y := reader.ReadFloat32() 
		hp := reader.ReadUint16() 

		// Nếu đây là chính mình (dựa trên ID mà Server cấp lúc Welcome)
		// Hoặc đơn giản là lấy ID đầu tiên nếu chưa biết MyID
		if state.MyID == 0 {
			state.MyID = uint32(id)
		}
		
		if uint32(id) == state.MyID {
			state.X = x
			state.Y = y
		}
		_ = hp
	}
	state.mu.Unlock()

	// Phần xử lý Event (Welcome, Spawn...) để lấy MyID chuẩn từ Server
	if !reader.HasMore() { return }
	header := reader.ReadUint8()
	if header != 0xFF { return }

	eventCount := reader.ReadUint8()
	for i := 0; i < int(eventCount) && reader.HasMore(); i++ {
		func(){

		seqID := reader.ReadUint16()
		evType := reader.ReadUint8()
		defer func ()  {
				if r := recover(); r != nil {
					// IN RA TOÀN BỘ THÔNG TIN ĐỂ DEBUG
					fmt.Printf("🔥 [BOT ERROR] Panic khi đọc Event thứ %d/%d\n", i+1, eventCount)
					fmt.Printf("Chi tiết lỗi: %v\n", r)
					fmt.Printf("Trạng thái Reader: Offset=%d, Kích thước gói=%d\n", reader.Offset, len(reader.Buf))
					fmt.Printf("Seq %d, evType %d  ",seqID,evType)
					// Tùy theo code reader của bạn, in ra thêm nếu được
					panic("Stop bot để xem lỗi") 
				}	
			}()
		payloadLen := reader.ReadUint16()

		eventPayload := reader.ReadBytes(int(payloadLen))

		// Cập nhật MyID khi nhận được Event Welcome
		if evType == uint8(def.EventWelcome) {
			welcomeReader := NewPacketReader(eventPayload)
			state.mu.Lock()
			state.MyID = uint32(welcomeReader.ReadUint8())
			state.mu.Unlock()
		}

		receiver.RecieveEvent(seqID, evType, eventPayload)
		}()
	}
}