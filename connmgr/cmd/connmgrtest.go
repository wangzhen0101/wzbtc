package main

import (
	"errors"
	"fmt"
	"github.com/wangzhen0101/wzbtc/addrmgr"
	"github.com/wangzhen0101/wzbtc/bclog"
	"github.com/wangzhen0101/wzbtc/chaincfg"
	"github.com/wangzhen0101/wzbtc/connmgr"
	"github.com/wangzhen0101/wzbtc/wire"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type logWriter struct{}

func (l logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return len(p), nil
}

var (
	backendLog      = bclog.NewBackend(logWriter{})
	log             = backendLog.Logger("TEST")
	amgrLog         = backendLog.Logger("ADMR")
	cngrLog         = backendLog.Logger("CNGR")
	activeNetParams = &chaincfg.MainNetParams
	adrmgr          *addrmgr.AddrManager
	conmgr          *connmgr.ConnManager
)

func init() {
	log.SetLevel(bclog.LevelTrace)
	amgrLog.SetLevel(bclog.LevelTrace)
	cngrLog.SetLevel(bclog.LevelTrace)
	connmgr.UseLogger(amgrLog)
	addrmgr.UseLogger(cngrLog)
}

// simpleAddr implements the net.Addr interface with two struct fields
type simpleAddr struct {
	net, addr string
}

// String returns the address.
//
// This is part of the net.Addr interface.
func (a simpleAddr) String() string {
	return a.addr
}

// Network returns the network.
//
// This is part of the net.Addr interface.
func (a simpleAddr) Network() string {
	return a.net
}

// Ensure simpleAddr implements the net.Addr interface.
var _ net.Addr = simpleAddr{}

func parseListeners(addrs []string) ([]net.Addr, error) {
	netAddrs := make([]net.Addr, 0, len(addrs)*2)
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// Shouldn't happen due to already being normalized.
			return nil, err
		}

		// Empty host or host of * on plan9 is both IPv4 and IPv6.
		if host == "" || (host == "*" && runtime.GOOS == "plan9") {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp4", addr: addr})
			netAddrs = append(netAddrs, simpleAddr{net: "tcp6", addr: addr})
			continue
		}

		// Strip IPv6 zone id if present since net.ParseIP does not
		// handle it.
		zoneIndex := strings.LastIndex(host, "%")
		if zoneIndex > 0 {
			host = host[:zoneIndex]
		}

		// Parse the IP.
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("'%s' is not a valid IP address", host)
		}

		// To4 returns nil when the IP is not an IPv4 address, so use
		// this determine the address type.
		if ip.To4() == nil {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp6", addr: addr})
		} else {
			netAddrs = append(netAddrs, simpleAddr{net: "tcp4", addr: addr})
		}
	}
	return netAddrs, nil
}

func initListeners(adrmgr *addrmgr.AddrManager, listenAddrs []string, flag wire.ServiceFlag) ([]net.Listener, error) {
	for _, ad := range listenAddrs {
		log.Infof("listen addr:%s", ad)
	}

	addrs, err := parseListeners(listenAddrs)
	if err != nil {
		log.Errorf("parseListeners %s fail:%s", listenAddrs, err)
		return nil, err
	}

	listeners := make([]net.Listener, 0, len(addrs))
	for idx, addr := range addrs {
		log.Infof("%d start listen on :%s-%s", idx, addr.Network(), addr.String())
		listener, err := net.Listen(addr.Network(), addr.String())
		if err != nil {
			log.Errorf("Can't listen on %s, %v", addr, err)
			continue
		}
		listeners = append(listeners, listener)
	}

	return listeners, nil
}

func inboundPeerConnected(conn net.Conn) {
	log.Infof("Exec func inboundPeerConnected")
	return
}

func outboundPeerConnected(c *connmgr.ConnReq, conn net.Conn) {
	log.Infof("Exec func outboundPeerConnected")
	buf := make([]byte, 1024)

	_, err := conn.Read(buf)
	if err != nil {
		if err == io.EOF {
			log.Warnf("%v 关闭连接", c)
		}

		if c.Permanent {
			conmgr.Disconnect(c.ID())
		} else {
			conmgr.Remove(c.ID())
			go conmgr.NewConnReq()
		}
	}

	return
}

func btcdDial(addr net.Addr) (net.Conn, error) {
	return net.DialTimeout(addr.Network(), addr.String(), time.Second*30)
}

