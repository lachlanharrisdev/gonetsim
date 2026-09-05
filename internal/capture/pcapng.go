package capture

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
)

func ReadPcap(pcapFile string) error {
	f, err := os.Open(pcapFile)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 4)
	_, err = f.ReadAt(buf, 0)
	if err != nil {
		return err
	}

	if binary.BigEndian.Uint32(buf) == 0x0a0d0d0a {
		// pcapng
		return ReadPcapNG(f)
	}
	magic := binary.BigEndian.Uint32(buf)
	littleMagic := binary.LittleEndian.Uint32(buf)

	if magic == 0xa1b2c3d4 || magic == 0xa1b23c4d ||
		littleMagic == 0xa1b2c3d4 || littleMagic == 0xa1b23c4d {
		//pcap
		return ReadLegacyPcap(pcapFile)
	}

	return fmt.Errorf("unknown pcap header")
}

func ReadPcapNG(f *os.File) error {
	reader, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		return err
	}

	packetSource := gopacket.NewPacketSource(reader, reader.LinkType())
	for packet := range packetSource.Packets() {
		fmt.Println(packet)
	}

	return nil
}

func ReadLegacyPcap(f string) error {
	handle, err := pcap.OpenOffline(f)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	// Loop through packets in file
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		fmt.Println(packet)
	}

	return nil
}
