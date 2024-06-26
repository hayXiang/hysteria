package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var lOG_LEVEL = 3 //0: no log, 1: error, 2: info, 3: debug

func LOG_DEBUG(out string) {
	if lOG_LEVEL >= 3 {
		log.Printf("[DEBUG]%s", out)
	}
}

func LOG_INFO(out string) {
	if lOG_LEVEL >= 2 {
		log.Printf("[INFO]%s", out)
	}
}

func LOG_ERROR(out string) {
	if lOG_LEVEL >= 1 {
		log.Printf("[ERROR]%s", out)
	}
}

type HttpConfigContext struct {
	transport                    *http.Transport
	transport_disable_keep_alive *http.Transport
	http_configs                 *(map[string]*HttpConfig)
}

type HttpRequestCallback func(*http.Request, *http.Client)

func HttpGet(src_url string, config *HttpConfig, time_out int, context *HttpConfigContext) (*http.Response, error) {
	LOG_INFO("begin to HttpGet " + src_url)
	defer func() {
		LOG_INFO("end to HttpGet " + src_url)
	}()
	req, err := http.NewRequest(http.MethodGet, src_url, nil)
	if err != nil {
		return nil, err
	}
	if len(config.headers) == 0 {
		req.Header.Add("User-Agent", "Dalvik/2.1.0 (Linux; U; Android 9; PCRT00 Build/PQ3A.190605.01111538)")
	} else {
		for key, value := range config.headers {
			req.Header.Add(key, value)
		}
	}

	client := http.Client{}

	if config.keep_alive {
		client.Transport = context.transport
		if config.socks5_address != "" && context.transport != nil {
			proxy_uri, _ := url.Parse(config.socks5_address)
			context.transport.Proxy = http.ProxyURL(proxy_uri)
		}
	} else {
		client.Transport = context.transport_disable_keep_alive
		if config.socks5_address != "" && context.transport_disable_keep_alive.Proxy != nil {
			proxy_uri, _ := url.Parse(config.socks5_address)
			context.transport_disable_keep_alive.Proxy = http.ProxyURL(proxy_uri)
		}
	}

	if !config.follow_redirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	if time_out != -1 {
		client.Timeout = time.Second * time.Duration(time_out)
	}
	res, err := client.Do(req)
	if err != nil {
		return res, err
	}
	return res, err

}

type HttpResponseCache struct {
	expried_time int
	create_time  time.Time
	body         []byte
	content_type string
	last_url     string
	mutex        sync.Mutex
}

type HttpConfig struct {
	request_validator     map[string]string
	headers               map[string]string
	url                   string
	url_replace           OrderMap
	response_body_replace OrderMap
	response_cache        *HttpResponseCache
	debug_response        bool
	follow_redirect       bool
	m3u8_proxy            bool
	error_code_to_302     bool
	time_out              int
	max_session           int
	keep_alive            bool
	mutex                 sync.Mutex
	socks5_address        string
}

type OrderMap struct {
	order_keys []string
	order_map  map[string]string
}

var https = make(map[*http.ServeMux]*HttpConfigContext)

func http_path(url string) string {
	path := strings.Replace(url, "http://", "", 1)
	path = strings.Replace(path, "https://", "", 1)
	index := strings.Index(path, "?")
	if index != -1 {
		path = path[:index]
	}
	return path
}

func get_http_config(uri string, context *HttpConfigContext) *HttpConfig {
	for key, value := range *context.http_configs {
		if key == "*" {
			continue
		}

		found, err := regexp.MatchString(key, uri)
		if err != nil || !found {
			continue
		}
		return value
	}
	return (*context.http_configs)["*"]
}

func dec_session_count(config *HttpConfig, is_need_to_restore_session *bool) {
	config.mutex.Lock()
	config.max_session -= 1
	config.mutex.Unlock()
	*is_need_to_restore_session = true
}

