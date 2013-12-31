package main

import (
	"encoding/json"
	"errors"
	"github.com/Terry-Mao/gopush-cluster/hash"
	"io/ioutil"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"time"
)

const (
	// internal failed
	retInternalErr = 65535
	// param error
	retParamErr = 65534
	// ok
	retOK = 0
	// create channel failed
	retCreateChannel = 1
	// add channel failed
	retAddChannle = 2
	// get channel failed
	retGetChannel = 3
	// add token failed
	retAddToken = 4
	// message push failed
	retPushMsg = 5
	// migrate failed
	retMigrate = 6
)

const (
	WebsocketProtocol = 0
	TCPProtocol       = 1
	heartbeatMsg      = "h"
	oneSecond         = int64(time.Second)
)

var (
	// Exceed the max subscriber per key
	MaxConnErr = errors.New("Exceed the max subscriber connection per key")
	// Assection type failed
	AssertTypeErr = errors.New("Subscriber assert type failed")
	// Auth token failed
	AuthTokenErr = errors.New("Auth token failed")
	// Token exists
	TokenExistErr = errors.New("Token already exist")

	// heartbeat bytes
	heartbeatBytes = []byte(heartbeatMsg)
	// heartbeat len
	heartbeatByteLen = len(heartbeatMsg)
)

func StartAdminHttp() error {
	adminServeMux := http.NewServeMux()
	// publish
	adminServeMux.HandleFunc("/pub", PublishHandle)
	// stat
	//adminServeMux.HandleFunc("/stat", StatHandle)
	// channel
	if Conf.Auth == 1 {
		adminServeMux.HandleFunc("/ch", ChannelHandle)
	}

	adminServeMux.HandleFunc("/debug/pprof/", pprof.Index)
	adminServeMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	adminServeMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	adminServeMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	Log.Info("start listen admin addr:%s", Conf.AdminAddr)
	err := http.ListenAndServe(Conf.AdminAddr, adminServeMux)
	if err != nil {
		Log.Error("http.ListenAdServe(\"%s\") failed (%s)", Conf.AdminAddr, err.Error())
		return err
	}

	return nil
}

// ChannelHandle create a user channle with the key by http
func ChannelHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		Log.Warn("client:%s's %s not allowed", r.RemoteAddr, r.Method)
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	// get params
	params := r.URL.Query()
	key := params.Get("key")
	if key == "" {
		Log.Warn("client:%s key param error", r.RemoteAddr)
		if err := retWrite(w, "key param error", retParamErr); err != nil {
			Log.Error("retWrite failed (%s)", err.Error())
		}

		return
	}

	Log.Info("user_key:\"%s\" add channel", key)
	// create a new channel for the user
	_, err := UserChannel.New(key)
	if err != nil {
		Log.Error("user_key:\"%s\" can't create channle", key)
		if err = retWrite(w, "create channel failed", retCreateChannel); err != nil {
			Log.Error("retWrite failed (%s)", err.Error())
		}

		return
	}

	// response
	if err = retWrite(w, "ok", retOK); err != nil {
		Log.Error("retWrite failed (%s)", err.Error())
	}

	return
}

