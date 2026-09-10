package script

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type tlsState interface {
	ConnectionState() tls.ConnectionState
	HandshakeContext(ctx context.Context) error
}

// luaConn adapts a net.Conn for the conn:* API: buffered reads with caps, and
// TLS introspection forwarded through any wrappers (idle deadline, capture).
// logger surfaces TLS handshake failures so the operator can see why conn:sni
// or conn:tls is returning nil.
type luaConn struct {
	net.Conn
	br     *bufio.Reader
	logger *slog.Logger
}

func newLuaConn(conn net.Conn, logger *slog.Logger) *luaConn {
	return &luaConn{Conn: conn, br: bufio.NewReader(conn), logger: logger}
}

func (lc *luaConn) tls() tlsState {
	tc, _ := lc.Conn.(tlsState)
	return tc
}

func (lc *luaConn) handshake(ctx context.Context) (tls.ConnectionState, bool) {
	tc := lc.tls()
	if tc == nil {
		return tls.ConnectionState{}, false
	}
	if !tc.ConnectionState().HandshakeComplete {
		if err := tc.HandshakeContext(ctx); err != nil && lc.logger != nil {
			lc.logger.Info("TLS handshake failed: " + err.Error())
		}
	}
	st := tc.ConnectionState()
	if st.Version == 0 {
		return tls.ConnectionState{}, false
	}
	return st, true
}

func (lc *luaConn) read(n int) (lua.LValue, error) {
	buf := make([]byte, n)
	nr, err := lc.br.Read(buf)
	if nr > 0 {
		return lua.LString(buf[:nr]), nil
	}
	if errors.Is(err, io.EOF) {
		return lua.LNil, nil
	}
	return nil, err
}

func (lc *luaConn) readLine() (lua.LValue, error) {
	var sb strings.Builder
	for {
		chunk, err := lc.br.ReadSlice('\n')
		sb.Write(chunk)
		if err == nil {
			return lua.LString(sb.String()), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			if sb.Len() > maxReadLen {
				return nil, fmt.Errorf("line exceeds %d bytes", maxReadLen)
			}
			continue
		}
		if errors.Is(err, io.EOF) {
			return eofValue(sb.String()), nil
		}
		return nil, err
	}
}

func (lc *luaConn) readUntil(delim []byte) (lua.LValue, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		if i := bytes.Index(buf, delim); i >= 0 {
			return lua.LString(buf[:i+len(delim)]), nil
		}
		if len(buf) > maxReadLen {
			return nil, fmt.Errorf("read exceeds %d bytes", maxReadLen)
		}
		n, err := lc.br.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return eofValue(string(buf)), nil
			}
			return nil, err
		}
	}
}

func eofValue(s string) lua.LValue {
	if s != "" {
		return lua.LString(s)
	}
	return lua.LNil
}