//func newAddressFunc() (net.Addr, error) {
//	log.Infof("Exec func GetNewAddress")
//	addrStr := "34.85.0.160:8333"
//	host, strPort, err := net.SplitHostPort(addrStr)
//	if err != nil {
//		return nil, err
//	}
//
//	ip := net.ParseIP(host)
//	if ip == nil {
//		return nil, errors.New("invalid ip")
//	}
//
//	port, err := strconv.Atoi(strPort)
//	if err != nil {
//		return nil, err
//	}
//
//	log.Infof("Get new address :%s-%d", ip, port)
//
//	return &net.TCPAddr{
//		IP:   ip,
//		Port: port,
//	}, nil
//}

func newAddressFunc() (net.Addr, error) {
	for tries := 0; tries < 100; tries++ {
		addr := adrmgr.GetAddress()
		if addr == nil {
			log.Warnf("GetAddresses return nil,Address number:%d", adrmgr.NumAddresses())
			break
		}

		// Address will not be invalid, local or unroutable
		// because addrmanager rejects those on addition.
		// Just check that we don't already have an address
		// in the same group so that we are not connecting
		// to the same network segment at the expense of
		// others.
		key := addrmgr.GroupKey(addr.NetAddress())
		log.Infof("GroupKey:%s", key)

		// only allow recent nodes (10mins) after we failed 30
		// times
		if tries < 30 && time.Since(addr.LastAttempt()) < 10*time.Minute {
			continue
		}

		// allow nondefault ports after 50 failed tries.
		if tries < 50 && fmt.Sprintf("%d", addr.NetAddress().Port) !=
			activeNetParams.DefaultPort {
			continue
		}

		// Mark an attempt for the valid address.
		adrmgr.Attempt(addr.NetAddress())

		addrString := addrmgr.NetAddressKey(addr.NetAddress())
		return addrStringToNetAddr(addrString)
	}

	return nil, errors.New("no valid connect address")
}

type onionAddr struct {
	addr string
}

// String returns the onion address.
//
// This is part of the net.Addr interface.
func (oa *onionAddr) String() string {
	return oa.addr
}

// Network returns "onion".
//
// This is part of the net.Addr interface.
func (oa *onionAddr) Network() string {
	return "onion"
}

// Ensure onionAddr implements the net.Addr interface.
var _ net.Addr = (*onionAddr)(nil)

func addrStringToNetAddr(addr string) (net.Addr, error) {
	host, strPort, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(strPort)
	if err != nil {
		return nil, err
	}

	// Skip if host is already an IP address.
	if ip := net.ParseIP(host); ip != nil {
		return &net.TCPAddr{
			IP:   ip,
			Port: port,
		}, nil
	}

	// Tor addresses cannot be resolved to an IP, so just return an onion
	// address instead.
	if strings.HasSuffix(host, ".onion") {
		return &onionAddr{addr: addr}, nil
	}

	// Attempt to look up an IP address associated with the parsed host.
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}

	return &net.TCPAddr{
		IP:   ips[0],
		Port: port,
	}, nil
}

func main() {
	cmdPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Infof("get path fial:%s", err)
	}
	log.Infof("cmdPath:%s", cmdPath)

	//cmgr, err := connmgr.New(&connmgr.Config{
	//	Listeners:      listeners,
	//	OnAccept:       s.inboundPeerConnected,
	//	RetryDuration:  connectionRetryInterval,
	//	TargetOutbound: uint32(targetOutbound),
	//	Dial:           net.DialTimeout,
	//	OnConnection:   s.outboundPeerConnected,
	//	GetNewAddress:  newAddressFunc,
	//})

	adrmgr = addrmgr.New(cmdPath, net.LookupIP)

	//从配置文件中获取已经存在的server节点
	adrmgr.Start()

	connmgr.SeedFromDNS(activeNetParams, wire.SFNodeNetwork, net.LookupIP, func(addrs []*wire.NetAddress) {
		adrmgr.AddAddresses(addrs, addrs[0])
	})

	listenAddr := make([]string, 0, 32)
	listenAddr = append(listenAddr, "0.0.0.0:8333")
	listeners, err := initListeners(adrmgr, listenAddr, wire.SFNodeNetwork)
	if err != nil {
		log.Errorf("Can't start listen on %s", listenAddr)
		return
	}

	conmgr, err = connmgr.New(&connmgr.Config{
		Listeners:      listeners,
		OnAccept:       inboundPeerConnected,
		TargetOutbound: 200,
		RetryDuration:  time.Second * 10,
		OnConnection:   outboundPeerConnected,
		GetNewAddress:  newAddressFunc,
		Dial:           btcdDial,
	})

	nt1, err := newAddressFunc()
	if err != nil {
		log.Errorf("Can't get net address:%s", err)
		return
	}

	log.Infof("Get net address:%s", nt1)

	conmgr.Start()

	conmgr.Wait()

	log.Infof("Main func Done.")
}
