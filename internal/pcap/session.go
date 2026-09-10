package pcap

import (
	"encoding/binary"
	"net"
	"net/netip"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const snapLen = 262144

const (
	// Every packet reserves a fixed-size comment slot so a capture:comment
	// can be retroactively written into the most recent packet without
	// shifting the append-only file layout.
	maxCommentLen  = 256
	commentSlotLen = 4 + maxCommentLen + 4 // opt_comment TLV + endofopt
)

// Session records the frames for one logical connection or UDP flow.
type Session struct {
	run    *Run
	iface  int
	netw   string // "tcp" or "udp"
	local  netip.AddrPort
	remote netip.AddrPort

	// the pcapng holds no real handshake, so the first Write emits SYN,
	// SYN-ACK, then data with synthetic seq/ack numbers tracked in
	// clientSeq/serverSeq; Close emits FIN/FIN-ACK.
	synSent bool
	pending string // comment waiting for a packet to attach to

	// lastOptions is the file offset of the comment slot in the most recent
	// packet this session wrote; 0 when no packet has been written yet.
	lastOptions int64

	clientSeq uint32
	serverSeq uint32
}

// Comment attaches text to the most recent packet this session emitted (the
// "current" packet, e.g. the line the handler just read). If no packet has
// been written yet it attaches to the next one. A comment that arrives before
// any packet, or when a previous comment already claimed the slot, waits for
// the next packet.
func (s *Session) Comment(text string) {
	if s == nil || s.run == nil {
		return
	}
	s.run.mu.Lock()
	defer s.run.mu.Unlock()
	if s.lastOptions == 0 {
		s.pending = text
		return
	}
	if _, err := s.run.f.WriteAt(commentOption(text), s.lastOptions); err != nil {
		// can't rewrite; fall back to the next packet
		s.pending = text
	}
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
	start := s.run.off
	length := 32 + frameLen(frame) + commentSlotLen
	b := make([]byte, 28)
	binary.LittleEndian.PutUint32(b[0:4], 6)
	binary.LittleEndian.PutUint32(b[4:8], uint32(length))
	binary.LittleEndian.PutUint32(b[8:12], uint32(s.iface))
	binary.LittleEndian.PutUint32(b[12:16], uint32(ts.UnixMicro()>>32))
	binary.LittleEndian.PutUint32(b[16:20], uint32(ts.UnixMicro()))
	binary.LittleEndian.PutUint32(b[20:24], uint32(len(frame)))
	binary.LittleEndian.PutUint32(b[24:28], uint32(len(frame)))
	if err := s.run.write(b); err != nil {
		return 0, err
	}
	if err := s.run.write(frame); err != nil {
		return 0, err
	}
	if pad := framePad(len(frame)); pad > 0 {
		if err := s.run.write(make([]byte, pad)); err != nil {
			return 0, err
		}
	}

	// the comment slot: filled from pending now, or retroactively later.
	s.lastOptions = start + int64(28+frameLen(frame))
	opts := make([]byte, commentSlotLen)
	if s.pending != "" {
		copy(opts, commentOption(s.pending))
		s.pending = ""
	}
	if err := s.run.write(opts); err != nil {
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

// commentOption encodes a comment slot: an opt_comment option (truncated to
// maxCommentLen), an end-of-options marker, and zero padding out to the fixed
// slot size so retroactive writes never change the block length.
func commentOption(text string) []byte {
	if len(text) > maxCommentLen {
		text = text[:maxCommentLen]
	}
	out := encodeOption(1, []byte(text)) // opt_comment
	out = append(out, encodeOption(0, nil)...)
	for len(out) < commentSlotLen {
		out = append(out, 0)
	}
	return out
}

func frameLen(b []byte) int {
	return len(b) + framePad(len(b))
}

func framePad(n int) int {
	return (4 - n%4) % 4
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
		network = ipLayer(src, dst, layers.IPProtocolTCP, tcp)
		transport = tcp
	default:
		udp := &layers.UDP{SrcPort: layers.UDPPort(src.Port()), DstPort: layers.UDPPort(dst.Port())}
		network = ipLayer(src, dst, layers.IPProtocolUDP, udp)
		transport = udp
	}
	return s.serialize(data, src, network, transport)
}

func (s *Session) buildTCPData(data []byte, fromClient bool, seq, ack uint32) []byte {
	return s.buildTCP(data, fromClient, false, true, false, len(data) > 0, seq, ack)
}

func (s *Session) buildTCPControl(fromClient, syn, ackFlag, fin bool, seq, ackNum uint32) []byte {
	return s.buildTCP(nil, fromClient, syn, ackFlag, fin, false, seq, ackNum)
}

func (s *Session) buildTCP(data []byte, fromClient, syn, ackFlag, fin, psh bool, seq, ack uint32) []byte {
	src, dst := s.endpoints(fromClient)
	tcp := newTCPLayer(src, dst, seq, ack, syn, ackFlag, fin, psh)
	return s.serialize(data, src, ipLayer(src, dst, layers.IPProtocolTCP, tcp), tcp)
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

func ipLayer(src, dst netip.AddrPort, proto layers.IPProtocol, transport gopacket.SerializableLayer) gopacket.SerializableLayer {
	setChecksum := func(ip gopacket.NetworkLayer) {
		switch t := transport.(type) {
		case *layers.TCP:
			_ = t.SetNetworkLayerForChecksum(ip)
		case *layers.UDP:
			_ = t.SetNetworkLayerForChecksum(ip)
		}
	}
	if src.Addr().Is4() {
		ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: proto, SrcIP: src.Addr().AsSlice(), DstIP: dst.Addr().AsSlice()}
		setChecksum(ip)
		return ip
	}
	ip := &layers.IPv6{Version: 6, HopLimit: 64, NextHeader: proto, SrcIP: src.Addr().AsSlice(), DstIP: dst.Addr().AsSlice()}
	setChecksum(ip)
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
