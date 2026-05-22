// session_flush_bench_test.go
package main

import (
	"testing"

	"golang.org/x/sys/unix"
)

type ClientsSoA2 struct{
	States [MaxPlayers]NetworkState
	Endpoints [MaxPlayers]NetworkEndpoint
	Events_Header [MaxPlayers]	EventHeader
	Events_EventIDs [MaxPlayers][MaxEvents]uint32
	Events_PacketSequences [MaxPlayers][MaxEvents]uint16																		
	
}
// 1. DỮ LIỆU MẪU TẤT ĐỊNH
func setupBenchmarkData(numPlayers int) (*ClientsSoA2, *GlobalEvent) {
	soa := &ClientsSoA2{}
	globalEv := &GlobalEvent{
		Head: 5000, // Giả sử game đã chạy được một lúc, sinh ra 5000 events
	}

	// Fake Global Events
	for i := 0; i < len(globalEv.Masks); i++ {
		// Giả sử cứ event chẵn thì team chẵn nhìn thấy
		globalEv.Masks[i].KnownByTeams[0] = 0xFFFFFFFFFFFFFFFF // Mọi người đều thấy cho dễ test
	}

	// Fake Clients
	for i := 0; i < numPlayers; i++ {
		soa.Endpoints[i].Addr = &unix.RawSockaddrAny{} // Đánh dấu là đang kết nối
		soa.Endpoints[i].TeamID = uint16(i)

		soa.States[i].IsDisconnected = false
		// soa.States[i].Cursor = 4000 // Mỗi client đang bị tụt lại 50 events so với Head
		
		soa.Events_Header[i] = EventHeader{
			Head: 0,
			Tail: 0,
			Cursor: 4000, // Mỗi client đang bị tụt lại 1000 events so với Head
			TeamID: uint16(i),
			IsActive: true,
		}
	
	}

	return soa, globalEv
}
var GlobalSink uint32
// Giữ nguyên logic của bạn, nhưng tách nó ra thành hàm thuần túy (Pure Function)
func flushToQueue_CurrentLayout(nextClientID uint16, clientSOA *ClientsSoA2, globalEvent *GlobalEvent) {
	currentGlobalHead := globalEvent.Head
	for i := uint16(0); i < nextClientID; i++ {
		if clientSOA.Events_Header[i].IsActive == false {
			continue
		}
		eventIDs := &clientSOA.Events_EventIDs[i]
		teamID := clientSOA.Events_Header[i].TeamID
		// maskTeamID 
		maskTeamID := teamID >> 6
		//maskAndTeamID :\
		maskAndTeamID := uint64(1) << (teamID & 63)

		// head :
		head := clientSOA.Events_Header[i].Head
		// cursor :
		// cursor :
		cursor := clientSOA.Events_Header[i].Cursor
		// var localSum uint32
		for j := cursor; j < currentGlobalHead; j++ {
			idx := j & (GlobalEventMask)
			if globalEvent.Masks[idx].KnownByTeams[maskTeamID]&maskAndTeamID == 0 {
				continue
			}
			eventIDs[head&(MaxEvents-1)] =j
			head++
		}
		// GlobalSink += localSum // Đảm bảo compiler không tối ưu biến localSum đi
		clientSOA.Events_Header[i].Head = head
		clientSOA.Events_Header[i].Cursor = currentGlobalHead
	}
}
func flushToQueue_Blocking(nextClientID uint16, clientSOA *ClientsSoA2, globalEvent *GlobalEvent) {
    const BLOCK_SIZE = 64 // Thử với 64 events một lần (64 * 4 bytes = 256 bytes, thừa sức nằm trong L1)

    // VÒNG NGOÀI: Lặp qua từng người chơi
    for i := uint16(0); i < nextClientID; i++ {
        header := &clientSOA.Events_Header[i]
        if !header.IsActive {
            continue
        }

        // Lấy con trỏ ra ngoài để tránh lặp lại
        eventIDs := &clientSOA.Events_EventIDs[i]
        head := header.Head
        cursor := header.Cursor
        currentGlobalHead := globalEvent.Head

        // VÒNG GIỮA: Chia 1000 events thành nhiều "khối" nhỏ (Blocks)
        for blockStart := cursor; blockStart < currentGlobalHead; blockStart += BLOCK_SIZE {
            
            // TÍNH TOÁN LOGIC CHO CẢ BLOCK TRƯỚC
            // (Ví dụ: kiểm tra Mask, lọc ra các events hợp lệ trong block này)
            // ... (Phần này sẽ phức tạp hơn một chút, nhưng là cốt lõi của tối ưu)

            // VÒNG TRONG: Ghi dữ liệu của block này vào mảng
            // Vòng này giờ chỉ chạy BLOCK_SIZE (64) lần, cực nhanh và nằm gọn trong L1
            blockEnd := blockStart + BLOCK_SIZE
            if blockEnd > currentGlobalHead {
                blockEnd = currentGlobalHead
            }
            
            for j := blockStart; j < blockEnd; j++ {
                // if (event j hợp lệ) { // logic kiểm tra ở trên
                    eventIDs[head&(MaxEvents-1)] = j // Không cần & (MaxEvents-1) vì head cứ tăng
                    head++
                // }
            }
        }
        
        header.Head = head
        header.Cursor = currentGlobalHead
    }
}
func flushToQueue_LoopInversion(nextClientID uint16, clientSOA *ClientsSoA2, globalEvent *GlobalEvent) {
	// BƯỚC 1: Xác định phạm vi Events cần xử lý trong Tick này.
	// Chúng ta không thể biết trước cursor của từng người, nên phải lấy cursor nhỏ nhất.
	minCursor := globalEvent.Head 
	for i := uint16(0); i < nextClientID; i++ {
		if clientSOA.Events_Header[i].IsActive && clientSOA.Events_Header[i].Cursor < minCursor {
			minCursor = clientSOA.Events_Header[i].Cursor
		}
	}
	
	currentGlobalHead := globalEvent.Head
	
	for j := minCursor; j < currentGlobalHead; j++ {
		// idx := j & (GlobalEventMask)
		// mask := globalEvent.Masks[idx]
		
		// BƯỚC 2: Lặp qua từng người chơi, kiểm tra xem event j có hợp lệ với họ không
		for i := uint16(0); i < 500; i++ {
			_= &clientSOA.Events_Header[i]
			// if !header.IsActive || header.Cursor > j {
			// 	continue
			// }
			
			// teamID := header.TeamID
			// maskTeamID := teamID >> 6
			// maskAndTeamID := uint64(1) << (teamID & 63)
			// if mask.KnownByTeams[maskTeamID] & maskAndTeamID == 0 {
			// 	continue
			// }
			
			// Nếu event j hợp lệ với người chơi i, ghi vào mảng của họ
			// eventIDs := &clientSOA.Events_EventIDs[i]
			// head := header.Head
			// eventIDs[head&(MaxEvents-1)] = j // Không cần & (MaxEvents-1) vì head cứ tăng
			// header.Head++
		}
	}
	for i := uint16(0); i < 500; i++ {
		if clientSOA.Events_Header[i].IsActive {
			clientSOA.Events_Header[i].Cursor = currentGlobalHead
		}
	}
}
// 2. HÀM BENCHMARK CHÍNH THỨC
func BenchmarkFlushToQueue_SoA(b *testing.B) {
	soa, globalEv := setupBenchmarkData(MaxPlayers)

	// b.ResetTimer() // RẤT QUAN TRỌNG: Chỉ bắt đầu đo sau khi setup xong data

	for i := 0; i < b.N; i++ {
	// 	// Reset lại Cursor sau mỗi vòng chạy b.N để đảm bảo loop bên trong hoạt động
	// 	// b.StopTimer() 
	// 	// for c := 0; c < MaxPlayers; c++ {
	// 		// soa.Events_Header[c].Cursor = globalEv.Head - 1000
	// 	// }
		globalEv.Head += 1000 // Giả sử mỗi lần chạy benchmark, có thêm 1000 events mới được sinh ra
	// 	// 2. BẬT LẠI ĐỒNG HỒ TRƯỚC KHI CHẠY HÀM THỰC SỰfgdfikhnjmbn hnjhhhhujvgbnjk                                               
	// 	// b.StartTimer()

	// 	// Gọi hàm cần đo
		flushToQueue_CurrentLayout(MaxPlayers, soa, globalEv)
	}
}                              
func BenchmarkFlush_Scale_4_Players(b *testing.B) {
	soa, globalEv := setupBenchmarkData(500) // Khởi tạo đủ struct cho 500 players
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		globalEv.Head += 1000
		flushToQueue_CurrentLayout(4, soa, globalEv) // Chỉ xử lý 4 players đầu tiên
	}
}

