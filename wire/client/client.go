package client

import (
	"crypto/tls"
	"github.com/smallfz/savage-wire/wire"
	"golang.org/x/net/websocket"
	"net/url"
	"path"
	"strings"
)

func connect(dsn string) (*conn, error) {
	uri, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	scheme := strings.ToLower(uri.Scheme)
	switch scheme {
	case "ws":
	case "wss":
	case "http":
		uri.Scheme = "ws"
	case "https":
		uri.Scheme = "wss"
	default:
		uri.Scheme = "wss"
	}

	dbName := path.Base(uri.Path)
	reqAuth := &wire.ReqAuth{DbName: strings.ToLower(dbName)}
	if uri.User != nil {
		reqAuth.User = uri.User.Username()
		if pwd, ok := uri.User.Password(); ok {
			reqAuth.Pwd = pwd
		}
	}

	origin, err := url.Parse("http://localhost")
	if err != nil {
		return nil, err
	}

	uri.Path = "/ws"
	uri.Fragment = ""

	cfg := &websocket.Config{
		Location: uri,
		Origin:   origin,
		TlsConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Protocol: []string{""},
		Version:  13,
	}
	wsConn, err := websocket.DialConfig(cfg)
	if err != nil {
		return nil, err
	}

	rw := wire.NewReadWriterAsync(wsConn)
	c := &conn{rw: rw}
	if err := c.handshake(reqAuth); err != nil {
		return nil, err
	}

	return c, nil
}

func Open(dsn string) (Conn, error) {
	return connect(dsn)
}

func OpenDSN(dsn string) (*conn, error) {
	return connect(dsn)
}
