package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)


const (
    BatchSize = 256
    PacketSize = 8192
	SendBatchSize = 256
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
}
func NewUDPEngine(port int) (*UdpEngine, error) {
	fd,err:= unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return nil, err
	}
	unix.SetNonblock(fd,true)
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
	for i := 0; i < BatchSize;i++{
		engine.recvBuffers[i] = make([]byte, PacketSize)
		engine.recvIovecs[i] =unix.Iovec{
			Base: (*byte) (&engine.recvBuffers[i][0]),
			Len: PacketSize,
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
		engine.sendBuffers[i] = make([]byte, PacketSize)
		engine.sendIovecs[i] = unix.Iovec{
			Base: &engine.sendBuffers[i][0],
			Len: 0,
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
func(e *UdpEngine)QueueToSend(data []byte, desAddr *unix.RawSockaddrAny)bool{
	if e.sendCount>=SendBatchSize{
		return true
	}
	idx:= e.sendCount
	copy(e.sendBuffers[idx],data)
	// //////fmt.println("copy ",len(data)," xuong xe cho hang")
	e.sendIovecs[idx].SetLen(len(data))
	e.sendMsgs[idx].Hdr.Name = (*byte)(unsafe.Pointer(desAddr))
	e.sendMsgs[idx].Hdr.Namelen = uint32(unsafe.Sizeof(unix.RawSockaddrAny{}))
	e.sendCount++
	return e.sendCount>=SendBatchSize
}
func (e *UdpEngine) FlushSend() error {
	if e.sendCount == 0 {
		return nil
	}

	sent := 0
	for sent < e.sendCount {
        // DÙNG RawSyscall6 cho socket Non-blocking!
		n, _, err := unix.RawSyscall6(
			unix.SYS_SENDMMSG,
			uintptr(e.fd),
			uintptr(unsafe.Pointer(&e.sendMsgs[sent])), // Đẩy con trỏ tới gói chưa gửi
			uintptr(e.sendCount - sent),
			0, 0, 0,
		)

		if err != 0 {
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				// Mạng OS đang kẹt. Trong Game Server, ta có 2 cách:
                // Cách 1: Vòng lặp chờ (Spin-wait) một chút rồi gửi lại (Nguy hiểm nếu kẹt lâu)
                // Cách 2: Chấp nhận DROP (rớt) các gói tin còn lại của Batch này để không làm đứng Server.
                // Tạm thời, tôi để Break (Chấp nhận rớt gói nếu quá tải mạng, udp mà!)
                break 
			}
			return fmt.Errorf("Lỗi Sendmmsg: %v", err)
		}
		
		sent += int(n) // n là số gói tin đã đẩy thành công vào OS Buffer
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