// Benchmark với 50 players thực tế được xử lý
func BenchmarkFlush_Scale_50_Players(b *testing.B) {
	soa, globalEv := setupBenchmarkData(500)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		globalEv.Head += 1000
		flushToQueue_CurrentLayout(50, soa, globalEv) // Xử lý 50 players
	}
}

// Benchmark với toàn bộ 500 players được xử lý
func BenchmarkFlush_Scale_500_Players(b *testing.B) {
	soa, globalEv := setupBenchmarkData(500)
	
	// Khởi tạo bộ đệm tái sử dụng ngoài vòng lặp Benchmark (Chỉ cấp phát đúng 1 lần)
	// scratchpad := &FlushScratchpad{} 
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		globalEv.Head += 1000
		flushToQueue_CurrentLayout(500, soa, globalEv)
	}
}
type PlayerMaskPrecomputed struct {
	TeamID        uint16
	MaskTeamID    uint16
	MaskAndTeamID uint64
}
type FlushScratchpad struct {
	TempEventIDs [500][128]uint32
	TempCount    [500]uint16
	PlayerMask [500]PlayerMaskPrecomputed
}
func flushToQueue_TemporalIsolation(nextClientID uint16, clientSOA *ClientsSoA2, globalEvent *GlobalEvent, scratchpad *FlushScratchpad) {
	currentGlobalHead := globalEvent.Head

	// Reset lại bộ đệm tái sử dụng (Không cấp phát mới)
	for i := uint16(0); i < nextClientID; i++ {
		scratchpad.TempCount[i] = 0
	}

	// BƯỚC 1: Tính toán trước thông tin Mask cho 500 players
	// var playerMasks [500]PlayerMaskPrecomputed
	minCursor := currentGlobalHead

	for i := uint16(0); i < nextClientID; i++ {
		header := &clientSOA.Events_Header[i]
		if header.IsActive {
			if header.Cursor < minCursor {
				minCursor = header.Cursor
			}
			teamID := header.TeamID
			scratchpad.PlayerMask[i].MaskAndTeamID = uint64(1) << (teamID & 63)
			scratchpad.PlayerMask[i].MaskTeamID = teamID >> 6
			scratchpad.PlayerMask[i].TeamID = teamID

		}
	}

	// ==========================================
	// BƯỚC 2: GATHER PHASE (ĐỌC TUẦN TỰ MASK)
	// ==========================================
	for j := minCursor; j < currentGlobalHead; j++ {
		idx := j & GlobalEventMask
		mask := &globalEvent.Masks[idx]

		for i := uint16(0); i < nextClientID; i++ {
			header := &clientSOA.Events_Header[i]
			if !header.IsActive || header.Cursor > j {
				continue
			}

			info := &scratchpad.PlayerMask[i]
			if mask.KnownByTeams[info.MaskTeamID]&info.MaskAndTeamID == 0 {
				continue
			}

			// Ghi vào bộ đệm tái sử dụng
			count := scratchpad.TempCount[i]
			if count < 128 {
				scratchpad.TempEventIDs[i][count] = j
				scratchpad.TempCount[i]++
			}
		}
	}

	// ==========================================
	// BƯỚC 3: SCATTER PHASE (CẬP NHẬT GỐC)
	// ==========================================
	for i := uint16(0); i < nextClientID; i++ {
		header := &clientSOA.Events_Header[i]
		if !header.IsActive {
			continue
		}

		writeCount := scratchpad.TempCount[i]
		if writeCount > 0 {
			eventIDs := &clientSOA.Events_EventIDs[i]
			head := header.Head

			for k := uint16(0); k < writeCount; k++ {
				eventIDs[head&(MaxEvents-1)] = scratchpad.TempEventIDs[i][k]
				head++
			}
			header.Head = head
		}

		header.Cursor = currentGlobalHead
	}
}
func flushToQueue_CacheBlocked(nextClientID uint16, clientSOA *ClientsSoA2, globalEvent *GlobalEvent) {
	currentGlobalHead := globalEvent.Head

	// BƯỚC 1: Tính toán trước thông tin Mask cho 500 players (L1-hot)
	var playerMasks [500]PlayerMaskPrecomputed
	minCursor := currentGlobalHead

	for i := uint16(0); i < nextClientID; i++ {
		header := &clientSOA.Events_Header[i]
		if header.IsActive {
			if header.Cursor < minCursor {
				minCursor = header.Cursor
			}
			teamID := header.TeamID
			playerMasks[i].MaskAndTeamID = uint64(1) << (teamID & 63)
			playerMasks[i].MaskTeamID = teamID >> 6
			// playerMasks[i].TeamID = teamID
		}
	}

	// Kích thước khối: 256 masks * 32 bytes = 8KB (Vừa vặn hoàn hảo trong L1 32KB)
	const BLOCK_SIZE = 256 

	// VÒNG NGOÀI CÙNG: Chia 1000 events thành các Blocks nhỏ
	for blockStart := minCursor; blockStart < currentGlobalHead; blockStart += BLOCK_SIZE {
		blockEnd := blockStart + BLOCK_SIZE
		if blockEnd > currentGlobalHead {
			blockEnd = currentGlobalHead
		}

		// VÒNG GIỮA: Duyệt qua từng người chơi
		for i := uint16(0); i < nextClientID; i++ {
			header := &clientSOA.Events_Header[i]
			if !header.IsActive {
				continue
			}

			cursor := header.Cursor
			// Nếu Cursor của player đã vượt qua block này, không cần xử lý
			if cursor >= blockEnd {
				continue
			}

			// Xác định điểm bắt đầu thực tế trong block này của Player i
			startJ := cursor
			if startJ < blockStart {
				startJ = blockStart
			}

			eventIDs := &clientSOA.Events_EventIDs[i]
			info := &playerMasks[i]
			head := header.Head

			// VÒNG TRONG CÙNG: Duyệt qua các Event TRONG BLOCK
			// Dữ liệu Masks[startJ..blockEnd] lúc này được nạp và giữ ấm hoàn toàn trong L1
			for j := startJ; j < blockEnd; j++ {
				idx := j & GlobalEventMask
				if globalEvent.Masks[idx].KnownByTeams[info.MaskTeamID]&info.MaskAndTeamID == 0 {
					continue
				}
				eventIDs[head&(MaxEvents-1)] = j
				head++
			}

			// Cập nhật lại Head và Cursor tạm thời cho Player sau khi xong block này
			header.Head = head
			header.Cursor = blockEnd
		}
	}

	// Đảm bảo tất cả player hoạt động đều đồng bộ Cursor với Head toàn cục ở cuối tick
	for i := uint16(0); i < nextClientID; i++ {
		if clientSOA.Events_Header[i].IsActive {
			clientSOA.Events_Header[i].Cursor = currentGlobalHead
		}
	}
}