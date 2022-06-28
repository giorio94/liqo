package main

import (
	"encoding/binary"
	"net"
	"strconv"

	"github.com/coreos/go-iptables/iptables"
	"github.com/ti-mo/conntrack"
	"k8s.io/klog/v2"
)

type entry struct {
	source *net.UDPAddr
}

type funnel struct {
	localPort  uint16
	connection *net.UDPConn
	upstreams  []*net.UDPAddr

	iptHandler *iptables.IPTables
	ctHandler  *conntrack.Conn

	cache map[uint32]entry
}

func New(external *net.UDPAddr, upstreams []*net.UDPAddr) (*funnel, error) {
	iptHandler, err := iptables.New()
	if err != nil {
		klog.Errorf("Failed to initialize UDP listener: %v", err)
		return nil, err
	}

	if err := iptHandler.ClearChain("nat", "PREROUTING"); err != nil {
		klog.Errorf("Could not flush the PREROUTING iptables chain: %v", err)
		return nil, err
	}

	ctHandler, err := conntrack.Dial(nil)
	if err != nil {
		klog.Errorf("Could not open conntrack socket: %v", err)
		return nil, err
	}

	if err := ctHandler.Flush(); err != nil {
		klog.Errorf("Could not flush conntrack entries: %v", err)
		return nil, err
	}

	connection, err := net.ListenUDP("udp", external)
	if err != nil {
		klog.Errorf("Failed to initialize UDP listener: %v", err)
		return nil, err
	}

	return &funnel{
		localPort: uint16(external.Port), connection: connection, upstreams: upstreams,
		iptHandler: iptHandler, ctHandler: ctHandler,
		cache: map[uint32]entry{}}, nil
}

func (f *funnel) start() {
	klog.Infof("Server initialized")
	buffer := make([]byte, 10240)
	oob := make([]byte, 10240)

	for {
		read, _, _, source, err := f.connection.ReadMsgUDP(buffer, oob)
		if err != nil {
			klog.Errorf("Failed to read message: %v", err)
			continue
		}

		if read < 12 {
			klog.Errorf("Invalid input message (length: %v)", read)
			return
		}

		switch buffer[0] {
		case 0x01:
			klog.Infof("Received first handshake message from %v", source)
			f.first(source, buffer[:read])
		case 0x02:
			klog.Infof("Received second handshake message from %v", source)
			f.second(source, buffer[:read])
		default:
			klog.Errorf("Unmanaged message (type: %x) from %v", buffer[0], source)
		}
	}
}

func (f *funnel) first(source *net.UDPAddr, buffer []byte) {
	if f.isUpstream(source) {
		klog.Warningf("Skipping first handshake message, since source is upstream")
		return
	}

	id := binary.BigEndian.Uint32(buffer[4:8])
	f.cache[id] = entry{source: source}
	klog.Infof("Storing cache entry for %v: %x", source, id)

	for _, upstream := range f.upstreams {
		if _, _, err := f.connection.WriteMsgUDP(buffer, nil, upstream); err != nil {
			klog.Errorf("Failed to send message to %v: %v", upstream, err)
		} else {
			klog.Infof("Message successfully sent to %v", upstream)
		}
	}
}

func (f *funnel) second(upstream *net.UDPAddr, buffer []byte) {
	if !f.isUpstream(upstream) {
		klog.Warningf("Skipping second handshake message, since source is not upstream")
		return
	}

	id := binary.BigEndian.Uint32(buffer[8:12])
	entry, found := f.cache[id]
	if !found {
		klog.Errorf("No matching entry for sender ID %v", id)
		return
	}
	klog.Infof("Destination: %v, upstream: %v", entry.source, upstream)

	rules := []string{"--proto", "udp", "--source", entry.source.IP.String(),
		"--sport", strconv.Itoa(entry.source.Port),
		"--jump", "DNAT", "--to-destination", upstream.String()}
	if err := f.iptHandler.AppendUnique("nat", "PREROUTING", rules...); err != nil {
		klog.Errorf("Failed adding iptables rules %v", rules)
		return
	}
	klog.Infof("iptables rules configured")

	if _, _, err := f.connection.WriteMsgUDP(buffer, nil, entry.source); err != nil {
		klog.Errorf("Failed to send message to %v: %v", entry.source, err)
		return
	}
	klog.Infof("Message forwarded to source: %v", entry.source)

	dst := make(net.IP, len(entry.source.IP))
	copy(dst, entry.source.IP)
	dst[len(dst)-1] = 4

	flow := conntrack.NewFlow(
		17, 0, dst, entry.source.IP,
		8080, 8080, 0, 0,
	)

	err := f.ctHandler.Delete(flow)
	if err != nil {
		klog.Errorf("Failed to delete conntrack entry: %v", err)
		return
	}
	klog.Infof("Conntrack rule deleted")
}

func (f *funnel) isUpstream(addr *net.UDPAddr) bool {
	for _, upstream := range f.upstreams {
		if addr.String() == upstream.String() {
			return true
		}
	}
	return false
}
