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
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclientset "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"

	"gitee.com/deep-spark/ix-feature-discovery/pkg/config"
)

// Outputer defines a mechanism to output labels.
type Outputer interface {
	Output(Labels) error
}

type NodeFeatureOutputer struct {
	nodeConfig   config.NodeConfig
	nfdClientSet nfdclientset.Interface
}

// NewOutputer creates a NodeFeatureOutputer.
func NewOutputer(config *config.Config, nodeConfig config.NodeConfig, clientSets config.ClientSets) (Outputer, error) {
	if nodeConfig.Name == "" {
		return nil, fmt.Errorf("required flag node-name not set")
	}
	if nodeConfig.Namespace == "" {
		return nil, fmt.Errorf("required flag namespace not set")
	}
	out := NodeFeatureOutputer{
		nodeConfig:   nodeConfig,
		nfdClientSet: clientSets.NFD,
	}
	return &out, nil
}

// Output creates or updates the node-specific NodeFeature custom resource.
func (n *NodeFeatureOutputer) Output(labels Labels) error {
	nodename := n.nodeConfig.Name
	if nodename == "" {
		return fmt.Errorf("required flag %q not set", "node-name")
	}
	namespace := n.nodeConfig.Namespace
	nodeFeatureName := strings.Join([]string{nodeFeaturePrefix, nodename}, "-")

	if nfr, err := n.nfdClientSet.NfdV1alpha1().NodeFeatures(namespace).Get(context.TODO(), nodeFeatureName, metav1.GetOptions{}); errors.IsNotFound(err) {
		klog.Infof("Creating NodeFeature object %s in namespace %s", nodeFeatureName, namespace)
		nfr = &nfdv1alpha1.NodeFeature{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{Name: nodeFeatureName, Labels: map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename}},
			Spec:       nfdv1alpha1.NodeFeatureSpec{Features: *nfdv1alpha1.NewFeatures(), Labels: labels},
		}
		nfrCreated, err := n.nfdClientSet.NfdV1alpha1().NodeFeatures(namespace).Create(context.TODO(), nfr, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create NodeFeature object %q: %w", nfr.Name, err)
		}
		klog.Infof("NodeFeature object %s created successfully: %v", nfrCreated.Name, nfrCreated)
	} else if err != nil {
		return fmt.Errorf("failed to get NodeFeature object %s: %w", nodeFeatureName, err)
	} else {
		nfrUpdated := nfr.DeepCopy()
		nfrUpdated.Labels = map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename}
		nfrUpdated.Spec = nfdv1alpha1.NodeFeatureSpec{Features: *nfdv1alpha1.NewFeatures(), Labels: labels}

		if !equality.Semantic.DeepEqual(nfr, nfrUpdated) {
			klog.Infof("Updating NodeFeature object %s in namespace %s", nodeFeatureName, namespace)
			nfrUpdated, err = n.nfdClientSet.NfdV1alpha1().NodeFeatures(namespace).Update(context.TODO(), nfrUpdated, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update NodeFeature object %q: %w", nfr.Name, err)
			}
			klog.Infof("NodeFeature object %s updated successfully: %v", nfrUpdated.Name, nfrUpdated)
		} else {
			klog.Infof("No changes detected in NodeFeature object %s, skipping update", nodeFeatureName)
		}
	}
	return nil
}
