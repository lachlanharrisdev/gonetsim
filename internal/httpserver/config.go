package httpserver

import "errors"

type Config struct {
	Addr string

	// if non-empty, a fixed status code returned for all requests
	// when zero, defaults to 200
	StatusCode int
}

func (c Config) validate() error {
	if c.Addr == "" {
		return errors.New("http listen addr is required")
	}
	return nil
}
