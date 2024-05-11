package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/apernet/hysteria/app/cmd"
	"github.com/apernet/hysteria/core/client"
)

func get_2_free_port() (int, int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return -1, -1, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return -1, -1, err
	}
	defer l.Close()
	port1 := l.Addr().(*net.TCPAddr).Port

	addr2, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return -1, -1, err
	}

	l2, err := net.ListenTCP("tcp", addr2)
	if err != nil {
		return -1, -1, err
	}
	defer l2.Close()
	port2 := l2.Addr().(*net.TCPAddr).Port
	return port1, port2, err
}

type HY2ClientURI struct {
	url url.URL
}

func (hy2 *HY2ClientURI) key() string {
	return hy2.url.String()
}

func (hy2 *HY2ClientURI) set_query(query string) {
	hy2.url.RawQuery = query
}

type HY2Client struct {
	client             *client.Client
	http_listen_port   int
	http_server        *http.Server
	socks5_listen_port int
	socks5_server      *net.Listener
	key                string
}

var locker sync.Mutex
var hy2_clients = make(map[string](*HY2Client))

func create_hy2_client_uri(hy2_play_url *url.URL) (*HY2ClientURI, error) {
	var client_uri HY2ClientURI
	_passwod, _ := hy2_play_url.User.Password()
	var server_ip string
	var server_port string
	if strings.Contains(hy2_play_url.User.String(), "_") {
		server_ip = strings.Split(_passwod, "_")[0]
		server_port = strings.Split(_passwod, "_")[1]
	} else {
		server_ip = hy2_play_url.Hostname()
		server_port = _passwod
	}

	_url, _err := url.Parse(fmt.Sprintf("hysteria2://%s@%s:%s/", hy2_play_url.User.Username(), server_ip, server_port))
	if _err != nil {
		fmt.Println(_err.Error())
		return nil, _err
	}
	client_uri.url = *_url
	return &client_uri, nil
}

func create_hy2_client(hy2_client_url *HY2ClientURI, http_listen_port int, socks5_listen_port int) (*HY2Client, error) {
	hy2_client_key := hy2_client_url.key()
	locker.Lock()
	defer locker.Unlock()
	hy2_client, ok := hy2_clients[hy2_client_key]
	if !ok {
		hy2_socks5_listen_port, hy2_http_listen_port, _ := get_2_free_port()
		if http_listen_port > 0 {
			hy2_http_listen_port = http_listen_port
		}

		if socks5_listen_port > 0 {
			hy2_socks5_listen_port = socks5_listen_port
		}
		hy2_porxy_address := fmt.Sprintf("127.0.0.1:%d", hy2_socks5_listen_port)
		fmt.Printf("start hy2, porxy=socks5://%s\n", hy2_porxy_address)

		http_server := start_proxy("socks5://"+hy2_porxy_address, fmt.Sprintf("127.0.0.1:%d", hy2_http_listen_port))
		_query := hy2_client_url.url.Query()
		_query.Set("proxy", hy2_porxy_address)
		_query.Set("bandwidth_up", "100")
		_query.Set("bandwidth_down", "300")
		hy2_client_url.set_query(_query.Encode())
		client, socks5_server, err := cmd.XYQ_Execute(&hy2_client_url.url)
		if err != nil {
			fmt.Println(err.Error())
			return nil, err
		}
		var new_hy2_client HY2Client
		hy2_client = &new_hy2_client
		new_hy2_client.key = hy2_client_key
		new_hy2_client.http_server = http_server
		new_hy2_client.socks5_server = socks5_server
		new_hy2_client.client = client
		new_hy2_client.http_listen_port = hy2_http_listen_port
		new_hy2_client.socks5_listen_port = hy2_socks5_listen_port
		hy2_clients[hy2_client_key] = &new_hy2_client
	}
	return hy2_client, nil
}

func get_play_url(hy2_play_url string, http_listen_port int, socks5_listen_port int) (*HY2Client, string) {
	if strings.Index(hy2_play_url, "hy2://") != 0 && strings.Index(hy2_play_url, "hy2s://") != 0 {
		return nil, hy2_play_url
	}

	_url, _err := url.Parse(hy2_play_url)
	if _err != nil {
		return nil, hy2_play_url
	}

	hy2_client_uri, err := create_hy2_client_uri(_url)
	if err != nil {
		return nil, hy2_play_url
	}
	hy2_client, err := create_hy2_client(hy2_client_uri, http_listen_port, socks5_listen_port)
	if err != nil {
		return nil, hy2_play_url
	}
	http_schema := "http"
	if strings.Index(hy2_play_url, "hy2s://") == 0 {
		http_schema = "https"
	}
	play_url := fmt.Sprintf("http://127.0.0.1:%d/%s:/%s%s", hy2_client.http_listen_port, http_schema, _url.Host, _url.Path)
	if _url.RawQuery != "" {
		play_url += ("?" + _url.RawQuery)
	}
	return hy2_client, play_url
}
