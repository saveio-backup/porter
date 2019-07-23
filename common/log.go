/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-07-23 
*/
package common

import (
	"time"
	"github.com/saveio/themis/common/log"
)

func CheckLogFileSize() {
	ti := time.NewTicker(time.Second)
	for {
		select {
		case <-ti.C:
			isNeedNewFile := log.CheckIfNeedNewFile()
			if isNeedNewFile {
				log.ClosePrintLog()
				log.InitLog(int(Parameters.LogLevel), GetLogDir(), log.Stdout)
			}
		}
	}
}
