package grdp

import (
	"errors"
	"fmt"
	"github.com/18121861183/grdp/core"
	"github.com/18121861183/grdp/glog"
	"github.com/18121861183/grdp/protocol/nla"
	"github.com/18121861183/grdp/protocol/pdu"
	"github.com/18121861183/grdp/protocol/sec"
	"github.com/18121861183/grdp/protocol/t125"
	"github.com/18121861183/grdp/protocol/tpkt"
	"github.com/18121861183/grdp/protocol/x224"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Host string // ip:port
	tpkt *tpkt.TPKT
	x224 *x224.X224
	mcs  *t125.MCSClient
	Sec  *sec.Client
	pdu  *pdu.Client
}

func NewClient(host string, logLevel glog.LEVEL) *Client {
	glog.SetLevel(logLevel)
	logger := log.New(os.Stdout, "", 0)
	glog.SetLogger(logger)
	return &Client{
		Host: host,
	}
}

func (g *Client) Login(user, pwd string) error {
	conn, err := net.DialTimeout("tcp", g.Host, 3*time.Second)
	if err != nil {
		return errors.New(fmt.Sprintf("[dial err] %v", err))
	}
	defer conn.Close()

	domain := strings.Split(g.Host, ":")[0]

	g.tpkt = tpkt.New(core.NewSocketLayer(conn, nla.NewNTLMv2(domain, user, pwd)))
	g.x224 = x224.New(g.tpkt)
	g.mcs = t125.NewMCSClient(g.x224)
	g.Sec = sec.NewClient(g.mcs)
	g.pdu = pdu.NewClient(g.Sec)

	g.Sec.SetUser(user)
	g.Sec.SetPwd(pwd)
	g.Sec.SetDomain(domain)

	g.tpkt.SetFastPathListener(g.pdu)
	g.pdu.SetFastPathSender(g.tpkt)

	g.x224.SetRequestedProtocol(x224.PROTOCOL_SSL)

	err = g.x224.Connect()
	if err != nil {
		return errors.New(fmt.Sprintf("[x224 connect err] %v", err))
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	g.pdu.On("error", func(e error) {
		err = e
		glog.Error(e)
		wg.Done()
	}).On("close", func() {
		err = errors.New("close")
		glog.Info("on close")
		wg.Done()
	}).On("success", func() {
		err = nil
		glog.Info("on success")
		wg.Done()
	}).On("ready", func() {
		glog.Info("on ready")
	}).On("update", func(rectangles []pdu.BitmapData) {
		glog.Info("on update")
	})

	wg.Wait()
	return err
}
