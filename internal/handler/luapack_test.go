package handler

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// runLuaN evaluates src through the sandboxed libraries and returns its nret
// return values.
func runLuaN(t *testing.T, src string, nret int) []lua.LValue {
	t.Helper()
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	openLibs(L)
	proto, err := L.LoadString(src)
	if err != nil {
		t.Fatalf("load %q: %v", src, err)
	}
	if err := L.CallByParam(lua.P{Fn: proto, NRet: nret, Protect: true}); err != nil {
		t.Fatalf("lua %q: %v", src, err)
	}
	// results land at the top of the stack; gopher-lua leaves evaluation
	// leftovers beneath them
	vals := make([]lua.LValue, nret)
	for i := range vals {
		vals[i] = L.Get(-nret + i)
	}
	L.Pop(nret)
	return vals
}

func lvString(v lua.LValue) string { return string(v.(lua.LString)) }

func lvNumber(t *testing.T, v lua.LValue) float64 {
	t.Helper()
	n, ok := v.(lua.LNumber)
	if !ok {
		t.Fatalf("expected number, got %s (%v)", v.Type().String(), v)
	}
	return float64(n)
}

func TestLuaPack(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		nret  int
		check func(t *testing.T, vals []lua.LValue)
	}{
		{"big endian int", `return string.pack(">i4", 1000)`, 1, func(t *testing.T, v []lua.LValue) {
			if got := lvString(v[0]); got != "\x00\x00\x03\xe8" {
				t.Errorf("got %q", got)
			}
		}},
		{"little endian int", `return string.pack("i4", 1000)`, 1, func(t *testing.T, v []lua.LValue) {
			if got := lvString(v[0]); got != "\xe8\x03\x00\x00" {
				t.Errorf("got %q", got)
			}
		}},
		{"signed sizes", `return string.pack("<h", -2), string.pack("b", -1), string.pack("B", 255), string.pack(">H", 65535)`, 4, func(t *testing.T, v []lua.LValue) {
			want := []string{"\xfe\xff", "\xff", "\xff", "\xff\xff"}
			for i, w := range want {
				if got := lvString(v[i]); got != w {
					t.Errorf("case %d = %q, want %q", i, got, w)
				}
			}
		}},
		{"default sizes", `return #string.pack("i", 1), #string.pack("j", 1), #string.pack("f", 1), #string.pack("d", 1), #string.pack("s", "hi")`, 5, func(t *testing.T, v []lua.LValue) {
			want := []float64{4, 8, 4, 8, 10} // plain "s" prefixes an 8-byte length
			for i, w := range want {
				if got := lvNumber(t, v[i]); got != w {
					t.Errorf("size %d = %v, want %v", i, got, w)
				}
			}
		}},
		{"strings", `return string.pack("z", "hi"), string.pack("c4", "ab"), string.pack("<s2", "hi")`, 3, func(t *testing.T, v []lua.LValue) {
			want := []string{"hi\x00", "ab\x00\x00", "\x02\x00hi"}
			for i, w := range want {
				if got := lvString(v[i]); got != w {
					t.Errorf("case %d = %q, want %q", i, got, w)
				}
			}
		}},
		{"floats", `return string.pack(">d", 1.5)`, 1, func(t *testing.T, v []lua.LValue) {
			if got := lvString(v[0]); got != "\x3f\xf8\x00\x00\x00\x00\x00\x00" {
				t.Errorf("got %q", got)
			}
		}},
		{"unpack signed", `return string.unpack("b", "\255"), string.unpack("<i4", "\254\255\255\255")`, 3, func(t *testing.T, v []lua.LValue) {
			// only the last call in a return list expands to all its values
			if got := lvNumber(t, v[0]); got != -1 {
				t.Errorf("b = %v", got)
			}
			if got := lvNumber(t, v[1]); got != -2 {
				t.Errorf("i4 = %v", got)
			}
			if got := lvNumber(t, v[2]); got != 5 {
				t.Errorf("i4 position = %v", got)
			}
		}},
		{"unpack positions", `return string.unpack(">I2", "\1\2\3\4", 3)`, 2, func(t *testing.T, v []lua.LValue) {
			if got := lvNumber(t, v[0]); got != 0x0304 {
				t.Errorf("value = %v", got)
			}
			if got := lvNumber(t, v[1]); got != 5 {
				t.Errorf("position = %v", got)
			}
		}},
		{"unpack length-prefixed", `
			local d = string.pack(">s2", "payload")
			local s, pos = string.unpack(">s2", d)
			return s, pos, #d
		`, 3, func(t *testing.T, v []lua.LValue) {
			if got := lvString(v[0]); got != "payload" {
				t.Errorf("string = %q", got)
			}
			if got := lvNumber(t, v[1]); got != 10 {
				t.Errorf("position = %v, want 10", got)
			}
			if got := lvNumber(t, v[2]); got != 9 {
				t.Errorf("total = %v, want 9", got)
			}
		}},
		{"unpack float roundtrip", `local ok, err = pcall(function() return string.unpack(">f", string.pack(">f", 0.5)) end); return ok, err`, 2, func(t *testing.T, v []lua.LValue) {
			if v[0] != lua.LTrue {
				t.Fatalf("pcall failed: %v", v[1])
			}
			if got := lvNumber(t, v[1]); got != 0.5 {
				t.Errorf("float roundtrip = %v", got)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, runLuaN(t, tc.src, tc.nret))
		})
	}
}

func TestLuaPackErrors(t *testing.T) {
	cases := []struct {
		src    string
		wantIn string
	}{
		{`local ok, err = pcall(string.pack, "B", 256); return ok, err`, "integer overflow"},
		{`local ok, err = pcall(string.pack, "i2", 70000); return ok, err`, "integer overflow"},
		{`local ok, err = pcall(string.pack, "i", 1.5); return ok, err`, "no integer representation"},
		{`local ok, err = pcall(string.pack, "z", "a\0b"); return ok, err`, "string contains zeros"},
		{`local ok, err = pcall(string.pack, "c2", "abc"); return ok, err`, "string longer than given size"},
		{`local ok, err = pcall(string.pack, "c", "x"); return ok, err`, "missing size for format option 'c'"},
		{`local ok, err = pcall(string.pack, "!", "x"); return ok, err`, "not supported"},
		{`local ok, err = pcall(string.pack, "q", 1); return ok, err`, "invalid format option"},
		{`local ok, err = pcall(string.pack, "i9", 1); return ok, err`, "out of limits"},
		{`local ok, err = pcall(string.unpack, ">i4", "\1\2"); return ok, err`, "data string too short"},
		{`local ok, err = pcall(string.unpack, "z", "no terminator"); return ok, err`, "zero terminator"},
		{`local ok, err = pcall(string.unpack, ">s2", "\0\5ab"); return ok, err`, "data string too short"},
		{`local ok, err = pcall(string.unpack, "b", "\1", 5); return ok, err`, "initial position out of string"},
	}
	for _, tc := range cases {
		vals := runLuaN(t, tc.src, 2)
		if vals[0] != lua.LFalse {
			t.Errorf("%s: expected pcall failure, got %v", tc.src, vals[0])
		}
		if got := lvString(vals[1]); !strings.Contains(got, tc.wantIn) {
			t.Errorf("%s: error %q does not contain %q", tc.src, got, tc.wantIn)
		}
	}
}
