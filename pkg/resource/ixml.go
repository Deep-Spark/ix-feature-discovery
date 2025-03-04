/*
 * Copyright (c) 2024, Shanghai Iluvatar CoreX Semiconductor Co., Ltd.
 * All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may
 * not use this file except in compliance with the License. You may obtain
 * a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package resource

import (
	"fmt"
	"strconv"
	"strings"

	"gitee.com/deep-spark/go-ixml/pkg/ixml"
	"k8s.io/klog/v2"
)

type ixmlLib struct {
}

var _ Manager = (*ixmlLib)(nil)

// NewIXMLManager creates a new manager that uses IXML to query and manage devices
func NewIXMLManager() Manager {
	m := ixmlLib{}
	return m
}

// GetCudaRuntimeVersion : Return the cuda runtime version using IXML
func (l ixmlLib) GetCudaRuntimeVersion() (*uint, *uint, error) {
	v, ret := ixml.SystemGetCudaDriverVersion()
	if ret != ixml.SUCCESS {
		return nil, nil, fmt.Errorf("failed to get cuda runtime version: %v", ret)
	}
	vi, err := strconv.Atoi(v)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert cuda runtime version: %v", err)
	}
	major := uint(vi) / 1000
	minor := uint(vi) % 1000 / 10
	klog.Infof("success to get cuda runtime version, major: %d, minor: %d", major, minor)
	return &major, &minor, nil
}

// GetDevices returns the IXML devices for the manager
func (l ixmlLib) GetDevices() ([]Device, error) {
	count, ret := ixml.DeviceGetCount()
	if ret != ixml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", ret)
	}

	var devices []Device
	for idx := uint(0); idx < count; idx++ {
		devRef := new(ixml.Device)
		ret = ixml.DeviceGetHandleByIndex(idx, devRef)
		if ret != ixml.SUCCESS {
			return nil, fmt.Errorf("failed to get device by index %d: %v", idx, ret)
		}

		device := ixmlDevice{
			Device: devRef,
		}
		devices = append(devices, device)
	}

	return devices, nil
}

// GetIXDriverVersion returns the ix driver version
func (l ixmlLib) GetIXDriverVersion() (string, error) {
	v, ret := ixml.SystemGetDriverVersion()
	if ret != ixml.SUCCESS {
		return "", fmt.Errorf("failed to get ix driver version: %v", ret)
	}
	klog.Infof("success to get ix driver version: %s", v)
	return v, nil
}

// Init initialises the library
func (l ixmlLib) Init() error {
	ret := ixml.Init()
	if ret != ixml.SUCCESS {
		return fmt.Errorf("failed to init: %v", ret)
	}
	return nil
}

// Shutdown shuts down the library
func (l ixmlLib) Shutdown() error {
	ret := ixml.Shutdown()
	if ret != ixml.SUCCESS {
		return fmt.Errorf("failed to shutdown: %v", ret)
	}
	return nil
}

type ixmlDevice struct {
	*ixml.Device
}

var _ Device = (*ixmlDevice)(nil)

// GetName returns the device name.
func (d ixmlDevice) GetName() (string, error) {
	name, ret := d.Device.GetName() // name example: "Iluvatar BI-V150S"
	if ret != ixml.SUCCESS {
		return "", fmt.Errorf("failed to get device name: %v", ret)
	}
	klog.Infof("success to get device name: %s", name)

	productName := strings.Split(name, " ")[1]
	return productName, nil
}

// GetTotalMemoryMB returns the total memory on a device in MB
func (d ixmlDevice) GetTotalMemoryMB() (uint64, error) {
	info, ret := d.Device.GetMemoryInfo()
	if ret != ixml.SUCCESS {
		return 0, fmt.Errorf("failed to get device memory info: %v", ret)
	}
	klog.Infof("success to get device memory: %d (MB)", info.Total)

	return info.Total, nil
}