func inc_session_count(config *HttpConfig, is_need_to_restore_session *bool) {
	if *is_need_to_restore_session {
		config.mutex.Lock()
		config.max_session += 1
		config.mutex.Unlock()
		*is_need_to_restore_session = false
	}
}

func http_resposne_copy(config *HttpConfig, content_type string, is_need_to_restore_session *bool, w *http.ResponseWriter, resp *http.Response, request_uri *url.URL, rawUrl *string) {
	if strings.Contains(content_type, "mpegurl") {
		m3u8_body := ""
		str_last_request_url := ""
		if config.response_cache != nil && config.response_cache.body != nil {
			m3u8_body = string(config.response_cache.body)
			str_last_request_url = config.response_cache.last_url
		} else {
			bytes_m3u8_body, _ := io.ReadAll(resp.Body)
			inc_session_count(config, is_need_to_restore_session)
			str_last_request_url = resp.Request.URL.String()
			if config.response_cache != nil && config.response_cache.expried_time != 0 {
				config.response_cache.content_type = content_type
				config.response_cache.mutex.Lock()
				config.response_cache.create_time = time.Now()
				config.response_cache.body = bytes_m3u8_body
				config.response_cache.last_url = str_last_request_url
				config.response_cache.mutex.Unlock()
			}
			m3u8_body = string(bytes_m3u8_body)
		}
		if config.debug_response {
			LOG_DEBUG(m3u8_body)
		}
		line := bufio.NewScanner(strings.NewReader(m3u8_body))
		lastRequestUrl, _ := url.Parse(str_last_request_url)
		final_m3u8_body := ""
		for line.Scan() {
			ext_info := line.Text()
			if strings.Contains(ext_info, ".ts") {
				if strings.Index(ext_info, "/") == 0 {
					ext_info = lastRequestUrl.Scheme + "://" + lastRequestUrl.Host + ext_info
				} else if strings.Index(ext_info, "http://") != 0 && strings.Index(ext_info, "https://") != 0 && lastRequestUrl.String() != *rawUrl {
					lastRequstRawUrl := lastRequestUrl.String()
					ext_info = lastRequstRawUrl[0:strings.LastIndex(lastRequstRawUrl, "/")] + "/" + ext_info
				}

				if config.m3u8_proxy {
					ext_info = strings.ReplaceAll(ext_info, "http://", "/http:/")
					ext_info = strings.ReplaceAll(ext_info, "https://", "/https:/")
				}
			}
			final_m3u8_body += (ext_info + "\n")
		}
		for _, _key := range config.response_body_replace.order_keys {
			re := regexp.MustCompile(_key)
			_value := config.response_body_replace.order_map[_key]
			_value = strings.ReplaceAll(_value, "{raw_url_query[*]}", request_uri.RawQuery)
			_value = strings.ReplaceAll(_value, "{raw_url_query[type]}", request_uri.Query().Get("type"))
			_value = strings.ReplaceAll(_value, "{raw_url_query[token]}", request_uri.Query().Get("token"))
			final_m3u8_body = re.ReplaceAllString(final_m3u8_body, _value)
		}
		io.Copy(*w, strings.NewReader(final_m3u8_body))
	} else {
		if config.response_cache == nil && len(config.response_body_replace.order_keys) == 0 {
			io.Copy(*w, resp.Body)
		} else {
			m3u8_body := ""
			if config.response_cache != nil && config.response_cache.body != nil {
				m3u8_body = string(config.response_cache.body)
			} else {
				bytes_m3u8_body, _ := io.ReadAll(resp.Body)
				inc_session_count(config, is_need_to_restore_session)
				if config.response_cache != nil && config.response_cache.expried_time != 0 {
					config.response_cache.content_type = content_type
					config.response_cache.mutex.Lock()
					config.response_cache.create_time = time.Now()
					config.response_cache.body = bytes_m3u8_body
					config.response_cache.last_url = *rawUrl
					config.response_cache.mutex.Unlock()
				}
				m3u8_body = string(bytes_m3u8_body)
			}
			if config.debug_response {
				LOG_DEBUG(m3u8_body)
			}
			for _, _key := range config.response_body_replace.order_keys {
				re := regexp.MustCompile(_key)
				_value := config.response_body_replace.order_map[_key]
				_value = strings.ReplaceAll(_value, "{raw_url_query[*]}", request_uri.RawQuery)
				_value = strings.ReplaceAll(_value, "{raw_url_query[type]}", request_uri.Query().Get("type"))
				_value = strings.ReplaceAll(_value, "{raw_url_query[token]}", request_uri.Query().Get("token"))
				m3u8_body = re.ReplaceAllString(m3u8_body, _value)
			}
			io.Copy(*w, strings.NewReader(m3u8_body))
		}
	}
}

