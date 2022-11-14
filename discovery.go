package discovery

import (
	"fmt"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"net"
	"strconv"
	"sync"
	"time"
)

type IPVersion uint

const (
	IPv4 IPVersion = 4
	IPv6 IPVersion = 6
)

type Options struct {
	// Limit is the number of to discover (default 1)
	Limit int
	// TimeLimit is the duration of to discover (default 10s)
	TimeLimit time.Duration
	// IPVersion specifies the version of the Internet Protocol (default IPv4)
	IPVersion IPVersion
	// Duration broadcast duration. duration < 1 will always broadcast
	Duration time.Duration
	// BroadcastDelay is time interval between broadcasts. The default delay is 1 second.
	BroadcastDelay time.Duration
	// Payload is the bytes that are sent out with each broadcast. Must be short.
	Payload []byte
	// Port is the port to broadcast on. default port is 9081
	Port string
	// MulticastAddress specifies the multicast address.
	// You should be able to use any of 224.0.0.0/4 or ff00::/8.
	// default address (239.255.255.250 for IPv4 or ff02::c for IPv6).
	MulticastAddress string

	payloadLen int
}

type NetPacketConn interface {
	JoinGroup(ifi *net.Interface, group net.Addr) error
	SetMulticastInterface(ini *net.Interface) error
	SetMulticastTTL(int) error
	ReadFrom(buf []byte) (int, net.Addr, error)
	WriteTo(buf []byte, dst net.Addr) (int, error)
}

type IPv4PacketConn struct {
	*ipv4.PacketConn
}

func (ip4 IPv4PacketConn) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, _, addr, err := ip4.PacketConn.ReadFrom(buf)
	return n, addr, err
}

func (ip4 IPv4PacketConn) WriteTo(buf []byte, dst net.Addr) (int, error) {
	return ip4.PacketConn.WriteTo(buf, nil, dst)
}

type IPv6PacketConn struct {
	*ipv6.PacketConn
}

func (ip6 IPv6PacketConn) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, _, addr, err := ip6.PacketConn.ReadFrom(buf)
	return n, addr, err
}

func (ip6 IPv6PacketConn) WriteTo(buf []byte, dst net.Addr) (int, error) {
	return ip6.PacketConn.WriteTo(buf, nil, dst)
}

func (ip6 IPv6PacketConn) SetMulticastTTL(i int) error {
	return ip6.SetMulticastHopLimit(i)
}

type Broadcast struct {
	Options *Options
	quit    chan bool
}

type Discover struct {
	sync.RWMutex
	Options  *Options
	received map[string]byte
	done     chan bool
}

type Discovered struct {
	Address string
}

// initOptions set default options
func initOptions(options *Options) {
	if options.IPVersion == 0 {
		options.IPVersion = IPv4
	}
	if options.Limit == 0 {
		options.Limit = 1
	}
	if options.TimeLimit == 0 {
		options.TimeLimit = time.Second * 10
	}
	if options.Duration == 0 {
		options.Duration = -1 * time.Second
	}
	if options.BroadcastDelay == 0 {
		options.BroadcastDelay = time.Second
	}
	if options.Payload == nil {
		options.Payload = []byte("hi")
	}
	if options.Port == "" {
		options.Port = "9081"
	}
	if options.MulticastAddress == "" {
		if options.IPVersion == IPv4 {
			options.MulticastAddress = "239.255.255.250"
		} else {
			options.MulticastAddress = "ff02::c"
		}
	}

	options.payloadLen = len(options.Payload)
}

