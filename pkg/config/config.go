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
package config

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

type Config struct {
	Flags *Flags `json:"flags,omitempty"     static:"flags,omitempty"`
}

func NewConfig(c *cli.Context, flags []cli.Flag) (*Config, error) {
	config := &Config{}

	if config.Flags == nil {
		config.Flags = &Flags{}
	}
	config.Flags.UpdateFromCLIFlags(c, flags)
	return config, nil
}

// Flags holds the full list of flags used to configure the ix-feature-discovery.
type Flags struct {
	NoTimestamp     *bool     `json:"noTimestamp"     static:"noTimestamp"`
	SleepInterval   *Duration `json:"sleepInterval"   static:"sleepInterval"`
	OutputFile      *string   `json:"outputFile"      static:"outputFile"`
	MachineTypeFile *string   `json:"machineTypeFile" static:"machineTypeFile"`
}

// UpdateFromCLIFlags updates Flags from settings in the cli Flags if they are set.
func (f *Flags) UpdateFromCLIFlags(c *cli.Context, flags []cli.Flag) {
	for _, flag := range flags {
		for _, n := range flag.Names() {
			switch n {
			case "output-file":
				updateFromCLIFlag(&f.OutputFile, c, n)
			case "sleep-interval":
				updateFromCLIFlag(&f.SleepInterval, c, n)
			case "no-timestamp":
				updateFromCLIFlag(&f.NoTimestamp, c, n)
			case "machine-type-file":
				updateFromCLIFlag(&f.MachineTypeFile, c, n)
			}
		}
	}
}

// prt returns a reference to whatever type is passed into it
func ptr[T any](x T) *T {
	return &x
}

// updateFromCLIFlag conditionally updates the config flag at 'pflag' to the value of the CLI flag with name 'flagName'
func updateFromCLIFlag[T any](pflag **T, c *cli.Context, flagName string) {
	if c.IsSet(flagName) || *pflag == (*T)(nil) {
		switch flag := any(pflag).(type) {
		case **string:
			*flag = ptr(c.String(flagName))
		case **[]string:
			*flag = ptr(c.StringSlice(flagName))
		case **bool:
			*flag = ptr(c.Bool(flagName))
		case **Duration:
			*flag = ptr(Duration(c.Duration(flagName)))
		default:
			panic(fmt.Errorf("unsupported flag type for %v: %T", flagName, flag))
		}
	}
}
