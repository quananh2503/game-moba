package main

import (
	"fmt"
	def "game/pkg" // Cập nhật đường dẫn package của bạn
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	ServerAddr = "127.0.0.1:9000"
	NumBots    = 497
	MapCenter  = 2000.0
	Boundary   = 1800.0
)

// ==================================================
// HỆ THỐNG MẠNG CỦA BOT (ZERO-ALLOCATION ACK)
// ==================================================
func SeqMoreRecent(s1, s2 uint16) bool {
	return ((s1 > s2) && (s1-s2 <= 32768)) || ((s1 < s2) && (s2-s1 > 32768))
}

type NetworkChannel struct {
	mu              sync.Mutex
	HighestReceived uint16
	ReceivedPackets [65536]bool
}

func NewNetworkChannel() *NetworkChannel {
	return &NetworkChannel{}
}

func (nc *NetworkChannel) OnPacketReceived(seq uint16) bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.ReceivedPackets[seq] {
		return false // Đã nhận rồi (Lặp gói)
	}

	nc.ReceivedPackets[seq] = true
	if SeqMoreRecent(seq, nc.HighestReceived) {
		nc.HighestReceived = seq
	}
	return true
}

func (nc *NetworkChannel) GetAckData() (uint16, uint32) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	highest := nc.HighestReceived
	var bitfield uint32 = 0
	for i := uint16(0); i < 32; i++ {
		seqToCheck := highest - 1 - i
		if nc.ReceivedPackets[seqToCheck] {
			bitfield |= (1 << i)
		}
	}
	return highest, bitfield
}

// ==================================================
// BOT LOGIC
// ==================================================
type BotState struct {
	MyID           uint32
	X, Y           float32
	mu             sync.RWMutex
	CurrentKeys    uint8
	ActionTimeLeft float32
}

func main() {
	fmt.Printf("🚀 Khởi động %d Headless Bots (Bitfield ACK)...\n", NumBots)
	serverIP, _ := net.ResolveUDPAddr("udp", ServerAddr)
	var wg sync.WaitGroup

	for i := 0; i < NumBots; i++ {
		wg.Add(1)
		go runBot(i, serverIP, &wg)
		time.Sleep(5 * time.Millisecond) // Tránh ddos cổng HTTP cục bộ
	}
	wg.Wait()
}

func runBot(botID int, serverIP *net.UDPAddr, wg *sync.WaitGroup) {
	defer wg.Done()

	// 1. GIAI ĐOẠN HTTP (Mô phỏng lấy Map)
	resp, err := http.Get("http://127.0.0.1:8080/join")
	if err == nil {
		io.ReadAll(resp.Body) // Đọc xong bỏ đi, tạo áp lực cho Server
		resp.Body.Close()
	}

	// 2. GIAI ĐOẠN UDP
	conn, err := net.DialUDP("udp", nil, serverIP)
	if err != nil {
		return
	}
	defer conn.Close()

	seed := time.Now().UnixNano() + int64(botID)
	localRand := rand.New(rand.NewSource(seed))

	state := &BotState{}
	receiver := NewNetworkChannel()
	readBuf := make([]byte, 8196)
	writeBuf := make([]byte, 2048)

	// Gửi gói Handshake đầu tiên
	conn.Write([]byte{0, 0, 0})

	// Luồng Đọc UDP
	go func() {
		for {
			n, _, err := conn.ReadFromUDP(readBuf)
			if err != nil { return }
			if n > 0 {
				parseServerPacket(readBuf[:n], receiver, state)
			}
		}
	}()

	// Luồng Gửi UDP (Tick Rate)
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()
	dt := float32(16.0 / 1000.0)

	validMoves := []uint8{
		0, uint8(def.InputW), uint8(def.InputS), uint8(def.InputA), uint8(def.InputD),
		uint8(def.InputW | def.InputA), uint8(def.InputW | def.InputD),
		uint8(def.InputS | def.InputA), uint8(def.InputS | def.InputD),
	}

	for range ticker.C {
		state.mu.Lock()
		curX, curY := state.X, state.Y
		state.ActionTimeLeft -= dt

		distX := curX - MapCenter
		distY := curY - MapCenter
		isOutOfBounds := distX > Boundary || distX < -Boundary || distY > Boundary || distY < -Boundary

		if curX != 0 && curY != 0 && isOutOfBounds {
			var newKeys uint8
			if distX > Boundary { newKeys |= uint8(def.InputA) }
			if distX < -Boundary { newKeys |= uint8(def.InputD) }
			if distY > Boundary { newKeys |= uint8(def.InputW) }
			if distY < -Boundary { newKeys |= uint8(def.InputS) }

			state.CurrentKeys = newKeys
			state.ActionTimeLeft = 1.5
		} else if state.ActionTimeLeft <= 0 {
			state.CurrentKeys = validMoves[localRand.Intn(len(validMoves))]
			state.ActionTimeLeft = 0.5 + localRand.Float32()*2.0
		}

		fakeKeys := state.CurrentKeys
		state.mu.Unlock()

		if localRand.Intn(100) < 3 { fakeKeys |= uint8(def.InputLeftClick) }
		if localRand.Intn(1000) < 5 { fakeKeys |= uint8(def.InputSpace) }
		if localRand.Intn(1000) < 5 { fakeKeys |= uint8(def.InputRightClick) }

		fakeAngle := uint16(localRand.Intn(360))
		fakeDist := uint16(localRand.Intn(300))

		// --- ĐÓNG GÓI GỬI LÊN SERVER ---
		writer := NewPacketWriter(writeBuf[:0])
		writer.WriteUint8(fakeKeys)
		writer.WriteUint16(fakeAngle)
		writer.WriteUint16(fakeDist)

		writer.WriteUint8(0xFF) // Header báo hiệu ACK
		highestSeq, bitfield := receiver.GetAckData()
		writer.WriteUint16(highestSeq)
		writer.WriteUint32(bitfield)

		conn.Write(writer.Bytes())
	}
}

