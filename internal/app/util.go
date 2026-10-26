package app

import (
	"encoding/json"
	"io"
	"net"
)

func jsonEncoder(w io.Writer) *json.Encoder {
	return json.NewEncoder(w)
}

func splitHostPort(addr string) (string, string, error) {
	return net.SplitHostPort(addr)
}
