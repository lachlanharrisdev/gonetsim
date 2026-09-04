package handler

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// string.pack / string.unpack: gopher-lua implements Lua 5.1, which predates
// these; this covers the practical 5.3 subset. Alignment ("!") is rejected
// rather than misencoded.

const (
	// maxPackSize caps the size of a single packed string.
	maxPackSize = 1 << 20 // 1 MiB

	// defaultIntSize is the size of "i"/"I"; defaultLenSize is the length
	// prefix size of plain "s", matching the 5.3 default of size_t.
	defaultIntSize = 4
	defaultLenSize = 8
)

type packItem struct {
	op     byte
	size   int
	endian binary.ByteOrder
}

func parsePackFormat(f string) ([]packItem, error) {
	var items []packItem
	var endian binary.ByteOrder = binary.LittleEndian // the '=' default; native equals little here

	for i := 0; i < len(f); i++ {
		c := f[i]
		switch c {
		case ' ':
		case '<':
			endian = binary.LittleEndian
		case '>':
			endian = binary.BigEndian
		case '=':
			endian = binary.LittleEndian
		case '!':
			return nil, fmt.Errorf("alignment ('!') is not supported")
		case 'x':
			items = append(items, packItem{op: 'x', size: 1})
		case 'b', 'B':
			items = append(items, packItem{op: c, size: 1, endian: endian})
		case 'h', 'H':
			items = append(items, packItem{op: c, size: 2, endian: endian})
		case 'i', 'I':
			n, next, err := scanSize(f, i+1, defaultIntSize)
			if err != nil {
				return nil, err
			}
			if n < 1 || n > 8 {
				return nil, fmt.Errorf("integral size (%d) out of limits [1,8]", n)
			}
			items = append(items, packItem{op: c, size: n, endian: endian})
			i = next - 1
		case 'l', 'L', 'j', 'J':
			items = append(items, packItem{op: c, size: 8, endian: endian})
		case 'f':
			items = append(items, packItem{op: c, size: 4, endian: endian})
		case 'd', 'n':
			items = append(items, packItem{op: c, size: 8, endian: endian})
		case 's':
			n, next, err := scanSize(f, i+1, defaultLenSize)
			if err != nil {
				return nil, err
			}
			if n < 1 || n > 8 {
				return nil, fmt.Errorf("string length size (%d) out of limits [1,8]", n)
			}
			items = append(items, packItem{op: c, size: n, endian: endian})
			i = next - 1
		case 'z':
			items = append(items, packItem{op: c})
		case 'c':
			n, next, err := scanSize(f, i+1, 0)
			if err != nil {
				return nil, err
			}
			if next == i+1 {
				return nil, fmt.Errorf("missing size for format option 'c' in %q", f)
			}
			items = append(items, packItem{op: c, size: n})
			i = next - 1
		default:
			return nil, fmt.Errorf("invalid format option '%c' in %q", c, f)
		}
	}
	return items, nil
}

func scanSize(f string, i, def int) (int, int, error) {
	start := i
	for i < len(f) && f[i] >= '0' && f[i] <= '9' {
		i++
	}
	if i == start {
		return def, start, nil
	}
	n, err := strconv.Atoi(f[start:i])
	if err != nil || n > 1<<20 {
		return 0, 0, fmt.Errorf("size in format %q out of range", f)
	}
	return n, i, nil
}

func isUnsigned(op byte) bool {
	return op == 'B' || op == 'H' || op == 'I' || op == 'L' || op == 'J'
}

// checkInteger errors on non-integral numbers; gopher-lua's CheckInt64
// would silently truncate and corrupt packed data.
func checkInteger(L *lua.LState, arg int) int64 {
	v := float64(L.CheckNumber(arg))
	if v != math.Trunc(v) || math.IsNaN(v) || math.IsInf(v, 0) ||
		v >= 9223372036854775808.0 || v < -9223372036854775808.0 {
		L.ArgError(arg, "number has no integer representation")
		return 0
	}
	return int64(v)
}

// checkIntRange enforces the 5.3 overflow rules.
func checkIntRange(op byte, size int, v int64) error {
	if size == 8 {
		return nil
	}
	if isUnsigned(op) {
		if v < 0 || uint64(v) > uint64(1)<<(8*size)-1 {
			return fmt.Errorf("integer overflow")
		}
		return nil
	}
	min := -(int64(1) << (8*size - 1))
	max := (int64(1) << (8*size - 1)) - 1
	if v < min || v > max {
		return fmt.Errorf("integer overflow")
	}
	return nil
}

func putInt(dst []byte, order binary.ByteOrder, v uint64, n int) {
	var buf [8]byte
	order.PutUint64(buf[:], v)
	if order == binary.BigEndian {
		copy(dst, buf[8-n:])
	} else {
		copy(dst, buf[:n])
	}
}

