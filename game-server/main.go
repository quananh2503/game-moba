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
	mapDataCache []byte
	// visions 
	
}
type MapNetEntity struct{
	NetID uint16 
	Entity Entity
	TeamID uint8
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
		packetBuffer: &PacketBuffer{
			Packets: make([]RawPacket, 0, 1024),
		},
	}
}
func( s *Server)StartLoop(){
	ticker := time.NewTicker(time.Second / TickRate)
	dt := float32(0.016)
	outbox := NewNetworkOutbox() // Tái sử dụng mỗi tick

	SpawnMapObjects(s.world.Engine)
	s.CacheMapData()
	delsEntities :=make([]Entity,0,256)
	acceptEntities:=make([]MapNetEntity,0,256)

	var totalWorkTime time.Duration
	var maxTickTime time.Duration
	var tickCount int
	lastReport := time.Now()
	targetTickTime := time.Second / TickRate //
	for {
		
		<-ticker.C
		start := time.Now()
		s.state.TimeNow = start
		s.state.TickCount++
		// fmt.Println("vao day roi ",s.state.TickCount)
		// start := time.Now()

		s.sessions.ProcessRawPackets(s.packetBuffer,s.inputs,&s.pendingId,s.state)
		// fmt.Println("toi day roi")
		s.world.AcceptPendingclients(&s.pendingId,&acceptEntities)
		s.sessions.CheckTimeouts(s.state.TickCount,&delsEntities)
		s.world.RemoveEntities(&delsEntities)
		// fmt.Println("toi day roi")
		s.sessions.SyncNetEntity(&acceptEntities,s.mapDataCache)
	
		s.world.Tick(dt, s.inputs, outbox)
		s.sessions.FlushNetworkOutbox(outbox)
		
		s.netIO.BroadcastState(s.world.Engine, s.state,&s.sessions.clients,outbox)
		outbox.Reset()

		elapsed := time.Since(start)
		totalWorkTime += elapsed
		tickCount++
		if elapsed > maxTickTime {
			maxTickTime = elapsed
		}
		if time.Since(lastReport) >= time.Second {
			avgTick := totalWorkTime / time.Duration(tickCount)
			
			// Tính toán Load % (Ví dụ: 8ms / 16.6ms = 48% Load)
			loadPercent := float64(avgTick.Microseconds()) / float64(targetTickTime.Microseconds()) * 100
			
			// fmt.Printf("\n[📊 SERVER PERFORMANCE]\n")
			fmt.Printf("Tick Rate: %d TPS | ", tickCount)
			fmt.Printf("Avg Work Time: %v | Max Tick: %v| ", avgTick, maxTickTime)
			fmt.Printf("CPU Game Load: %.2f%%| ", loadPercent)
			fmt.Printf("Entities: %d | Active Clients: %d| ", s.world.Engine.NextIndex, s.sessions.nextClientID)

			if avgTick > targetTickTime {
				fmt.Printf("⚠️  [WARNING] Server đang bị LAG! Logic chậm hơn 16.6ms| ")
			}
			fmt.Printf("---------------------------\n")

			// Reset bộ đếm cho giây tiếp theo
			totalWorkTime = 0
			maxTickTime = 0
			tickCount = 0
			lastReport = time.Now()
		}
		// fmt.Println("toi day roi")

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
