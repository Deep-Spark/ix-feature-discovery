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
package label

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"gitee.com/deep-spark/ix-feature-discovery/pkg/config"
	"gitee.com/deep-spark/ix-feature-discovery/pkg/resource"
)

// NewIXDeviceLabeler creates a new labeler for the specified resource manager.
func NewIXDeviceLabeler(manager resource.Manager, config *config.Config) (Labeler, error) {
	if err := manager.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize resource manager: %v", err)
	}
	defer func() {
		_ = manager.Shutdown()
	}()

	devices, err := manager.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %v", err)
	}

	if len(devices) == 0 {
		return empty{}, nil
	}

	machineTypeLabeler, err := newMachineTypeLabeler(*config.Flags.MachineTypeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to construct machine type labeler: %v", err)
	}

	versionLabeler, err := ixmlVersionLabeler(manager)
	if err != nil {
		return nil, fmt.Errorf("failed to construct version labeler: %v", err)
	}

	ixResourceLabeler, err := newIXResourceLabeler(manager)
	if err != nil {
		return nil, fmt.Errorf("error creating resource labeler: %v", err)
	}

	l := Merge(
		machineTypeLabeler,
		versionLabeler,
		ixResourceLabeler,
	)

	return l, nil
}

// ixmlVersionLabeler creates a labeler that generates the driver and runtime version labels.
func ixmlVersionLabeler(manager resource.Manager) (Labeler, error) {
	driverVersion, err := manager.GetIXDriverVersion()
	if err != nil {
		return nil, fmt.Errorf("error getting ix driver version: %v", err)
	}

	driverVersionSplit := strings.Split(driverVersion, ".")
	if len(driverVersionSplit) > 3 || len(driverVersionSplit) < 2 {
		return nil, fmt.Errorf("error getting driver version: Version \"%s\" does not match format \"X.Y[.Z]\"", driverVersion)
	}

	driverMajor := driverVersionSplit[0]
	driverMinor := driverVersionSplit[1]
	driverRev := ""
	if len(driverVersionSplit) > 2 {
		driverRev = driverVersionSplit[2]
	}

	cudaMajor, cudaMinor, err := manager.GetCudaRuntimeVersion()
	if err != nil {
		return nil, fmt.Errorf("error getting cuda driver version: %v", err)
	}

	labels := Labels{
		nodeLabelPrefix + "/ix.driver-version.full":     driverVersion,
		nodeLabelPrefix + "/ix.driver-version.major":    driverMajor,
		nodeLabelPrefix + "/ix.driver-version.minor":    driverMinor,
		nodeLabelPrefix + "/ix.driver-version.revision": driverRev,
		nodeLabelPrefix + "/cuda.runtime-version.full":  fmt.Sprintf("%d.%d", *cudaMajor, *cudaMinor),
		nodeLabelPrefix + "/cuda.runtime-version.major": fmt.Sprintf("%d", *cudaMajor),
		nodeLabelPrefix + "/cuda.runtime-version.minor": fmt.Sprintf("%d", *cudaMinor),
	}
	return labels, nil
}

// newIXResourceLabeler creates a labeler for available IX resources.
func newIXResourceLabeler(manager resource.Manager) (Labeler, error) {
	devices, err := manager.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %v", err)
	}

	// If no GPUs are detected, we return an empty labeler
	if len(devices) == 0 {
		return empty{}, nil
	}

	counts := make(map[string]int)
	memorys := make(map[string]string)
	for _, dev := range devices {
		name, err := dev.GetName()
		if err != nil {
			return nil, fmt.Errorf("error getting device name: %v", err)
		}
		memory, err := dev.GetTotalMemoryMB()
		if err != nil {
			return nil, fmt.Errorf("error getting device memory: %v", err)
		}
		klog.Infof("success to get the memory of device %s: %d (MB)", name, memory)

		counts[name]++
		memorys[name] = strconv.Itoa(int(memory))
	}

	if len(counts) > 1 {
		var names []string
		for n := range counts {
			names = append(names, n)
		}
		klog.Warningf("Multiple device types detected: %v", names)
	}

	var labelers labelerList

	for name, count := range counts {
		l := Labels{
			nodeLabelPrefix + "/gpu.product": name,
			nodeLabelPrefix + "/gpu.count":   strconv.Itoa(count),
			nodeLabelPrefix + "/gpu.memory":  memorys[name],
		}
		labelers = append(labelers, l)
	}

	labels, err := labelers.Labels()
	if err != nil {
		return nil, err
	}

	return Merge(labels), nil
}
