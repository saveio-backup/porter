/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26 
*/
package main

import (
	oniProxy "github.com/oniio/oniProxy/proxy"
)

func main() {
	proxy:= oniProxy.Init()
	proxy.StartServer()
}