// ==================================================
// PARSER (Đọc dữ liệu từ Server)
// ==================================================
func parseServerPacket(payload []byte, receiver *NetworkChannel, state *BotState) {
	if len(payload) < 6 || payload[0] != 0xAA {
		return
	}
	reader := NewPacketReader(payload)
	reader.ReadUint8() // Bỏ qua 0xAA

	packetSeq := reader.ReadUint16()

	// Đánh dấu nhận, nếu trùng lặp (false) thì vứt gói luôn
	if !receiver.OnPacketReceived(packetSeq) {
		return
	}

	receiver.mu.Lock()
	highest := receiver.HighestReceived
	receiver.mu.Unlock()
	isOutdated := !SeqMoreRecent(packetSeq, highest) && packetSeq != highest

	hasSnapshot := reader.ReadUint8()
	if hasSnapshot == 1 {
		reader.ReadUint8() // MatchState
		reader.ReadFloat32() // ZoneX
		reader.ReadFloat32() // ZoneY
		reader.ReadFloat32() // ZoneRad

		playerCount := reader.ReadUint16()
		state.mu.Lock()
		for i := 0; i < int(playerCount); i++ {
			id := reader.ReadUint16()
			x := reader.ReadFloat32()
			y := reader.ReadFloat32()
			reader.ReadUint16() // HP

			if state.MyID == 0 {
				state.MyID = uint32(id) // Lấy tạm nếu chưa có
			}
			// CHỐNG GIẬT LÙI: Chỉ cập nhật tọa độ nếu không phải gói quá khứ
			if !isOutdated && uint32(id) == state.MyID {
				state.X = x
				state.Y = y
			}
		}
		state.mu.Unlock()
	}

	if reader.HasMore() && reader.ReadUint8() == 0xFF {
		eventCount := reader.ReadUint8()
		for i := 0; i < int(eventCount) && reader.HasMore(); i++ {
			evType := reader.ReadUint8()
			payloadLen := reader.ReadUint16()
			eventPayload := reader.ReadBytes(int(payloadLen))

			if evType == uint8(def.EventWelcome) {
				r := NewPacketReader(eventPayload)
				state.mu.Lock()
				state.MyID = uint32(r.ReadUint8())
				state.mu.Unlock()
			}
			// Các Event khác bot mù không cần quan tâm, chỉ cần báo ACK là đủ!
		}
	}
}

// ==================================================
// PACKET WRITER & READER (Kèm theo để chạy được Script)
// ==================================================
type PacketWriter struct { Buf []byte }
func NewPacketWriter(payload []byte) *PacketWriter { return &PacketWriter{Buf: payload} }
func (w *PacketWriter) WriteUint8(v uint8) { w.Buf = append(w.Buf, v) }
func (w *PacketWriter) WriteUint16(v uint16) { w.Buf = append(w.Buf, byte(v>>8), byte(v)) }
func (w *PacketWriter) WriteUint32(v uint32) { w.Buf = append(w.Buf, byte(v>>24), byte(v>>16), byte(v>>8), byte(v)) }
func (w *PacketWriter) Bytes() []byte { return w.Buf }

type PacketReader struct { Buf []byte; Offset int }
func NewPacketReader(data []byte) *PacketReader { return &PacketReader{Buf: data, Offset: 0} }
func (r *PacketReader) ReadUint8() uint8 { v := r.Buf[r.Offset]; r.Offset++; return v }
func (r *PacketReader) ReadUint16() uint16 { v := uint16(r.Buf[r.Offset])<<8 | uint16(r.Buf[r.Offset+1]); r.Offset += 2; return v }
func (r *PacketReader) ReadFloat32() float32 {
	i := uint32(r.Buf[r.Offset])<<24 | uint32(r.Buf[r.Offset+1])<<16 | uint32(r.Buf[r.Offset+2])<<8 | uint32(r.Buf[r.Offset+3])
	r.Offset += 4
	return math.Float32frombits(i)
}
func (r *PacketReader) ReadBytes(len int) []byte {
	payload := make([]byte, len)
	copy(payload, r.Buf[r.Offset:r.Offset+len])
	r.Offset += len
	return payload
}
func (r *PacketReader) HasMore() bool { return r.Offset < len(r.Buf) }