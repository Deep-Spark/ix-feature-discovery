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
	"os"
	"regexp"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"gitee.com/deep-spark/ix-feature-discovery/pkg/config"
	"gitee.com/deep-spark/ix-feature-discovery/pkg/resource"
)

// Labels defines a type for labels
type Labels map[string]string

// Labels method returns the labels as is, implementing the Labeler interface
func (labels Labels) Labels() (Labels, error) {
	return labels, nil
}

// empty represents an empty set of labels
type empty struct{}

// Labels method returns an empty set of labels, implementing the Labeler interface
func (manager empty) Labels() (Labels, error) {
	return nil, nil
}

// Labeler defines an interface for generating labels
type Labeler interface {
	Labels() (Labels, error)
}

// labelerList represents a list of labelers that itself implements the Labeler interface.
type labelerList []Labeler

// Merge converts a set of labelers to a single composite labeler.
func Merge(labelers ...Labeler) Labeler {
	list := labelerList(labelers)

	return list
}

// Labels method returns the labels from a set of labelers. Labels later in the list
// overwrite earlier labels.
func (labelers labelerList) Labels() (Labels, error) {
	allLabels := make(Labels)
	for _, labeler := range labelers {
		labels, err := labeler.Labels()
		if err != nil {
			return nil, fmt.Errorf("error generating labels: %v", err)
		}
		for k, v := range labels {
			allLabels[k] = v
		}
	}

	return allLabels, nil
}

// NewLabelers constructs the required labelers from the specified config
func NewLabelers(manager resource.Manager, config *config.Config) (Labeler, error) {
	deviceLabeler, err := NewIXDeviceLabeler(manager, config)
	if err != nil {
		return nil, fmt.Errorf("error creating labeler: %v", err)
	}

	return deviceLabeler, nil
}

// NewTimestampLabeler creates a new label manager for generating timestamp.
// If the noTimestamp option is set an empty label manager is returned.
func NewTimestampLabeler(config *config.Config) Labeler {
	if *config.Flags.NoTimestamp {
		return empty{}
	}

	return Labels{
		nodeLabelPrefix + "/ix.timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}
}

// newMachineTypeLabeler creates a new labeler for machine type based on the provided path
func newMachineTypeLabeler(machineTypePath string) (Labeler, error) {
	machineType, err := getMachineType(machineTypePath)
	if err != nil {
		klog.Warningf("Error getting machine type from %v: %v", machineTypePath, err)
		machineType = machineTypeUnknown
	}

	machineType = sanitise(machineType)
	klog.Infof("Successfully got machine type: %s", machineType)

	l := Labels{
		nodeLabelPrefix + "/gpu.machine": machineType,
	}

	return l, nil
}

// getMachineType reads the machine type from the specified path
func getMachineType(path string) (string, error) {
	if path == "" {
		return machineTypeUnknown, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not open machine type file: %v", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// sanitise removes any non-alphanumeric characters and extra spaces from the input string
func sanitise(input string) string {
	var sanitised string
	re := regexp.MustCompile("[^A-Za-z0-9-_. ]")
	input = re.ReplaceAllString(input, "")
	// Remove redundant blank spaces
	sanitised = strings.Join(strings.Fields(input), "-")

	return sanitised
}
