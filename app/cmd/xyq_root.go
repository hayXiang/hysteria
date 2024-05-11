package cmd

import (
	"errors"
	"net"
	"net/url"
	net_url "net/url"

	"github.com/apernet/hysteria/app/internal/socks5"
	"github.com/apernet/hysteria/core/client"
	"github.com/apernet/hysteria/extras/correctnet"
	"go.uber.org/zap"
)

func XYQ_Execute(hy_url *url.URL) (*client.Client, *net.Listener, error) {
	initLogger()
	return XYQ_RunClientByUrl(hy_url)
}

func XYQ_RunClientByUrl(hy2_url *net_url.URL) (*client.Client, *net.Listener, error) {
	logger.Info("client mode")
	var config clientConfig
	config.Auth = hy2_url.User.Username()
	config.Server = hy2_url.Host
	config.TLS.Insecure = false
	if hy2_url.Query().Get("insecure") == "" || hy2_url.Query().Get("insecure") == "1" {
		config.TLS.Insecure = true
	}
	config.Bandwidth.Up = hy2_url.Query().Get("bandwidth_up") + " mbps"
	config.Bandwidth.Down = hy2_url.Query().Get("bandwidth_down") + " mbps"
	var socks5_config socks5Config
	config.SOCKS5 = &socks5_config
	config.SOCKS5.Listen = hy2_url.Query().Get("proxy")
	config.Lazy = true
	if hy2_url.Query().Get("lazy") == "false" {
		config.Lazy = false
	}

	c, err := client.NewReconnectableClient(
		config.Config,
		func(c client.Client, info *client.HandshakeInfo, count int) {
			connectLog(info, count)
			// On the client side, we start checking for updates after we successfully connect
			// to the server, which, depending on whether lazy mode is enabled, may or may not
			// be immediately after the client starts. We don't want the update check request
			// to interfere with the lazy mode option.
			if count == 1 && !disableUpdateCheck {
				go runCheckUpdateClient(c)
			}
		}, config.Lazy)
	if err != nil {
		return nil, nil, err
	}

	uri := config.URI()
	logger.Info("use this URI to share your server", zap.String("uri", uri))
	l, err := XYQ_ClientSOCKS5(*config.SOCKS5, c)
	if err != nil {
		return nil, nil, err
	}
	return &c, l, nil
}

func XYQ_ClientSOCKS5(config socks5Config, c client.Client) (*net.Listener, error) {
	if config.Listen == "" {
		return nil, configError{Field: "listen", Err: errors.New("listen address is empty")}
	}
	l, err := correctnet.Listen("tcp", config.Listen)
	if err != nil {
		return nil, configError{Field: "listen", Err: err}
	}
	var authFunc func(username, password string) bool
	username, password := config.Username, config.Password
	if username != "" && password != "" {
		authFunc = func(u, p string) bool {
			return u == username && p == password
		}
	}
	s := socks5.Server{
		HyClient:    c,
		AuthFunc:    authFunc,
		DisableUDP:  config.DisableUDP,
		EventLogger: &socks5Logger{},
	}
	logger.Info("SOCKS5 server listening", zap.String("addr", config.Listen))
	go s.Serve(l)
	return &l, nil
}
