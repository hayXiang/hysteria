package main

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

//export HY2_Init
func HY2_Init(hy2_url *C.char) C.ulonglong {
	var hy2_client_uri HY2ClientURI
	_url, _err := url.Parse(C.GoString(hy2_url))
	if _err != nil {
		return 0
	}
	hy2_client_uri.url = *_url
	http_listen_port := -1
	if _url.Query().Get("http_listen_port") != "" {
		i, err := strconv.Atoi(_url.Query().Get("http_listen_port"))
		if err != nil {
			return 0
		}
		http_listen_port = i
	}

	socks_listen_port := -1
	if _url.Query().Get("socks_listen_port") != "" {
		i, err := strconv.Atoi(_url.Query().Get("socks_listen_port"))
		if err != nil {
			return 0
		}
		socks_listen_port = i
	}

	client, err := create_hy2_client(&hy2_client_uri, http_listen_port, socks_listen_port)
	if err != nil {
		return 0
	}
	return C.ulonglong(uintptr(unsafe.Pointer(client)))
}

//export HY2_GetPlayUrl
func HY2_GetPlayUrl(handler C.ulonglong, play_url *C.char, result *C.char) int {
	hy2_client := (*HY2Client)(unsafe.Pointer(uintptr(handler)))
	ret_play_url := strings.ReplaceAll(C.GoString(play_url), "//", "/")
	ret_play_url = fmt.Sprintf("http://127.0.0.1:%d/%s", hy2_client.http_listen_port, ret_play_url)
	cs_play_url := C.CString(ret_play_url)
	C.strcpy(result, cs_play_url)
	C.free(unsafe.Pointer(cs_play_url))
	return 0
}

//export HY2_GetProxyPorts
func HY2_GetProxyPorts(handler C.ulonglong, http_listen_port *C.int, socks5_listen_port *C.int) int {
	hy2_client := (*HY2Client)(unsafe.Pointer(uintptr(handler)))
	*http_listen_port = C.int(hy2_client.http_listen_port)
	*socks5_listen_port = C.int(hy2_client.socks5_listen_port)
	return 0
}

//export HY2_UnInit
func HY2_UnInit(handler C.ulonglong) int {
	hy2_client := (*HY2Client)(unsafe.Pointer(uintptr(handler)))
	(*hy2_client.client).Close()
	(*hy2_client.socks5_server).Close()
	hy2_client.http_server.Close()
	delete(hy2_clients, hy2_client.key)
	return 0
}

//export XYQ_GetPlayUrl
func XYQ_GetPlayUrl(hy2_play_url *C.char, result *C.char) C.ulonglong {
	client, play_url := get_play_url(C.GoString(hy2_play_url), -1, -1)
	if play_url == "" {
		return 0
	}
	cs_play_url := C.CString(play_url)
	C.strcpy(result, cs_play_url)
	C.free(unsafe.Pointer(cs_play_url))
	return C.ulonglong(uintptr(unsafe.Pointer(client)))
}

func main() {
	fmt.Println(get_play_url("hy2://idhLuAuSjmU3aZmC9JiFmWoF:8092@free.9528.eu.org:9529/stream/hk/test/master.m3u8?u=admin&p=70db29aeaebf312fd2b93d3d69d99a16ad3319d0b354567f2be5c5e91c6be18b", -1, -1))
	time.Sleep(30 * time.Second)
}
