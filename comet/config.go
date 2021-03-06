package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Terry-Mao/gopush-cluster/log"
	"io/ioutil"
	"runtime"
	"strings"
)

var (
	Conf     *Config
	ConfFile string
)

func init() {
	flag.StringVar(&ConfFile, "c", "./gopush2.conf", " set gopush2 config file path")
}

type Config struct {
	Node                string `json:"node"`
	DNS                 string `json:"dns"`
	Addr                string `json:"addr"`
	AdminAddr           string `json:"admin_addr"`
	PprofAddr           string `json:"pprof_addr"`
	RPCAddr             string `json:"rpc_addr"`
	RPCHeartbeatSec     int    `json:"rpc_heartbeat_sec"`
	RPCRetrySec         int    `json:"rpc_retry_sec"`
	ZookeeperAddr       string `json:"zookeeper_addr"`
	ZookeeperTimeout    int    `json:"zookeeper_timeout"`
	ZookeeperPath       string `json:"zookeeper_path"`
	Log                 string `json:"log"`
	LogLevel            int    `json:"log_level"`
	MaxProcs            int    `json:"max_procs"`
	TCPKeepAlive        int    `json:"tcp_keepalive"`
	HeartbeatSec        int    `json:"heartbeat_sec"`
	MessageExpireSec    int64  `json:"message_expire_sec"`
	ChannelExpireSec    int64  `json:"channel_expire_sec"`
	MaxStoredMessage    int    `json:"max_stored_message"`
	MaxSubscriberPerKey int    `json:"max_subscriber_per_key"`
	ChannelBucket       int    `json:"channel_bucket"`
	ReadBufInstance     int    `json:"read_buf_instance"`
	ReadBufNumPerInst   int    `json:"read_buf_num_per_inst"`
	ReadBufByte         int    `json:"read_buf_byte"`
	WriteBufByte        int    `json:"write_buf_byte"`
	Protocol            int    `json:"protocol"`
	Debug               int    `json:"debug"`
	Auth                int    `json:"auth"`
}

// NewConfig get a config struct.
func NewConfig(file string) (*Config, error) {
	c, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Printf("ioutil.ReadFile(\"%s\") failed (%s)", file, err.Error())
		return nil, err
	}

	cf := &Config{
		Node:                "node1",
		DNS:                 "localhost",
		Addr:                "localhost:8080",
		AdminAddr:           "localhost:8081",
		PprofAddr:           "localhost:8082",
		ZookeeperAddr:       "localhost:2181",
		RPCAddr:             "localhost:8083",
		RPCHeartbeatSec:     1,
		RPCRetrySec:         3,
		ZookeeperTimeout:    28800,
		ZookeeperPath:       "/gopush-cluster",
		Log:                 "./gopush.log",
		LogLevel:            0,
		MaxProcs:            runtime.NumCPU(),
		TCPKeepAlive:        1,
		HeartbeatSec:        30,
		MessageExpireSec:    10800,  // 3 hour
		ChannelExpireSec:    604800, // 24 * 7 hour
		MaxStoredMessage:    20,
		MaxSubscriberPerKey: 0, // no limit
		ChannelBucket:       16,
		ReadBufInstance:     runtime.NumCPU(),
		ReadBufNumPerInst:   1024,
		ReadBufByte:         512,
		WriteBufByte:        512,
		Protocol:            0,
		Debug:               0,
		Auth:                1,
	}

	if err = json.Unmarshal(c, cf); err != nil {
		log.DefaultLogger.Error("json.Unmarshal() failed (%s), config json: \"%s\"", err.Error(), string(c))
		return nil, err
	}

	cf.ZookeeperPath = strings.TrimRight(cf.ZookeeperPath, "/")
	return cf, nil
}
