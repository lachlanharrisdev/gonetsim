package capture

import (
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	snapLen = 262144
)

type Session struct {
	run    *Run
	iface  int
	netw   string // "tcp" or "udp"
	local  netip.AddrPort
	remote netip.AddrPort

	synSent bool
	pending string

	clientSeq uint32
	serverSeq uint32
}

func (s *Session) Comment(text string) {
	if s == nil || s.run == nil {
		return
	}
	s.run.mu.Lock()
	defer s.run.mu.Unlock()
	s.pending = text
}

func (s *Session) Write(data []byte, fromClient bool) error {
	if s == nil || s.run == nil {
		return nil
	}
	s.run.mu.Lock()
	defer s.run.mu.Unlock()
	if s.netw == "udp" {
		return s.writeUDP(data, fromClient)
	}
	return s.writeTCP(data, fromClient)
}

func (s *Session) Close() error {
	if s == nil || s.run == nil {
		return nil
	}
	s.run.mu.Lock()
	defer s.run.mu.Unlock()
	if s.netw == "tcp" && s.synSent {
		if err := s.emitTCP(true, false, true, true, s.clientSeq, s.serverSeq); err != nil {
			return err
		}
		s.clientSeq++
		if err := s.emitTCP(false, false, true, true, s.serverSeq, s.clientSeq); err != nil {
			return err
		}
	}
	s.synSent = false
	return nil
}

func (s *Session) Flush() error {
	if s == nil || s.run == nil {
		return nil
	}
	s.run.mu.Lock()
	defer s.run.mu.Unlock()
	return s.run.f.Sync()
}

func encodeOption(code uint16, value []byte) []byte {
	out := make([]byte, 4+len(value))
	binary.LittleEndian.PutUint16(out[0:2], code)
	binary.LittleEndian.PutUint16(out[2:4], uint16(len(value)))
	copy(out[4:], value)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}

func (s *Session) writeUDP(data []byte, fromClient bool) error {
	src, dst := s.endpoints(fromClient)
	_, err := s.epb(s.build(data, src, dst, isUDP))
	return err
}

func (s *Session) writeTCP(data []byte, fromClient bool) error {
	if !s.synSent {
		if _, err := s.epb(s.buildTCPControl(true, true, false, false, s.clientSeq, s.serverSeq)); err != nil {
			return err
		}
		s.synSent = true
		s.clientSeq++
	}
	if s.serverSeq == 0 {
		if _, err := s.epb(s.buildTCPControl(false, true, true, false, s.serverSeq, s.clientSeq)); err != nil {
			return err
		}
		s.serverSeq++
	}

	seq, ack := s.clientSeq, s.serverSeq
	if !fromClient {
		seq, ack = s.serverSeq, s.clientSeq
	}
	_, err := s.epb(s.buildTCPData(data, fromClient, seq, ack))
	if fromClient {
		s.clientSeq += uint32(len(data))
	} else {
		s.serverSeq += uint32(len(data))
	}
	return err

}

func (s *Session) emitTCP(fromClient, syn, ackFlag, fin bool, seq, ackNum uint32) error {
	_, err := s.epb(s.buildTCPControl(fromClient, syn, ackFlag, fin, seq, ackNum))
	return err
}

func (s *Session) epb(frame []byte) (int, error) {
	ts := time.Now()
	opts := s.takeComment()
	length := 32 + frameLen(frame) + len(opts)
	b := make([]byte, 28)
	binary.LittleEndian.PutUint32(b[0:4], 6)
	binary.LittleEndian.PutUint32(b[4:8], uint32(length))
	binary.LittleEndian.PutUint32(b[8:12], uint32(s.iface))
	binary.LittleEndian.PutUint32(b[12:16], uint32(ts.UnixMicro()>>32))
	binary.LittleEndian.PutUint32(b[16:20], uint32(ts.UnixMicro()))
	binary.LittleEndian.PutUint32(b[20:24], uint32(len(frame)))
	binary.LittleEndian.PutUint32(b[24:28], uint32(len(frame)))
	if err := writeAll(s.run.f, b); err != nil {
		return 0, err
	}
	if err := writeAll(s.run.f, frame); err != nil {
		return 0, err
	}
	if pad := framePad(len(frame)); pad > 0 {
		if err := writeAll(s.run.f, make([]byte, pad)); err != nil {
			return 0, err
		}
	}
	if err := writeAll(s.run.f, opts); err != nil {
		return 0, err
	}
	if err := s.run.writeTrailerLocked(length); err != nil {
		return 0, err
	}
	s.run.packets++
	if s.run.packets == 1 {
		s.run.first = ts
	}
	s.run.last = ts
	return len(frame), nil
}

func (s *Session) takeComment() []byte {
	if s.pending == "" {
		return nil
	}
	out := encodeOption(1, []byte(s.pending)) // opt_comment, no nul
	s.pending = ""
	return out
}

func frameLen(b []byte) int {
	return len(b) + framePad(len(b))
}