func proxy(w http.ResponseWriter, req *http.Request) {
	context := req.Context().Value("my_context").(*HttpConfigContext)
	LOG_INFO("[S] " + GetClientIP(req) + "," + req.RequestURI)
	defer LOG_INFO("[E]" + req.RequestURI)
	request_uri, _ := url.Parse(req.RequestURI)

	rawUrl := req.RequestURI[1:]
	rawUrl = strings.Replace(rawUrl, "http:/", "http://", 1)
	rawUrl = strings.Replace(rawUrl, "https:/", "https://", 1)

	config := get_http_config(rawUrl, context)
	if strings.Contains(rawUrl, "follow_redirect=false") {
		rawUrl = strings.Replace(rawUrl, "?follow_redirect", "", 1)
		rawUrl = strings.Replace(rawUrl, "&follow_redirect", "", 1)
		config.follow_redirect = false
	}

	for _key, _value := range config.request_validator {
		infos := strings.Split(_key, ".")
		if len(infos) != 2 {
			panic("validator config error:")
		}

		if infos[0] == "header" {
			re := regexp.MustCompile(_value)
			http_value := strings.Join(req.Header[infos[1]], ",")
			if !re.MatchString(http_value) {
				error_msg := "http " + infos[0] + " error, key=" + _key
				w.WriteHeader(500)
				w.Write([]byte(error_msg))
				LOG_ERROR(error_msg + ",value=" + http_value)
				return
			}
		} else if infos[0] == "query" {
			re := regexp.MustCompile(_value)
			http_value := strings.Join(request_uri.Query()[infos[1]], ",")
			if !re.MatchString(http_value) {
				error_msg := "http " + infos[0] + " error, key=" + _key
				w.WriteHeader(500)
				w.Write([]byte(error_msg))
				LOG_ERROR(error_msg + ",value=" + http_value)
				return
			}
		}
	}

	if config.response_cache != nil && config.response_cache.expried_time != 0 {
		if config.response_cache.body != nil {
			if config.response_cache.create_time.Add(time.Duration(config.response_cache.expried_time * int(time.Second))).After(time.Now()) {
				http_resposne_copy(config, config.response_cache.content_type, nil, &w, nil, request_uri, &rawUrl)
				return
			} else {
				config.response_cache.body = nil
			}
		}
	}

	config.mutex.Lock()
	if config.max_session == 0 {
		location := "/" + req.RequestURI[1:]
		err_msg := "session count is 0, redirect to " + location
		config.mutex.Unlock()
		LOG_ERROR(err_msg)
		time.Sleep(1000 * time.Millisecond)
		w.Header().Set("Location", location)
		w.WriteHeader(302)
		return
	}
	config.mutex.Unlock()

	is_need_to_restore_session := false
	dec_session_count(config, &is_need_to_restore_session)
	time_out := config.time_out
	if strings.Contains(rawUrl, "live_mode=ts") || strings.Contains(rawUrl, "live_mode=flv") {
		time_out = -1
	}

	real_url := rawUrl
	for _, _key := range config.url_replace.order_keys {
		re := regexp.MustCompile(_key)
		_value := config.url_replace.order_map[_key]
		_value = strings.ReplaceAll(_value, "{raw_url_query[*]}", request_uri.RawQuery)
		_value = strings.ReplaceAll(_value, "{raw_url_query[type]}", request_uri.Query().Get("type"))
		_value = strings.ReplaceAll(_value, "{raw_url_query[token]}", request_uri.Query().Get("token"))
		real_url = re.ReplaceAllString(real_url, _value)
	}

	if !strings.Contains(real_url, ("http://")) && !strings.Contains(real_url, ("https://")) {
		err_msg := "invalid url:" + req.RequestURI[1:]
		w.WriteHeader(500)
		w.Write([]byte(err_msg))
		LOG_ERROR("invalid url:" + real_url + ", must be start http:// or https://")
		inc_session_count(config, &is_need_to_restore_session)
		return
	}

	resp, err := HttpGet(real_url, config, time_out, context)
	defer func() {
		if err == nil {
			resp.Body.Close()
		}
		inc_session_count(config, &is_need_to_restore_session)
	}()
	if err != nil {
		LOG_ERROR(fmt.Sprint(err))
		if config.error_code_to_302 {
			location := "/" + req.RequestURI[1:]
			LOG_ERROR("get " + location + " error, redirect to get it")
			w.Header().Set("Location", location)
			w.WriteHeader(302)

		} else {
			w.WriteHeader(500)
			w.Write([]byte("error"))
		}
		return
	}

	if resp.StatusCode == 301 || resp.StatusCode == 302 {
		location := resp.Header.Get("Location")
		w.Header().Set("Location", location)
	} else if (resp.StatusCode < 200 || resp.StatusCode > 299) && config.error_code_to_302 {
		m3u8_body, _ := io.ReadAll(resp.Body)
		LOG_ERROR(fmt.Sprintf("http_code:%d,status:%s,body:%s", resp.StatusCode, resp.Status, string(m3u8_body)))
		location := "/" + req.RequestURI[1:]
		LOG_ERROR("get " + location + " error, redirect to get it")
		w.Header().Set("Location", location)
		w.WriteHeader(302)
		return
	}

	w.WriteHeader(resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	http_resposne_copy(config, contentType, &is_need_to_restore_session, &w, resp, request_uri, &rawUrl)
	if strings.Contains(rawUrl, "live_mode=ts") || strings.Contains(rawUrl, "live_mode=flv") {
		panic("the stream must not be close")
	}
}

func GetClientIP(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	x_ip := strings.TrimSpace(strings.Split(xForwardedFor, ",")[0])
	if x_ip == "" {
		x_ip = r.Header.Get("X-Real-IP")
	}
	return "r-ip:" + r.RemoteAddr + ",x-ip:" + x_ip
}

func start_proxy(socks5_address string, listen_address string) *http.Server {
	is_m3u8_proxy := true             //flag.Bool("m3u8_proxy", true, "replace m3u8 ts file")
	http_config_file := "./http.json" //flag.String("c", "./http.json", "http config file")
	http_error_code_to_302 := false   //.Bool("http_error_code_to_302", false, "replace http error code to 302")
	http_time_out := 30               //flag.Int("http_tim_out", 30, "http time out")
	http_keep_alive := true           //flag.Bool("http_keep_alive", true, "http keep alive")

	var default_config HttpConfig
	default_config.follow_redirect = true
	default_config.m3u8_proxy = is_m3u8_proxy
	default_config.error_code_to_302 = http_error_code_to_302
	default_config.time_out = http_time_out
	default_config.max_session = -1
	default_config.debug_response = true
	default_config.keep_alive = http_keep_alive
	default_config.socks5_address = socks5_address
	var http_configs = make(map[string]*HttpConfig)
	http_configs["*"] = &default_config

	data, err := ioutil.ReadFile(http_config_file)
	if err == nil {
		str_http_config := string(data)
		if str_http_config != "" {
			var config interface{}
			json.Unmarshal([]byte(str_http_config), &config)

			datas := config.([]interface{})
			for _, data := range datas {
				var http_config HttpConfig
				http_config.follow_redirect = default_config.follow_redirect
				http_config.m3u8_proxy = default_config.m3u8_proxy
				http_config.error_code_to_302 = default_config.error_code_to_302
				http_config.time_out = default_config.time_out
				http_config.keep_alive = default_config.keep_alive
				http_config.max_session = default_config.max_session
				http_config.headers = make(map[string]string)
				http_config.url_replace.order_map = make(map[string]string)
				http_config.request_validator = make(map[string]string)
				http_config.response_body_replace.order_map = make(map[string]string)
				http_config.debug_response = default_config.debug_response

				for k, v := range data.(map[string]interface{}) {
					if k == "request_validator" {
						for _key, _value := range v.(map[string]interface{}) {
							http_config.request_validator[_key] = _value.(string)
						}
					}

					if k == "headers" {
						for header_key, header_value := range v.(map[string]interface{}) {
							http_config.headers[header_key] = header_value.(string)
						}
					}
					if k == "url_replace" {
						for _, _maps := range v.([]interface{}) {
							for _key, _value := range _maps.(map[string]interface{}) {
								http_config.url_replace.order_keys = append(http_config.url_replace.order_keys, _key)
								http_config.url_replace.order_map[_key] = _value.(string)
							}
						}
					}
					if k == "response_body_replace" {
						for _, _maps := range v.([]interface{}) {
							for _key, _value := range _maps.(map[string]interface{}) {
								http_config.response_body_replace.order_keys = append(http_config.response_body_replace.order_keys, _key)
								http_config.response_body_replace.order_map[_key] = _value.(string)
							}
						}
					}

					if k == "resposne_body_cache_time" {
						cache_time := int(v.(float64))
						if cache_time > 0 {
							http_config.response_cache = new(HttpResponseCache)
							http_config.response_cache.expried_time = cache_time
						}
					}

					if k == "url" {
						http_config.url = v.(string)
					}

					if k == "debug_response" {
						http_config.debug_response = v.(bool)
					}

					if k == "follow_redirect" {
						http_config.follow_redirect = v.(bool)
					}

					if k == "m3u8_proxy" {
						http_config.m3u8_proxy = v.(bool)
					}

					if k == "max_session" {
						http_config.max_session = int(v.(float64))
					}

					if k == "error_code_to_302" {
						http_config.error_code_to_302 = v.(bool)
					}

					if k == "time_out" {
						http_config.time_out = int(v.(float64))
					}

					if k == "keep_alive" {
						http_config.keep_alive = v.(bool)
					}
				}
				http_configs[http_path(http_config.url)] = &http_config
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		config_context, ok := https[mux]
		if !ok {
			var _config_context HttpConfigContext
			config_context = &_config_context
			_config_context.http_configs = &http_configs
			_config_context.transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				Dial: func(network, addr string) (net.Conn, error) {
					return net.DialTimeout(network, addr, time.Duration(5*time.Second))
				},
			}

			_config_context.transport_disable_keep_alive = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				Dial: func(network, addr string) (net.Conn, error) {
					return net.DialTimeout(network, addr, time.Duration(5*time.Second))
				},
				DisableKeepAlives: true,
			}
			https[mux] = &_config_context
		}
		ctx := context.WithValue(r.Context(), "my_context", config_context)
		*r = *r.WithContext(ctx)
		proxy(w, r)
	})

	server := &http.Server{
		Addr:    listen_address,
		Handler: mux,
	}
	fmt.Println("server listen :" + listen_address)
	go server.ListenAndServe()
	return server
}
