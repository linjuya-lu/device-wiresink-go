package main

import (
	"github.com/edgexfoundry/device-sdk-go/v4/pkg/startup"
	"github.com/linjuya-lu/device-wiresink-go/internal/driver"
)

const (
	serviceName string = "device-wiresink"
	Version     string = "1.0.0"
)

func main() {
	d := driver.WireSinkDeviceDriver()

	startup.Bootstrap(serviceName, Version, d)

}