func getInt(src []byte, order binary.ByteOrder, n int) uint64 {
	var buf [8]byte
	if order == binary.BigEndian {
		copy(buf[8-n:], src)
	} else {
		copy(buf[:n], src)
	}
	return order.Uint64(buf[:])
}

func luaPack(L *lua.LState) int {
	f := L.CheckString(1)
	items, err := parsePackFormat(f)
	if err != nil {
		L.ArgError(1, err.Error())
		return 0
	}

	var buf []byte
	arg := 2
	for _, it := range items {
		switch it.op {
		case 'x':
			buf = append(buf, 0)
		case 'b', 'h', 'i', 'l', 'j', 'B', 'H', 'I', 'L', 'J':
			v := checkInteger(L, arg)
			if err := checkIntRange(it.op, it.size, v); err != nil {
				L.ArgError(arg, err.Error())
				return 0
			}
			off := len(buf)
			buf = append(buf, make([]byte, it.size)...)
			putInt(buf[off:], it.endian, uint64(v), it.size)
		case 'f':
			bits := math.Float32bits(float32(L.CheckNumber(arg)))
			off := len(buf)
			buf = append(buf, make([]byte, 4)...)
			it.endian.PutUint32(buf[off:], bits)
		case 'd', 'n':
			bits := math.Float64bits(float64(L.CheckNumber(arg)))
			off := len(buf)
			buf = append(buf, make([]byte, 8)...)
			it.endian.PutUint64(buf[off:], bits)
		case 's':
			s := L.CheckString(arg)
			if it.size < 8 && int64(len(s)) >= int64(1)<<(8*it.size) {
				L.ArgError(arg, "string longer than size")
				return 0
			}
			off := len(buf)
			buf = append(buf, make([]byte, it.size)...)
			putInt(buf[off:], it.endian, uint64(len(s)), it.size)
			buf = append(buf, s...)
		case 'z':
			s := L.CheckString(arg)
			if strings.IndexByte(s, 0) >= 0 {
				L.ArgError(arg, "string contains zeros")
				return 0
			}
			buf = append(buf, s...)
			buf = append(buf, 0)
		case 'c':
			s := L.CheckString(arg)
			if len(s) > it.size {
				L.ArgError(arg, "string longer than given size")
				return 0
			}
			buf = append(buf, s...)
			buf = append(buf, make([]byte, it.size-len(s))...)
		}
		arg++
	}

	if len(buf) > maxPackSize {
		L.ArgError(1, fmt.Sprintf("resulting string larger than %d bytes", maxPackSize))
		return 0
	}
	L.Push(lua.LString(buf))
	return 1
}

func luaUnpack(L *lua.LState) int {
	f := L.CheckString(1)
	data := L.CheckString(2)
	items, err := parsePackFormat(f)
	if err != nil {
		L.ArgError(1, err.Error())
		return 0
	}

	init := 1
	if L.GetTop() >= 3 {
		init = L.CheckInt(3)
	}
	if init < 0 {
		init = len(data) + init + 1
	}
	if init < 1 || init > len(data)+1 {
		L.ArgError(3, "initial position out of string")
		return 0
	}
	off := init - 1

	var results []lua.LValue
	for _, it := range items {
		if off+it.size > len(data) {
			L.RaiseError("data string too short")
			return 0
		}

		switch it.op {
		case 'x':
			off++
		case 'b', 'h', 'i', 'l', 'j':
			v := int64(getInt([]byte(data[off:off+it.size]), it.endian, it.size))
			results = append(results, lua.LNumber(signExtend(v, it.size)))
			off += it.size
		case 'B', 'H', 'I', 'L', 'J':
			results = append(results, lua.LNumber(getInt([]byte(data[off:off+it.size]), it.endian, it.size)))
			off += it.size
		case 'f':
			bits := it.endian.Uint32([]byte(data[off : off+4]))
			results = append(results, lua.LNumber(math.Float32frombits(bits)))
			off += 4
		case 'd', 'n':
			bits := it.endian.Uint64([]byte(data[off : off+8]))
			results = append(results, lua.LNumber(math.Float64frombits(bits)))
			off += 8
		case 's':
			n := int(getInt([]byte(data[off:off+it.size]), it.endian, it.size))
			off += it.size
			if n < 0 || off+n > len(data) {
				L.RaiseError("data string too short")
				return 0
			}
			results = append(results, lua.LString(data[off:off+n]))
			off += n
		case 'z':
			end := strings.IndexByte(data[off:], 0)
			if end < 0 {
				L.RaiseError("no zero terminator found")
				return 0
			}
			results = append(results, lua.LString(data[off:off+end]))
			off += end + 1
		case 'c':
			results = append(results, lua.LString(data[off:off+it.size]))
			off += it.size
		}
	}

	for _, v := range results {
		L.Push(v)
	}
	L.Push(lua.LNumber(off + 1))
	return len(results) + 1
}

func signExtend(v int64, size int) int64 {
	if size == 8 {
		return v
	}
	shift := 64 - 8*size
	return v << shift >> shift
}
