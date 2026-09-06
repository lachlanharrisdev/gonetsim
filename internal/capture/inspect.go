package capture

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

type FileInfo struct {
	LinkType   layers.LinkType
	Packets    uint64
	First      time.Time
	Last       time.Time
	CreatedBy  string
	Interfaces []string
}

// true if b holds the magic bytes of a legacy pcap
// for either byte order and either timestamp resolution
func isLegacyMagic(b []byte) bool {
	return b[0] == 0xa1 && b[1] == 0xb2 && b[2] == 0xc3 && b[3] == 0xd4 ||
		b[0] == 0xa1 && b[1] == 0xb2 && b[2] == 0x3c && b[3] == 0x4d ||
		b[3] == 0xa1 && b[2] == 0xb2 && b[1] == 0xc3 && b[0] == 0xd4 ||
		b[3] == 0xa1 && b[2] == 0xb2 && b[1] == 0x3c && b[0] == 0x4d
}

func isHeaderOnly(f *os.File) (bool, error) {
	st, err := f.Stat()
	if err != nil {
		return false, err
	}
	var hdr [8]byte
	if _, err := f.ReadAt(hdr[:], 0); err != nil {
		return false, err
	}
	blockType := binary.LittleEndian.Uint32(hdr[0:4])
	blockLen := int64(binary.LittleEndian.Uint32(hdr[4:8]))
	return blockType == 0x0A0D0D0A && blockLen == st.Size(), nil
}

func Inspect(path string) (FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileInfo{}, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return FileInfo{}, fmt.Errorf("%q is not a pcapng file: %w", path, err)
	}
	if isLegacyMagic(magic[:]) {
		return FileInfo{}, fmt.Errorf("%q is a legacy pcap file; pcapng is the only supported format", path)
	}
	if magic != [4]byte{0x0a, 0x0d, 0x0d, 0x0a} {
		return FileInfo{}, fmt.Errorf("%q is not a pcapng file", path)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return FileInfo{}, fmt.Errorf("seek %q: %w", path, err)
	}
	nr, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		if empty, serr := isHeaderOnly(f); serr == nil && empty {
			return FileInfo{LinkType: layers.LinkTypeEthernet}, nil
		}
		return FileInfo{}, fmt.Errorf("read pcapng %q: %w", path, err)
	}

	info := FileInfo{LinkType: nr.LinkType(), CreatedBy: nr.SectionInfo().Application}
	for {
		_, ci, err := nr.ReadPacketData()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return FileInfo{}, fmt.Errorf("read packet %d from %q: %w", info.Packets+1, path, err)
		}
		if info.Packets == 0 {
			info.First = ci.Timestamp
		}
		info.Last = ci.Timestamp
		info.Packets++
	}
	for i := 0; i < nr.NInterfaces(); i++ {
		iface, err := nr.Interface(i)
		if err != nil {
			break
		}
		info.Interfaces = append(info.Interfaces, strings.TrimRight(iface.Name, "\x00"))
	}
	return info, nil
}
