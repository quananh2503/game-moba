package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

type NetworkMetrics struct {
	CallCount        uint64
	TotalPackets     uint64
	TotalBytes       uint64
	DroppedPackets   uint64
	
	// Histogram kích thước gói (Buckets: 0-100, 101-500, 501-1000, 1001-1200, >1200)
	PacketSizeBuckets [5]uint64 

	// Thống kê theo Tick (được cập nhật bởi FlushSend)
	MaxBytesInTick   uint64
	currentTickBytes uint64 

	BatchDistribution [MaxPlayers + 5]uint64
}

var Metrics NetworkMetrics
func StartNetworkReporter() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			calls := atomic.SwapUint64(&Metrics.CallCount, 0)
			if calls == 0 { continue }

			totalPkts := atomic.SwapUint64(&Metrics.TotalPackets, 0)
			totalBytes := atomic.SwapUint64(&Metrics.TotalBytes, 0)
			dropped := atomic.SwapUint64(&Metrics.DroppedPackets, 0)
			maxTickBytes := atomic.SwapUint64(&Metrics.MaxBytesInTick, 0)

			fmt.Printf("\n--- 🌐 NETWORK DIAGNOSTICS (Last 10s) ---\n")
			fmt.Printf("Băng thông: %.2f MB/s\n", float64(totalBytes)/(1024*1024)/10.0)
			fmt.Printf("Tổng gói tin thành công: %d | Bị rớt: %d (%.2f%%)\n", totalPkts, dropped, float64(dropped)/float64(totalPkts+dropped+1)*100)
			fmt.Printf("Lưu lượng đỉnh/Tick: %.2f KB\n", float64(maxTickBytes)/1024.0)
			fmt.Printf("Kích thước gói TB: %d bytes\n", totalBytes/(totalPkts+1))
			
			fmt.Printf("\nPhân bổ kích thước gói:\n")
			buckets := [5]string{"0-100B", "101-500B", "501-1000B", "1001-1200B", ">1200B"}
			for i, label := range buckets {
				count := atomic.SwapUint64(&Metrics.PacketSizeBuckets[i], 0)
				fmt.Printf("  %-12s: %d gói\n", label, count)
			}

			fmt.Printf("\nTop Batch Sizes (Gói/Syscall):\n")
			for i := 1; i <= MaxPlayers; i++ {
				count := atomic.SwapUint64(&Metrics.BatchDistribution[i], 0)
				if count > (calls / 100) { // Chỉ in những size chiếm > 1% tổng số lần gọi
					fmt.Printf("  [%d gói]: %d lần\n", i, count)
				}
			}
			fmt.Printf("----------------------------------------\n")
		}
	}()
}