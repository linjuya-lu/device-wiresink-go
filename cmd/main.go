package main

import (
	"github.com/edgexfoundry/device-sdk-go/v4/pkg/startup"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/config"
	"github.com/linjuya-lu/device-wiresink-go-arm/internal/driver"
)

func main() {
	d := driver.WireSinkDeviceDriver()
	startup.Bootstrap(config.ServiceName, config.Version, d)
}
