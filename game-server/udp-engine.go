package main

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
)


const (
    BatchSize = MaxPlayers
    PacketSize = 8192
	SendBatchSize = MaxPlayers
)
type Mmsghdr struct {
	Hdr unix.Msghdr
	Len uint32
	_   [4]byte // Padding cho đúng kích thước 64-bit
}
type UdpEngine struct {
    fd int 
    recvMsgs []Mmsghdr
    recvIovecs []unix.Iovec
    recvBuffers [][]byte
	recvAddrs []unix.RawSockaddrAny

	sendMsgs []Mmsghdr
	sendIovecs []unix.Iovec
	sendBuffers [][]byte
	sendCount int

    sendDataBlock []byte // THÊM DẢI NHỚ LIỀN KỀ
    recvDataBlock []byte
}
func NewUDPEngine(port int) (*UdpEngine, error) {
	fd,err:= unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return nil, err
	}
	unix.SetNonblock(fd,true)
	unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_SNDBUF, 4*1024*1024)
	addr:= &unix.SockaddrInet4{Port: port}
	copy(addr.Addr[:], []byte{0,0,0,0})
	if err := unix.Bind(fd, addr); err != nil {
		unix.Close(fd)
		return nil, err
	}
	engine := &UdpEngine{
		fd: fd,
		recvMsgs: make([]Mmsghdr, BatchSize),
		recvIovecs: make([]unix.Iovec, BatchSize),
		recvBuffers: make([][]byte, BatchSize),
		recvAddrs: make([]unix.RawSockaddrAny, BatchSize),

		sendMsgs: make([]Mmsghdr, SendBatchSize),
		sendIovecs: make([]unix.Iovec, SendBatchSize),
		sendBuffers: make([][]byte, SendBatchSize),
		sendCount: 0,
	}
	engine.recvDataBlock = make([]byte, BatchSize * PacketSize)
    engine.sendDataBlock = make([]byte, SendBatchSize * PacketSize)
	for i := 0; i < BatchSize;i++{
		start := i * PacketSize
        end := start + PacketSize
        engine.recvBuffers[i] = engine.recvDataBlock[start:end]
        
        engine.recvIovecs[i] = unix.Iovec{
            Base: &engine.recvDataBlock[start],
            Len:  uint64(PacketSize),
        }
		engine.recvMsgs[i] = Mmsghdr{
			Hdr: unix.Msghdr{
				Name: (*byte)(unsafe.Pointer(&engine.recvAddrs[i])),
				Namelen: (uint32)(unsafe.Sizeof(unix.RawSockaddrAny{})),
				Iov: &engine.recvIovecs[i],
				Iovlen: 1,		
			},
		}
	}
	for i := 0; i < SendBatchSize;i++{

		start := i * PacketSize
        end := start + PacketSize
        engine.sendBuffers[i] = engine.sendDataBlock[start:end]
        
        engine.sendIovecs[i] = unix.Iovec{
            Base: &engine.sendDataBlock[start],
            Len:  0,
        }

		engine.sendMsgs[i] = Mmsghdr{
			Hdr: unix.Msghdr{
				Iov: &engine.sendIovecs[i],
				Iovlen: 1,

			},
		}
	}

	return engine, nil
}
func (e *UdpEngine) ReadBatch() (int, error) {
	n,_,err:= unix.Syscall6(
		unix.SYS_RECVMMSG,
		uintptr(e.fd),
		uintptr(unsafe.Pointer(&e.recvMsgs[0])),
		uintptr(BatchSize),
		0,
		0,
		0)
	if err != 0 {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return 0, nil
		}
		return 0,  fmt.Errorf("Lỗi Syscall: %v", err)
	}
	return int(n), nil
}
func (e *UdpEngine) ProcessPackets(n int ){
	for i:=0; i< n; i++ {
		packetLen := e.recvMsgs[i].Len
		data := e.recvBuffers[i][:packetLen]
		rawAddr := &e.recvAddrs[i]
		if rawAddr.Addr.Family == unix.AF_INET {
			addr4 := (*unix.RawSockaddrInet4)(unsafe.Pointer(rawAddr))
			ip := addr4.Addr
			port := (addr4.Port >> 8) | (addr4.Port << 8)

		fmt.Printf("[Gói tin %d] Từ %d.%d.%d.%d:%d | Dài %d bytes | Data: %s\n",
				i, ip[0], ip[1], ip[2], ip[3], port, packetLen, string(data))
		}
	}
}
func (e *UdpEngine) QueueToSend(data []byte, desAddr *unix.RawSockaddrAny) {
	dataLen := uint64(len(data))
	if e.sendCount >= SendBatchSize {
		e.FlushSend()
	}
	
	idx := e.sendCount
	copy(e.sendBuffers[idx], data)
	e.sendIovecs[idx].SetLen(len(data))
	e.sendMsgs[idx].Hdr.Name = (*byte)(unsafe.Pointer(desAddr))
	e.sendMsgs[idx].Hdr.Namelen = uint32(unsafe.Sizeof(unix.RawSockaddrAny{}))
	e.sendCount++

	// --- THỐNG KÊ CHI TIẾT ---
	atomic.AddUint64(&Metrics.TotalBytes, dataLen)
	atomic.AddUint64(&Metrics.currentTickBytes, dataLen)
	
	// Phân loại kích thước gói
	bucketIdx := 0
	if dataLen <= 100 { bucketIdx = 0 } else 
	if dataLen <= 500 { bucketIdx = 1 } else 
	if dataLen <= 1000 { bucketIdx = 2 } else 
	if dataLen <= 1200 { bucketIdx = 3 } else { bucketIdx = 4 }
	atomic.AddUint64(&Metrics.PacketSizeBuckets[bucketIdx], 1)
}

func (e *UdpEngine) FlushSend() error {
	if e.sendCount == 0 {
		return nil
	}
	
	// Theo dõi đỉnh băng thông trong 1 tick
	currTick := atomic.SwapUint64(&Metrics.currentTickBytes, 0)
	for {
		max := atomic.LoadUint64(&Metrics.MaxBytesInTick)
		if currTick <= max || atomic.CompareAndSwapUint64(&Metrics.MaxBytesInTick, max, currTick) {
			break
		}
	}

	atomic.AddUint64(&Metrics.CallCount, 1)
	atomic.AddUint64(&Metrics.BatchDistribution[e.sendCount], 1)

	n, _, err := unix.RawSyscall6(
		unix.SYS_SENDMMSG,
		uintptr(e.fd),
		uintptr(unsafe.Pointer(&e.sendMsgs[0])),
		uintptr(e.sendCount),
		0, 0, 0,
	)

	if err != 0 {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			atomic.AddUint64(&Metrics.DroppedPackets, uint64(e.sendCount))
		}
		e.sendCount = 0
		return nil
	}

	atomic.AddUint64(&Metrics.TotalPackets, uint64(n))
	if int(n) < e.sendCount {
		atomic.AddUint64(&Metrics.DroppedPackets, uint64(e.sendCount-int(n)))
	}

	e.sendCount = 0
	return nil
}

// func main() {
// 	engine, err := NewUDPEngine(9000)
// 	if err != nil {
// 		panic(err)
// 	}
// 	//////fmt.println("Server UDP đang chạy tại port 9000...")
	
// 	for {
// 		n, err := engine.ReadBatch()
// 		if err != nil {
// 			//////fmt.println("Lỗi:", err)
// 			continue
// 		}
// 		if n > 0 {
// 			engine.ProcessPackets(n)
// 		}
// 	}
// }