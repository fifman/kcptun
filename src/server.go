package src

import (
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/xtaci/kcp-go"
	"github.com/xtaci/smux"
)

// handle multiplex-ed connection
func handleMux(conn io.ReadWriteCloser, config *Config) {
	// stream multiplex
	smuxConfig := smux.DefaultConfig()
	smuxConfig.MaxReceiveBuffer = config.SockBuf
	smuxConfig.KeepAliveInterval = time.Duration(config.KeepAlive) * time.Second

	mux, err := smux.Server(conn, smuxConfig)
	if err != nil {
		log.Println(err)
		return
	}
	defer mux.Close()
	for {
		p1, err := mux.AcceptStream()
		if err != nil {
			log.Println(err)
			return
		}
		p2, err := net.DialTimeout("tcp", config.Target, 5*time.Second)
		if err != nil {
			p1.Close()
			log.Println(err)
			continue
		}
		go handleClient(p1, p2, config.Quiet)
	}
}

func RunServer(config ServerConfig) {
	setLogFile(config.Config)
	log.Println("version:", VERSION)
	log.Println("initiating key derivation")
	block := createBlock(config.Config)
	lis, err := kcp.ListenWithOptions(config.Listen, block, config.DataShard, config.ParityShard)
	checkError(err)
	logConfig(config.Config)
	log.Println("pprof:", config.Pprof)

	if err := lis.SetDSCP(config.DSCP); err != nil {
		log.Println("SetDSCP:", err)
	}
	if err := lis.SetReadBuffer(config.SockBuf); err != nil {
		log.Println("SetReadBuffer:", err)
	}
	if err := lis.SetWriteBuffer(config.SockBuf); err != nil {
		log.Println("SetWriteBuffer:", err)
	}

	go snmpLogger(config.SnmpLog, config.SnmpPeriod)
	if config.Pprof {
		go http.ListenAndServe(":6060", nil)
	}

	for {
		if conn, err := lis.AcceptKCP(); err == nil {
			log.Println("remote address:", conn.RemoteAddr())
			conn.SetStreamMode(true)
			conn.SetWriteDelay(false)
			conn.SetNoDelay(config.NoDelay, config.Interval, config.Resend, config.NoCongestion)
			conn.SetMtu(config.MTU)
			conn.SetWindowSize(config.SndWnd, config.RcvWnd)
			conn.SetACKNoDelay(config.AckNodelay)

			if config.NoComp {
				go handleMux(conn, &config.Config)
			} else {
				go handleMux(newCompStream(conn), &config.Config)
			}
		} else {
			log.Printf("%+v", err)
		}
	}
}
