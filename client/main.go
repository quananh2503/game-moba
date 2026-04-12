package main

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	// ServerAddr = "127.0.0.1:9000"
	SendRate   = 16 * time.Millisecond // Tốc độ gửi (60 lần/giây)
)

func main() {
	if len(os.Args) < 2 {
		//////fmt.println("Cách dùng: go run . <số lượng client>")
		os.Exit(1)
	}
	numClients, _ := strconv.Atoi(os.Args[1])

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go runPlayer(i, &wg)
	}
	wg.Wait()
}

func runPlayer(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	// 1. Mở kết nối UDP
	conn, err := net.Dial("udp", ServerAddr)
	if err != nil {
		fmt.Printf("[Player %d] Lỗi kết nối: %v\n", id)
		return
	}
	defer conn.Close()

	// Tạo 2 Goroutine cho mỗi người chơi: 1 để Gửi, 1 để Nhận
	go sendLoop(id, conn)
	go receiveLoop(id, conn)

	// Giữ Goroutine của Player này sống
	select {}
}

// sendLoop: Chạy ngầm, liên tục gửi tọa độ lên server
func sendLoop(id int, conn net.Conn) {
	ticker := time.NewTicker(SendRate)
	var seq uint16 = 0

	for {
		<-ticker.C
		seq++
		x := uint16(100 + id*5) // Vị trí X cố định để dễ quan sát
		y := uint16(rand.Intn(500)) // Vị trí Y ngẫu nhiên

		// Đóng gói data: [2 byte Seq][2 byte X][2 byte Y]
		packet := make([]byte, 6)
		binary.BigEndian.PutUint16(packet[0:2], seq)
		binary.BigEndian.PutUint16(packet[2:4], x)
		binary.BigEndian.PutUint16(packet[4:6], y)

		conn.Write(packet)
	}
}

// receiveLoop: Chạy ngầm, lắng nghe gói tin từ Server
func receiveLoop(id int, conn net.Conn) {
	buf := make([]byte, 1024)

	for {

		n, err := conn.Read(buf)
	//////fmt.println("nhan dc  ", n)
		if err != nil {
			fmt.Printf("[Player %d] Lỗi nhận: %v\n", id, err)
			return
		}

		packet := buf[:n]
		if len(packet) < 1 {
			continue
		}

		// Giải mã gói tin từ Server
		packetType := packet[0]
		if packetType == 0xAA { // Đây là gói Snapshot!
			// //////fmt.println("Toi day roi ", len(packet))
			if len(packet) < 3 {
				continue
			}
			playerCount := binary.BigEndian.Uint16(packet[1:3])

			// Chỉ in ra log của Player 0 để tránh spam màn hình
			if id == 0 {
				fmt.Printf("[Player 0] NHẬN SNAPSHOT: Có %d người chơi đang di chuyển.\n", playerCount)
			}
		}
		// (Bạn có thể thêm code giải mã gói ACK ở đây sau này)
	}

}