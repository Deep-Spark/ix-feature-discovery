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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"gitee.com/deep-spark/ix-feature-discovery/pkg/config"
	"gitee.com/deep-spark/ix-feature-discovery/pkg/label"
	"gitee.com/deep-spark/ix-feature-discovery/pkg/resource"
	"gitee.com/deep-spark/ix-feature-discovery/pkg/utils"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

// Config represents a collection of config options for ix-feature-discovery.
type Config struct {
	kubeClientConfig config.KubeClientConfig
	nodeConfig       config.NodeConfig

	// flags stores the CLI flags for later processing.
	flags []cli.Flag
}

func main() {
	config := &Config{}

	app := cli.NewApp()
	app.Name = "IX Feature Discovery"
	app.Usage = "generate node labels for iluvatar corex gpu devices"
	app.Action = func(ctx *cli.Context) error {
		return start(ctx, config)
	}

	config.flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "no-timestamp",
			Value:   false,
			Usage:   "Do not add the timestamp to the labels",
			EnvVars: []string{"NO_TIMESTAMP"},
		},
		&cli.DurationFlag{
			Name:    "sleep-interval",
			Value:   60 * time.Second,
			Usage:   "Time to sleep between labeling",
			EnvVars: []string{"SLEEP_INTERVAL"},
		},
		&cli.StringFlag{
			Name:    "output-file",
			Aliases: []string{"output", "o"},
			Value:   "/etc/kubernetes/node-feature-discovery/features.d/ix-features",
			EnvVars: []string{"OUTPUT_FILE"},
		},
		&cli.StringFlag{
			Name:    "machine-type-file",
			Value:   "/sys/class/dmi/id/product_name",
			Usage:   "a path to a file that contains the DMI (SMBIOS) information for the node",
			EnvVars: []string{"MACHINE_TYPE_FILE"},
		},
	}

	config.flags = append(config.flags, config.kubeClientConfig.Flags()...)
	config.flags = append(config.flags, config.nodeConfig.Flags()...)

	app.Flags = config.flags

	if err := app.Run(os.Args); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

// loadConfig loads the config from the spec file.
func (cfg *Config) loadConfig(ctx *cli.Context) (*config.Config, error) {
	conf, err := config.NewConfig(ctx, cfg.flags)
	if err != nil {
		return nil, fmt.Errorf("unable to finalize config: %v", err)
	}
	return conf, nil
}

func start(ctx *cli.Context, cfg *Config) error {
	defer func() {
		klog.Info("Exiting")
	}()

	klog.Info("Starting OS watcher.")
	sigs := utils.Signals(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	for {
		// Load the configuration file
		klog.Info("Loading configuration.")
		config, err := cfg.loadConfig(ctx)
		if err != nil {
			return fmt.Errorf("unable to load config: %v", err)
		}
		// Print the config to the output.
		configJSON, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %v", err)
		}
		klog.Infof("\nRunning with config:\n%v", string(configJSON))

		manager := resource.NewIXMLManager()

		clientSets, err := cfg.kubeClientConfig.NewClientSets()
		if err != nil {
			return fmt.Errorf("failed to create clientsets: %w", err)
		}

		labelOutputer, err := label.NewOutputer(
			config,
			cfg.nodeConfig,
			clientSets,
		)
		if err != nil {
			return fmt.Errorf("failed to create label outputer: %w", err)
		}

		klog.Info("Start running")
		d := &ixfd{
			manager:       manager,
			config:        config,
			labelOutputer: labelOutputer,
		}
		restart, err := d.run(sigs)
		if err != nil {
			return err
		}

		if !restart {
			return nil
		}
	}
}

type ixfd struct {
	manager       resource.Manager
	config        *config.Config
	labelOutputer label.Outputer
}

func (d *ixfd) run(sigs chan os.Signal) (restart bool, err error) {
	defer func() {
		if d.config.Flags.OutputFile != nil && *d.config.Flags.OutputFile == "" {
			return
		}
		err := removeOutputFile(*d.config.Flags.OutputFile)
		if err != nil {
			klog.Warningf("Error removing output file: %v", err)
		}
	}()

	timestampLabeler := label.NewTimestampLabeler(d.config)
rerun:
	loopLabelers, err := label.NewLabelers(d.manager, d.config)
	if err != nil {
		return false, err
	}

	labelers := label.Merge(
		timestampLabeler,
		loopLabelers,
	)

	labels, err := labelers.Labels()
	if err != nil {
		return false, fmt.Errorf("error generating labels: %v", err)
	}

	if len(labels) <= 1 {
		klog.Warning("No labels generated from any source")
	}

	klog.Info("Creating Labels")
	if err := d.labelOutputer.Output(labels); err != nil {
		return false, err
	}

	klog.Info("Sleeping ", time.Duration(*d.config.Flags.SleepInterval).String())
	rerunTimeout := time.After(time.Duration(*d.config.Flags.SleepInterval))

	for {
		select {
		case <-rerunTimeout:
			goto rerun

		// Watch for any signals from the OS. On SIGHUP trigger a reload of the config.
		// On all other signals, exit the loop and exit the program.
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				klog.Info("Received SIGHUP, restarting.")
				return true, nil
			default:
				klog.Infof("Received signal %v, shutting down.", s)
				return false, nil
			}
		}
	}
}

func removeOutputFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to retrieve absolute path of output file: %v", err)
	}

	err = os.Remove(absPath)
	if err != nil {
		return fmt.Errorf("failed to remove output file: %v", err)
	}

	return nil
}