func (b *Broadcast) StartBroadcast() error {
	initOptions(b.Options)

	ifaces, err := FilterInterfaces(b.Options.IPVersion == IPv4)
	if err != nil {
		return err
	}
	if len(ifaces) == 0 {
		return fmt.Errorf("no multicast interface found")
	}

	address := net.JoinHostPort(b.Options.MulticastAddress, b.Options.Port)

	c, err := net.ListenPacket(fmt.Sprintf("udp%d", b.Options.IPVersion), address)
	if err != nil {
		return err
	}
	defer c.Close()

	group := net.ParseIP(b.Options.MulticastAddress)
	port, err := strconv.Atoi(b.Options.Port)
	if err != nil {
		return err
	}
	var npc NetPacketConn
	if b.Options.IPVersion == IPv4 {
		npc = IPv4PacketConn{ipv4.NewPacketConn(c)}
	} else {
		npc = IPv6PacketConn{ipv6.NewPacketConn(c)}
	}

	for i := range ifaces {
		err := npc.JoinGroup(ifaces[i], &net.UDPAddr{IP: group, Port: port})
		if err != nil {
			fmt.Println(err)
		}
	}

	ticker := time.NewTicker(b.Options.BroadcastDelay)

	if b.Options.Duration > 0 {
		go func() {
			time.AfterFunc(b.Options.Duration, func() {
				b.quit <- true
			})
		}()
	}
LOOP:
	for {
		select {
		case <-b.quit:
			break LOOP
		case <-ticker.C:
			for i := range ifaces {
				if errMulticast := npc.SetMulticastInterface(ifaces[i]); errMulticast != nil {
					continue
				}
				_ = npc.SetMulticastTTL(2)
				if _, errMulticast := npc.WriteTo(b.Options.Payload, &net.UDPAddr{IP: group, Port: port}); errMulticast != nil {
					continue
				}
			}
		}
	}
	return nil
}

func (b *Broadcast) StartAsSync() {
	go b.StartBroadcast()
}

func (b *Broadcast) StopBroadcast() {
	b.quit <- true
}

func (d *Discover) DiscoverBroadcast() ([]*Discovered, error) {
	initOptions(d.Options)

	ds := make([]*Discovered, 0)

	err := d.receive()
	if err != nil {
		return nil, err
	}

	for host := range d.received {
		ds = append(ds, &Discovered{host})
	}

	return ds, nil
}

func (d *Discover) receive() error {
	ifaces, err := FilterInterfaces(d.Options.IPVersion == IPv4)
	if err != nil {
		return err
	}
	if len(ifaces) == 0 {
		fmt.Println("no multicast interface found")
		return err
	}

	address := net.JoinHostPort(d.Options.MulticastAddress, d.Options.Port)

	c, err := net.ListenPacket(fmt.Sprintf("udp%d", d.Options.IPVersion), address)
	if err != nil {
		return err
	}
	defer c.Close()

	group := net.ParseIP(d.Options.MulticastAddress)
	port, err := strconv.Atoi(d.Options.Port)
	if err != nil {
		return err
	}

	var npc NetPacketConn
	if d.Options.IPVersion == IPv4 {
		npc = IPv4PacketConn{ipv4.NewPacketConn(c)}
	} else {
		npc = IPv6PacketConn{ipv6.NewPacketConn(c)}
	}
	for i := range ifaces {
		err := npc.JoinGroup(ifaces[i], &net.UDPAddr{IP: group, Port: port})
		if err != nil {
			//return  nil, err
		}
	}

	time.AfterFunc(d.Options.TimeLimit, func() {
		d.done <- true
	})

	go func() {
		var buf [66507]byte
		n, src, err := npc.ReadFrom(buf[:])
		if err != nil {
			fmt.Println(err)
		}
		if n > 0 && string(d.Options.Payload) == string(buf[:d.Options.payloadLen]) {
			srcHost, _, _ := net.SplitHostPort(src.String())
			d.Lock()
			if _, ok := d.received[srcHost]; !ok {
				d.received[srcHost] = byte('0')
			}
			d.Unlock()

			if d.Options.Limit > 0 {
				if d.Options.Limit == len(d.received) {
					d.done <- true
				}
			}
		}
	}()

LOOP:
	for {
		select {
		case <-d.done:
			break LOOP
		default:
		}
	}
	return err
}

func NewBroadcast(options *Options) *Broadcast {
	return &Broadcast{
		Options: options,
		quit:    make(chan bool),
	}
}

func NewDiscover(options *Options) *Discover {
	return &Discover{
		Options:  options,
		received: make(map[string]byte, 0),
		done:     make(chan bool, 1),
	}
}
