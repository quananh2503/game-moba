package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

type Server struct{
	netIO * NetworkIO
	inputs *[200]atomic.Uint64
	sessions *SessionManager
	world *World
	state *MatchState
	pendingId []uint16
	packetBuffer *PacketBuffer
	
}
func NewServer(udpEngine *UdpEngine) *Server {
	// Tạo các thùng Data và Engine
	sessions := NewSessionManager()
	inputs := [200]atomic.Uint64{}

	return &Server{
		sessions: sessions,
		netIO:    NewNetworkIO(udpEngine, &inputs ),
		world:    NewWord(),
		state:    &MatchState{ /*...khởi tạo...*/ },
		inputs:   &inputs,
		pendingId: make([]uint16, 0,256),
	}
}
func( s *Server)StartLoop(){
	ticker := time.NewTicker(time.Second / TickRate)
	dt := float32(0.016)
	outbox := NewNetworkOutbox() // Tái sử dụng mỗi tick

	SpawnMapObjects(s.world.Engine)

	for {
		<-ticker.C
		s.state.TimeNow = time.Now()
		s.state.TickCount++


		s.sessions.ProcessRawPackets(s.packetBuffer,s.inputs,&s.pendingId)
		s.world.AcceptPendingclients(&s.pendingId)

		s.world.Tick(dt, s.inputs, outbox)
		s.netIO.BroadcastState(s.world.Engine, s.state)

	}
}

func main() {
	go func() {
		//fmt.println("🔍 Hệ thống PPROF đang chạy tại http://localhost:6060/debug/pprof/")
		// Lưu ý: Tham số thứ 2 PHẢI LÀ nil để nó dùng DefaultServeMux của Go
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			fmt.Println("loi khoi dong ",err)
		}
	}()
	
	// 1. Khởi tạo Engine UDP
	engine, err := NewUDPEngine(9000)
	if err != nil {
		panic(err)
	}
	// 2. Khởi tạo Bộ não Game
	server := NewServer(engine)

	//////fmt.println("Server UDP đang chạy tại port 9000...")

	epollFD,err:=unix.EpollCreate1(0)
	if err!=nil{
		panic(err)
	}
	event := unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd: int32(engine.fd),
	}
	if err:= unix.EpollCtl(epollFD,unix.EPOLL_CTL_ADD,engine.fd,&event);err!=nil{
		panic(err)
	}
	go server.StartLoop()
	//////fmt.println("🔥 MOBA Server đã sẵn sàng tại port 9000 (Epoll Optimized)...")

	events := make([] unix.EpollEvent,1)
	for {
		_,err:=unix.EpollWait(epollFD,events,-1)
		if err !=nil{
			if err==unix.EINTR{continue}
			//////fmt.println("Lỗi EpollWait:", err)
			break
		}
		server.netIO.ReadBatch(server.packetBuffer)
	}

}
