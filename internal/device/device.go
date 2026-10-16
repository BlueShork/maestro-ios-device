package device

import (
	"fmt"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
)

type Device struct {
	Serial      string
	Name        string
	OSVersion   string
	ProductType string
	Entry       goios.DeviceEntry
}

func Get(udid string) (*Device, error) {
	entry, err := goios.GetDevice(udid)
	if err != nil {
		return nil, fmt.Errorf("device %s not found", udid)
	}
	return newDevice(udid, entry), nil
}

// WaitFor polls usbmux for the device for up to timeout. The iPhone drops off
// usbmux while it is locked or right after it is plugged in, and comes back
// within a few seconds once unlocked.
func WaitFor(udid string, timeout time.Duration) (*Device, error) {
	deadline := time.Now().Add(timeout)
	for {
		dev, err := Get(udid)
		if err == nil {
			return dev, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("iPhone %s is not reachable over USB. Unlock it and keep it "+
				"unlocked; if it still doesn't show up, unplug and replug the cable and tap Trust", udid)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func List() ([]Device, error) {
	list, err := goios.ListDevices()
	if err != nil {
		return nil, err
	}

	devices := make([]Device, 0, len(list.DeviceList))
	for _, d := range list.DeviceList {
		devices = append(devices, *newDevice(d.Properties.SerialNumber, d))
	}
	return devices, nil
}

func newDevice(serial string, entry goios.DeviceEntry) *Device {
	d := &Device{
		Serial: serial,
		Entry:  entry,
		Name:   "iOS Device",
	}

	if values, err := goios.GetValues(entry); err == nil {
		if values.Value.DeviceName != "" {
			d.Name = values.Value.DeviceName
		} else if values.Value.ProductType != "" {
			d.Name = values.Value.ProductType
		}
		d.OSVersion = values.Value.ProductVersion
		d.ProductType = values.Value.ProductType
	}

	return d
}
