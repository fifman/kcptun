package src

import (
	"io"
	"log"
	"net"
	"time"

	"github.com/xtaci/kcp-go"
	"github.com/xtaci/smux"

	"github.com/pkg/errors"
)

func handleClient2(sess *smux.Session, p1 io.ReadWriteCloser, quiet bool) {
	p2, err := sess.OpenStream()
	if err != nil {
		return
	}
	handleClient(p1, p2, quiet)
}

func RunClient(config ClientConfig) {
	setLogFile(config.Config)
	log.Println("version:", VERSION)
	addr, err := net.ResolveTCPAddr("tcp", config.Listen)
	checkError(err)
	listener, err := net.ListenTCP("tcp", addr)
	checkError(err)

	log.Println("initiating key derivation")
	block := createBlock(config.Config)

	log.Println("listening on:", listener.Addr())
	logConfig(config.Config)
	log.Println("conn:", config.Conn)
	log.Println("autoexpire:", config.AutoExpire)
	log.Println("scavengettl:", config.ScavengeTTL)

	smuxConfig := smux.DefaultConfig()
	smuxConfig.MaxReceiveBuffer = config.SockBuf
	smuxConfig.KeepAliveInterval = time.Duration(config.KeepAlive) * time.Second

	createConn := func() (*smux.Session, error) {
		kcpconn, err := kcp.DialWithOptions(config.Target, block, config.DataShard, config.ParityShard)
		if err != nil {
			return nil, errors.Wrap(err, "createConn()")
		}
		kcpconn.SetStreamMode(true)
		kcpconn.SetWriteDelay(false)
		kcpconn.SetNoDelay(config.NoDelay, config.Interval, config.Resend, config.NoCongestion)
		kcpconn.SetWindowSize(config.SndWnd, config.RcvWnd)
		kcpconn.SetMtu(config.MTU)
		kcpconn.SetACKNoDelay(config.AckNodelay)

		if err := kcpconn.SetDSCP(config.DSCP); err != nil {
			log.Println("SetDSCP:", err)
		}
		if err := kcpconn.SetReadBuffer(config.SockBuf); err != nil {
			log.Println("SetReadBuffer:", err)
		}
		if err := kcpconn.SetWriteBuffer(config.SockBuf); err != nil {
			log.Println("SetWriteBuffer:", err)
		}

		// stream multiplex
		var session *smux.Session
		if config.NoComp {
			session, err = smux.Client(kcpconn, smuxConfig)
		} else {
			session, err = smux.Client(newCompStream(kcpconn), smuxConfig)
		}
		if err != nil {
			return nil, errors.Wrap(err, "createConn()")
		}
		log.Println("connection:", kcpconn.LocalAddr(), "->", kcpconn.RemoteAddr())
		return session, nil
	}

	// wait until a connection is ready
	waitConn := func() *smux.Session {
		for {
			if session, err := createConn(); err == nil {
				return session
			} else {
				log.Println("re-connecting:", err)
				time.Sleep(time.Second)
			}
		}
	}
	numconn := uint16(config.Conn)
	muxes := make([]struct {
		session *smux.Session
		ttl     time.Time
	}, numconn)

	for k := range muxes {
		muxes[k].session = waitConn()
		muxes[k].ttl = time.Now().Add(time.Duration(config.AutoExpire) * time.Second)
	}

	chScavenger := make(chan *smux.Session, 128)
	go scavenger(chScavenger, config.ScavengeTTL)
	go snmpLogger(config.SnmpLog, config.SnmpPeriod)
	rr := uint16(0)
	for {
		p1, err := listener.AcceptTCP()
		if err != nil {
			log.Fatalln(err)
		}
		checkError(err)
		idx := rr % numconn

		// do auto expiration && reconnection
		if muxes[idx].session.IsClosed() || (config.AutoExpire > 0 && time.Now().After(muxes[idx].ttl)) {
			chScavenger <- muxes[idx].session
			muxes[idx].session = waitConn()
			muxes[idx].ttl = time.Now().Add(time.Duration(config.AutoExpire) * time.Second)
		}

		go handleClient2(muxes[idx].session, p1, config.Quiet)
		rr++
	}
}

type scavengeSession struct {
	session *smux.Session
	ts      time.Time
}

func scavenger(ch chan *smux.Session, ttl int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var sessionList []scavengeSession
	for {
		select {
		case sess := <-ch:
			sessionList = append(sessionList, scavengeSession{sess, time.Now()})
			log.Println("session marked as expired")
		case <-ticker.C:
			var newList []scavengeSession
			for k := range sessionList {
				s := sessionList[k]
				if s.session.NumStreams() == 0 || s.session.IsClosed() {
					log.Println("session normally closed")
					s.session.Close()
				} else if ttl >= 0 && time.Since(s.ts) >= time.Duration(ttl)*time.Second {
					log.Println("session reached scavenge ttl")
					s.session.Close()
				} else {
					newList = append(newList, sessionList[k])
				}
			}
			sessionList = newList
		}
	}
}
