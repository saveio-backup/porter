/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-29 
*/
package opcode

type Opcode uint32

const (
	ProxyRequestCode         	   Opcode = 0x0000f // 15
	ProxyResponseCode         	   Opcode = 0x00009 // 9
	KeepaliveCode          		   Opcode = 0x00002 // 20
	DisconnectCode         		   Opcode = 0x0000e // 14
)
