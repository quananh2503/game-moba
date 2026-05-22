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
	inputs *[MaxPlayers]atomic.Uint64
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
	TeamID uint16
}
func NewServer(udpEngine *UdpEngine) *Server {
	// Tạo các thùng Data và Engine
	sessions := NewSessionManager()
	inputs := [MaxPlayers]atomic.Uint64{}

	return &Server{
		sessions: sessions,
		netIO:    NewNetworkIO(udpEngine, &inputs ),
		world:    NewWord(),
		state:    &MatchState{ /*...khởi tạo...*/ },
		inputs:   &inputs,
		pendingId: make([]uint16, 0,MaxPlayers),
		packetBuffer: &PacketBuffer{
			Packets: make([]RawPacket, 0, 1024),
		},
	}
}
var sumEvent int
func( s *Server)StartLoop(){
	InitVisionTemplates()
	InitMathTables()
	ticker := time.NewTicker(time.Second / TickRate)
	dt := float32(0.016)
	GlobalEvent := &GlobalEvent{
		
	}
	frameShapshot := make([]SnapShotData, 0, MaxPlayers)
	finalSnapshot := FinalSnapshot{}


	// SpawnMapObjects(s.world.Engine)
	// s.CacheMapData()
	delsEntities :=make([]Entity,0,MaxPlayers)
	acceptEntities:=make([]MapNetEntity,0,MaxPlayers)

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

		s.sessions.ProcessRawPackets(s.packetBuffer,s.inputs,&s.pendingId,s.state,s.state.TickCount, GlobalEvent.Head)
		s.sessions.ProcessBatchAck(s.sessions.acks)
		// fmt.Println("toi day roi")
		s.world.AcceptPendingclients(&s.pendingId,&acceptEntities)
		s.sessions.CheckTimeouts(s.state.TickCount,&delsEntities)
		s.world.RemoveEntities(&delsEntities)
		// fmt.Println("toi day roi")
		s.sessions.SyncNetEntity(&acceptEntities, GlobalEvent)
	
		s.world.Tick(dt, s.inputs, GlobalEvent, &frameShapshot)
		
		s.sessions.FlushToQueue(GlobalEvent)
		// a := s.sessions.clients
		s.netIO.GatherVisibleSnapshots(s.sessions.nextClientID, frameShapshot, frameShapshot, s.sessions.clientSOA, &finalSnapshot)
		s.netIO.WriteBatch(GlobalEvent, frameShapshot, s.sessions.clientSOA, s.state.TickCount, &finalSnapshot) 
		frameShapshot = frameShapshot[:0] // Reset slice mà không tạo lại bộ nhớ
		

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
			fmt.Printf("event global head: %d ", GlobalEvent.Head)
			fmt.Printf("event added: %d ", sumEvent/(TickRate* 1000))
			fmt.Printf("event head : %d ", s.sessions.clientSOA.Events_Header[53].Head)
			fmt.Printf("---------------------------\n")

			// Reset bộ đếm cho giây tiếp theo
			totalWorkTime = 0
			maxTickTime = 0
			tickCount = 0
			sumEvent = 0
			lastReport = time.Now()
		}
		// fmt.Println("toi day roi")

	}
}
func StartHTTPServer(mapData []byte) {
	http.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		w.Write(mapData) 
	})

	fmt.Println("HTTP Server mở tại cổng 8080 để tải Map...")
	http.ListenAndServe(":8080", nil)
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
	SpawnMapObjects(server.world.Engine)
	server.CacheMapData()

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
	go StartHTTPServer(server.mapDataCache)
	// StartNetworkReporter()
	
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
