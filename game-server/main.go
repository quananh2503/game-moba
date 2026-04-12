package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"golang.org/x/sys/unix"
)

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
	server := NewGameServer()

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
	go server.StartLoop(engine)
	//////fmt.println("🔥 MOBA Server đã sẵn sàng tại port 9000 (Epoll Optimized)...")

	events := make([] unix.EpollEvent,1)
	for {
		_,err:=unix.EpollWait(epollFD,events,-1)
		if err !=nil{
			if err==unix.EINTR{continue}
			//////fmt.println("Lỗi EpollWait:", err)
			break
		}
		n,_:=engine.ReadBatch()
		if n>0{
			for i:=0;i<n;i++{
				packetLen:=engine.recvMsgs[i].Len
				data := engine.recvBuffers[i][:packetLen]
				server.HandlePacket(data,&engine.recvAddrs[i])
			}
		}
	}

}
