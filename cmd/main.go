package main

import (
	"github.com/edgexfoundry/device-sdk-go/v4/pkg/startup"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/driver"
)

func main() {

	// hexStr := "238A08411011104200B300238A084110112C012C042CFFFFFFFFFFFF24238A0821BF342C002C002C238A0841AC0824238A0821BEF22C002C002C238A0841AC0824238A084110102C002C012C238A0841101124238A0841AC082C012C012C238A0841101124238A082623192C012C022C238A0841AC0824238A0841AC092C002C012C238A0841101124238A08405D012C002C012C238A0841101124238A0841AC1F2C002C012C238A08411011240000000000002C002C022C238A08411011"
	// data, err := hex.DecodeString(hexStr)
	// if err != nil {
	// 	panic(err)
	// }
	// crc := config.CRC16(data)
	// fmt.Printf("CRC = 0x%04X\n", crc)

	d := driver.WireSinkDeviceDriver()
	startup.Bootstrap(config.ServiceName, config.Version, d)
}
