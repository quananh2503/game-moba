package main

import (
	"time"
)

const (
	ServerAddr   = "127.0.0.1:9000"
	TestTimeout  = 5 * time.Second
	NumClients   = 5
	NumTestLoops = 3
)

// func main() {
// 	fmt.Printf("🚀 Bắt đầu bài test tuần tự với %d clients, lặp lại %d lần...\n", NumClients, NumTestLoops)

// 	// 1. Khởi tạo 5 kết nối client
// 	clients := make([]net.Conn, NumClients)
// 	for i := 0; i < NumClients; i++ {
// 		conn, err := net.Dial("udp", ServerAddr)
// 		if err != nil {
// 			fmt.Printf("❌ FAIL: Không thể tạo kết nối cho Client %d: %v\n", i, err)
// 			os.Exit(1)
// 		}
// 		defer conn.Close()
// 		clients[i] = conn
// 	}
// 	//////fmt.println("✅ Đã tạo thành công 5 kết nối client.")

// 	// Channel để Client 0 báo cáo kết quả kiểm tra
// 	resultChan := make(chan bool)

// 	// Chạy Goroutine lắng nghe cho Client 0
// 	go listenForSnapshots(clients[0], resultChan)

// 	// 2. Chạy 3 vòng lặp test
// 	for loop := 1; loop <= NumTestLoops; loop++ {
// 		fmt.Printf("\n--- Vòng lặp Test #%d ---\n", loop)

// 		// Tạo dữ liệu test cho vòng lặp này
// 		expectedState := make(map[uint16]struct{ X, Y uint16 })
// 		for i := 0; i < NumClients; i++ {
// 			x := uint16(100*loop + i)
// 			y := uint16(200*loop + i)
// 			expectedState[uint16(i)] = struct{ X, Y uint16 }{X: x, Y: y}
// 		}

// 		// 3. Gửi (Send Phase): 5 client lần lượt gửi tọa độ
// 		for i := 0; i < NumClients; i++ {
// 			playerID := uint16(i)
// 			pos := expectedState[playerID]
// 			fmt.Printf("   -> Client %d gửi tọa độ: X=%d, Y=%d\n", i, pos.X, pos.Y)
// 			sendPacket(clients[i], uint16(loop), pos.X, pos.Y)
// 			time.Sleep(10 * time.Millisecond) // Giả lập độ trễ nhỏ giữa các gói tin
// 		}
		
// 		// 4. Chờ và Kiểm tra (Assert Phase)
// 		//////fmt.println("   ... Chờ Server broadcast trạng thái mới...")
// 		select {
// 		case success := <-resultChan:
// 			if success {
// 				fmt.Printf("   ✅ Vòng lặp #%d: Client 0 xác nhận trạng thái thế giới chính xác!\n", loop)
// 			} else {
// 				fmt.Printf("❌ FAIL: Vòng lặp #%d: Trạng thái thế giới không chính xác!\n", loop)
// 				os.Exit(1)
// 			}
// 		case <-time.After(TestTimeout):
// 			fmt.Printf("❌ FAIL: Vòng lặp #%d: Hết thời gian chờ phản hồi từ Server.\n", loop)
// 			os.Exit(1)
// 		}
// 	}
	
// 	//////fmt.println("\n🎉🎉🎉 PASS: Tất cả các vòng lặp test đều thành công!")
// }

// // listenForSnapshots: Client 0 sẽ chạy hàm này để kiểm tra gói tin từ Server
// func listenForSnapshots(conn net.Conn, resultChan chan<- bool) {
// 	buf := make([]byte, 1024)
// 	var lastLoopChecked int = 0

// 	for {
// 		n, err := conn.Read(buf)
// 		if err != nil { return }
		
// 		packet := buf[:n]
// 		if len(packet) > 0 && packet[0] == 0xAA { // Gói Snapshot
			
// 			// Kiểm tra xem đây có phải là snapshot của vòng lặp tiếp theo không
// 			loopToTest := lastLoopChecked + 1
// 			if loopToTest > NumTestLoops { continue }
			
// 			// Xây dựng lại trạng thái thế giới mong đợi cho vòng lặp này
// 			expectedState := make(map[uint16]struct{ X, Y uint16 })
// 			for i := 0; i < NumClients; i++ {
// 				x := uint16(100*loopToTest + i)
// 				y := uint16(200*loopToTest + i)
// 				expectedState[uint16(i)] = struct{ X, Y uint16 }{X: x, Y: y}
// 			}

// 			// Giải mã gói tin Snapshot
// 			currentState := make(map[uint16]struct{ X, Y uint16 })
// 			playerCount := binary.BigEndian.Uint16(packet[1:3])
// 			offset := 3
// 			for i := 0; i < int(playerCount); i++ {
// 				pID := binary.BigEndian.Uint16(packet[offset:])
// 				x := binary.BigEndian.Uint16(packet[offset+2:])
// 				y := binary.BigEndian.Uint16(packet[offset+4:])
// 				currentState[pID] = struct{ X, Y uint16 }{X: x, Y: y}
// 				offset += 6
// 			}
			
// 			// So sánh trạng thái nhận được với trạng thái mong đợi
// 			match := true
// 			if len(currentState) < NumClients {
// 				match = false
// 			} else {
// 				for id, pos := range expectedState {
// 					if receivedPos, ok := currentState[id]; !ok || receivedPos != pos {
// 						match = false
// 						break
// 					}
// 				}
// 			}

// 			// Nếu khớp, báo cáo thành công cho vòng lặp hiện tại
// 			if match {
// 				lastLoopChecked = loopToTest
// 				resultChan <- true
// 			}
// 		}
// 	}
// }

// func sendPacket(conn net.Conn, seq, x, y uint16) {
// 	packet := make([]byte, 6)
// 	binary.BigEndian.PutUint16(packet[0:2], seq)
// 	binary.BigEndian.PutUint16(packet[2:4], x)
// 	binary.BigEndian.PutUint16(packet[4:6], y)
// 	conn.Write(packet)
// }