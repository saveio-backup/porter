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
)
