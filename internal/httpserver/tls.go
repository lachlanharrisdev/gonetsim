package httpserver

import "crypto/tls"

func (s *Server) SetTLSConfig(conf *tls.Config) {
	s.http.TLSConfig = conf
}
