/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-05
 */
package test

import (
	"fmt"
	"github.com/saveio/porter/common"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	fmt.Println("local-ip:", common.GetLocalIP())
}
