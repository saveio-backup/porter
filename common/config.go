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

type Configuration struct {
	InterfaceName   string        `json:"InterfaceName"`
	InnerIP         string        `json:"InnerIP"`
	PublicIP        string        `json:"PublicIP"`
	RandomPortBegin int           `json:"RandomPortBegin"`
	RandomPortRange int           `json:"RandomPortRange"`
	UPort           int           `json:"UPort"`
	KPort           int           `json:"KPort"`
	QPort           int           `json:"QPort"`
	TPort           int           `json:"TPort"`
	PortTimeout     time.Duration `json:"PortTimeout"`
	LogDir          string        `json:"LogDir"`
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
