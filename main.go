/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26 
*/
package main

import (
	oniProxy "github.com/saveio/porter/proxy"
)

func main() {
	proxy:= oniProxy.Init()
	proxy.StartServer()
}