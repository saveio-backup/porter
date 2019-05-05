/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-05 
*/
package test

import (
	"testing"
	"github.com/saveio/porter/common"
	"fmt"
)

func TestGetLocalIP(t *testing.T)  {
	fmt.Println("local-ip:",common.GetLocalIP())
}