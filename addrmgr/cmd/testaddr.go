package main

import (
	"github.com/wangzhen0101/wzbtc/addrmgr"
	"github.com/wangzhen0101/wzbtc/bclog"
	"github.com/wangzhen0101/wzbtc/chaincfg"
	"github.com/wangzhen0101/wzbtc/connmgr"
	"github.com/wangzhen0101/wzbtc/wire"
	"net"
	"os"
	"path/filepath"
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
	activeNetParams = &chaincfg.MainNetParams
)

func init() {
	log.SetLevel(bclog.LevelTrace)
	addrmgr.UseLogger(log)
}

func main() {
	cmdPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Infof("get path fial:%s", err)
	}
	log.Infof("cmdPath:%s", cmdPath)

	adrmgr := addrmgr.New(cmdPath, net.LookupIP)
	//dst := wire.NewNetAddress(&net.TCPAddr{[]byte("138.152.1.8"), 53333, ""}, wire.SFNodeNetwork|wire.SFNodeBloom)
	//src := wire.NewNetAddress(&net.TCPAddr{[]byte("127.0.0.1"),   53333, ""}, wire.SFNodeNetwork|wire.SFNodeBloom)
	//adrmgr.AddAddress(dst, src)
	connmgr.SeedFromDNS(activeNetParams, wire.SFNodeNetwork, net.LookupIP, func(addrs []*wire.NetAddress) {
		adrmgr.AddAddresses(addrs, addrs[0])
	})

	adrmgr.Start()

	time.Sleep(time.Second * 20)

	log.Infof("%s", adrmgr.GetAddress())
	log.Infof("host number:%d", adrmgr.NumAddresses())

	adrmgr.Wait()

	log.Infof("Main func Done.")
}