func framePad(n int) int {
	return (4 - n%4) % 4
}

func writeAll(f *os.File, b []byte) error {
	_, err := f.Write(b)
	return err
}

type transportKind int

const (
	isUDP transportKind = iota
	isTCP
)

func (s *Session) build(data []byte, src, dst netip.AddrPort, kind transportKind) []byte {
	var network, transport gopacket.SerializableLayer

	switch kind {
	case isTCP:
		tcp := &layers.TCP{SrcPort: layers.TCPPort(src.Port()), DstPort: layers.TCPPort(dst.Port())}
		network = tcpIPLayer(src, dst, layers.IPProtocolTCP, tcp)
		transport = tcp
	default:
		udp := &layers.UDP{SrcPort: layers.UDPPort(src.Port()), DstPort: layers.UDPPort(dst.Port())}
		network = udpIPLayer(src, dst, udp)
		transport = udp
	}
	return s.serialize(data, src, network, transport)
}

func (s *Session) buildTCPData(data []byte, fromClient bool, seq, ack uint32) []byte {
	src, dst := s.endpoints(fromClient)
	tcp := newTCPLayer(src, dst, seq, ack, false, true, false, len(data) > 0)
	return s.buildWithTCP(data, src, dst, tcp)
}

func (s *Session) buildTCPControl(fromClient, syn, ackFlag, fin bool, seq, ackNum uint32) []byte {
	src, dst := s.endpoints(fromClient)
	tcp := newTCPLayer(src, dst, seq, ackNum, syn, ackFlag, fin, false)
	return s.buildWithTCP(nil, src, dst, tcp)
}

func newTCPLayer(src, dst netip.AddrPort, seq, ack uint32, syn, ackFlag, fin, psh bool) *layers.TCP {
	return &layers.TCP{
		SrcPort: layers.TCPPort(src.Port()),
		DstPort: layers.TCPPort(dst.Port()),
		Seq:     seq,
		Ack:     ack,
		SYN:     syn,
		ACK:     ackFlag,
		FIN:     fin,
		PSH:     psh,
		Window:  65535,
	}
}

func (s *Session) buildWithTCP(data []byte, src, dst netip.AddrPort, tcp *layers.TCP) []byte {
	return s.serialize(data, src, tcpIPLayer(src, dst, layers.IPProtocolTCP, tcp), tcp)
}

func tcpIPLayer(src, dst netip.AddrPort, proto layers.IPProtocol, tcp *layers.TCP) gopacket.SerializableLayer {
	if src.Addr().Is4() {
		ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: proto, SrcIP: src.Addr().AsSlice(), DstIP: dst.Addr().AsSlice()}
		_ = tcp.SetNetworkLayerForChecksum(ip)
		return ip
	}
	ip := &layers.IPv6{Version: 6, HopLimit: 64, NextHeader: proto, SrcIP: src.Addr().AsSlice(), DstIP: dst.Addr().AsSlice()}
	_ = tcp.SetNetworkLayerForChecksum(ip)
	return ip
}

func udpIPLayer(src, dst netip.AddrPort, udp *layers.UDP) gopacket.SerializableLayer {
	if src.Addr().Is4() {
		ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolUDP, SrcIP: src.Addr().AsSlice(), DstIP: dst.Addr().AsSlice()}
		_ = udp.SetNetworkLayerForChecksum(ip)
		return ip
	}
	ip := &layers.IPv6{Version: 6, HopLimit: 64, NextHeader: layers.IPProtocolUDP, SrcIP: src.Addr().AsSlice(), DstIP: dst.Addr().AsSlice()}
	_ = udp.SetNetworkLayerForChecksum(ip)
	return ip
}

func (s *Session) serialize(data []byte, src netip.AddrPort, network, transport gopacket.SerializableLayer) []byte {
	eth := s.ethLayer(src)
	buf := gopacket.NewSerializeBuffer()
	layersToWrite := []gopacket.SerializableLayer{&eth, network, transport}
	if len(data) > 0 {
		layersToWrite = append(layersToWrite, gopacket.Payload(data))
	}
	if err := gopacket.SerializeLayers(buf, serializeOpts, layersToWrite...); err != nil {
		return nil
	}
	return buf.Bytes()
}

func (s *Session) ethLayer(src netip.AddrPort) layers.Ethernet {
	var etherType = layers.EthernetTypeIPv4
	if !src.Addr().Is4() {
		etherType = layers.EthernetTypeIPv6
	}
	eth := layers.Ethernet{SrcMAC: clientMAC, DstMAC: serverMAC, EthernetType: etherType}
	if src.Addr() == s.local.Addr() {
		eth.SrcMAC = serverMAC
		eth.DstMAC = clientMAC
	}
	return eth
}

func (s *Session) endpoints(fromClient bool) (netip.AddrPort, netip.AddrPort) {
	if fromClient {
		return s.remote, s.local
	}
	return s.local, s.remote
}

var (
	serializeOpts = gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	clientMAC     = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	serverMAC     = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x02}
)