// PublishHandle pub a message to a user with a key by http
func PublishHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Log.Warn("client:%s's %s not allowed", r.RemoteAddr, r.Method)
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	// get params
	params := r.URL.Query()
	key := params.Get("key")
	if key == "" {
		Log.Warn("client:%s key param error", r.RemoteAddr)
		if err := retWrite(w, "key param error", retParamErr); err != nil {
			Log.Error("retWrite failed (%s)", err.Error())
		}

		return
	}

	expireStr := params.Get("expire")
	expire, err := strconv.ParseInt(expireStr, 10, 64)
	if err != nil {
		// use default setting
		expire = Conf.MessageExpireSec * Second
		Log.Warn("user_key:\"%s\" param expire ParseInt failed use default setting %d", key, expire)
	}

	expire = time.Now().UnixNano() + expire*Second
	midStr := params.Get("mid")
	mid, err := strconv.ParseInt(midStr, 10, 64)
	if err != nil {
		Log.Warn("user_key:\"%s\" mid param error", key)
		if err = retWrite(w, "mid param error", retParamErr); err != nil {
			Log.Error("retWrite failed (%s)", err.Error())
		}

		return
	}

	// get message from http body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		Log.Error("user_key:\"%s\" ioutil.ReadAll(r.Body) failed (%s)", key, err.Error())
		if err = retWrite(w, "read http body error", retInternalErr); err != nil {
			Log.Error("retWrite() failed (%s)", err.Error())
		}

		return
	}

	// get a user channel
	c, err := UserChannel.Get(key)
	if err != nil {
		Log.Warn("user_key:\"%s\" can't get a channel (%s)", key, err.Error())
		if err = retWrite(w, "can't get a channel", retGetChannel); err != nil {
			Log.Error("retWrite() failed (%s)", err.Error())
		}

		return
	}

	// use the channel push message
	if err = c.PushMsg(&Message{Msg: string(body), Expire: expire, MsgID: mid}, key); err != nil {
		Log.Error("user_key:\"%s\" push message failed (%s)", key, err.Error())
		if err = retWrite(w, "push msg failed", retPushMsg); err != nil {
			Log.Error("retWrite() failed (%s)", err.Error())
		}

		return
	}

	// ret response
	if err = retWrite(w, "ok", retOK); err != nil {
		Log.Error("retWrite() failed (%s)", err.Error())
		return
	}
}

// MigrateHandle close Channel when node add or remove
func MigrateHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		Log.Warn("client:%s's %s not allowed", r.RemoteAddr, r.Method)
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	// get params
	params := r.URL.Query()
	nodesStr := params.Get("nodes")
	nodes := strings.Split(nodesStr, ",")
	if len(nodes) == 0 {
		Log.Warn("client:%s's nodes param error", r.RemoteAddr)
		if err := retWrite(w, "nodes param error", retParamErr); err != nil {
			Log.Error("retWrite failed (%s)", err.Error())
		}

		return
	}

	vnodeStr := params.Get("vnode")
	vnode, err := strconv.Atoi(vnodeStr)
	if err != nil {
		Log.Error("strconv.Atoi(\"%s\") failed (%s)", vnodeStr, err.Error())
		if err = retWrite(w, "vnode param error", retParamErr); err != nil {
			Log.Error("retWrite failed (%s)", err.Error())
		}

		return
	}

	// check current node in the nodes
	has := false
	for _, str := range nodes {
		if str == Conf.Node {
			has = true
		}
	}

	if !has {
		Log.Crit("make sure your migrate nodes right, there is no %s in nodes, this will cause all the node hit miss", Conf.Node)
		if err = retWrite(w, "migrate nodes may be error", retMigrate); err != nil {
			Log.Error("retWrite failed (%s)", err.Error())
		}

		return
	}

	channels := []Channel{}
	// init ketama
	ketama := hash.NewKetama2(nodes, vnode)
	// get all the channel lock
	for i, c := range UserChannel.Channels {
		Log.Info("migrate channel bucket:%d", i)
		c.Lock()
		for k, v := range c.Data {
			hn := ketama.Node(k)
			if hn != Conf.Node {
				channels = append(channels, v)
				Log.Debug("migrate key:\"%s\" hit node:\"%s\"", k, hn)
			}
		}

		c.Unlock()
		Log.Info("migrate channel bucket:%d finished", i)
	}

	// close all the migrate channels
	Log.Info("close all the migrate channels")
	for _, channel := range channels {
		if err = channel.Close(); err != nil {
			Log.Error("channel.Close() failed (%s)", err.Error())
			continue
		}
	}

	Log.Info("close all the migrate channels finished")
	// ret response
	if err = retWrite(w, "ok", retOK); err != nil {
		Log.Error("retWrite() failed (%s)", err.Error())
		return
	}
}

// retWrite write error response to the client
func retWrite(w http.ResponseWriter, msg string, ret int) error {
	res := map[string]interface{}{
		"msg": msg,
		"ret": ret,
	}

	strJson, err := json.Marshal(res)
	if err != nil {
		Log.Error("json.Marshal(\"%v\") failed", res)
		return err
	}

	respJson := string(strJson)
	if _, err := w.Write(strJson); err != nil {
		Log.Error("w.Write(\"%s\") failed (%s)", respJson, err.Error())
		return err
	}

	return nil
}