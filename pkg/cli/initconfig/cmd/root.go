/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var rootCmd = &cobra.Command{
	Use:   "config",
	Short: "init config for zadig",
	Long:  `init system config  for zadig`,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()

	log.Init(&log.Config{
		Level:    config.LogLevel(),
		NoCaller: true,
	})
}
