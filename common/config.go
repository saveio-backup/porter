/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-20
 */
package common

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"time"
)

const (
	DefaultConfigFilename = "./config.json"
)

var Version string

type Compression struct {
	Enable       bool     `json:"Enable"`
	CompressAlgo AlgoType `json:"CompressAlgo"`
	FileSize     int      `json:"FileSize"`
}

type InfluxDB struct {
	URL      string `json:"URL"`
	DBName   string `json:"DBName"`
	UserName string `json:"UserName"`
	Password string `json:"Password"`
	Interval int    `json:"Interval"`
}

type Configuration struct {
	InterfaceName   string        `json:"InterfaceName"`
	InnerIP         string        `json:"InnerIP"`
	PublicIP        string        `json:"PublicIP"`
	RandomPortBegin int           `json:"RandomPortBegin"`
	RandomPortRange int           `json:"RandomPortRange"`
	Protocol        string		  `json:"Protocol"`
	UPort           int           `json:"UPort"`
	KPort           int           `json:"KPort"`
	QPort           int           `json:"QPort"`
	TPort           int           `json:"TPort"`
	PortTimeout     time.Duration `json:"PortTimeout"`
	NetworkID       uint32        `json:"NetworkID"`
	WriteBufferSize int           `json:"WriteBufferSize"`
	LogDir          string        `json:"LogDir"`
	LogLevel        int           `json:"LogLevel"`
	PorterDBPath    string        `json:"PorterDBPath"`
	Compression     Compression   `json:"Compression"`
	InfluxDB        InfluxDB      `json:"InfluxDB"`
}

type ConfigFile struct {
	ConfigFile Configuration `json:"Configuration"`
}

var Parameters *Configuration

func InitConfig() {
	file, e := ioutil.ReadFile(DefaultConfigFilename)
	if e != nil {
		log.Fatalf("File error: %v\n", e)
		os.Exit(1)
	}
	// Remove the UTF-8 Byte Order Mark
	file = bytes.TrimPrefix(file, []byte("\xef\xbb\xbf"))

	config := ConfigFile{}
	e = json.Unmarshal(file, &config)
	if e != nil {
		log.Fatalf("Unmarshal json file erro %v", e)
		os.Exit(1)
	}
	Parameters = &(config.ConfigFile)
}
