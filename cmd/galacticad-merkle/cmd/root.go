/*
 * Copyright 2024 Galactica Network
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Galactica-corp/merkle-proof-service/cmd/galacticad-merkle/cmd/ctx"
	"github.com/Galactica-corp/merkle-proof-service/cmd/galacticad-merkle/cmd/indexer"
	"github.com/Galactica-corp/merkle-proof-service/internal/viperhelper"
)

const (
	EnvPrefix = "GALACTICA"

	FlagHome   = "home"
	FlagConfig = "config"

	EnvHome   = "HOME"
	EnvConfig = "CONFIG"

	FlagLogLevel    = "log-level"
	EvnLogLevel     = "LOG_LEVEL"
	ViperLogLevel   = "log_level"
	DefaultLogLevel = "info"

	DefaultHomeSubDir     = ".galacticad-merkle"
	DefaultConfigFileName = "merkle.yaml"
)

type Config struct {
	Home     string
	Config   string
	LogLevel string
}

func createRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "galacticad-merkle",
		Short: "Galactica Network merkle cli",
		Long: `Galactica Network merkle cli. 
This is a CLI tool to interact with the Galactica Network merkle service.`,
		Version: Version + " (" + GitCommit + ")",
	}
}

func Execute() {
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rootCmd := initRootCmd(createRootCmd())

	if err := rootCmd.ExecuteContext(rootCtx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initRootCmd(rootCmd *cobra.Command) *cobra.Command {
	cfg := Config{}

	viper.SetEnvPrefix(EnvPrefix)
	viper.AutomaticEnv()

	// root command flags
	rootCmd.PersistentFlags().StringVar(&cfg.Home, FlagHome, "", "home directory (default is $HOME/"+DefaultHomeSubDir+")")
	rootCmd.PersistentFlags().StringVar(&cfg.Config, FlagConfig, "", "config file (default is $HOME/"+DefaultHomeSubDir+"/"+DefaultConfigFileName+")")
	rootCmd.PersistentFlags().StringVar(&cfg.LogLevel, FlagLogLevel, DefaultLogLevel, "log level, available options: [debug, info, error, none]")

	// bind flags to viper
	viperhelper.MustBindPFlag(viper.GetViper(), ViperLogLevel, rootCmd.PersistentFlags().Lookup(FlagLogLevel))

	// bind env variables to viper
	viper.MustBindEnv(FlagHome, EnvHome)
	viper.MustBindEnv(FlagConfig, EnvConfig)
	viper.MustBindEnv(FlagLogLevel, EvnLogLevel)

	// init config
	if initedCfg, err := initConfig(cfg); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		cfg = initedCfg
	}

	// init logger
	writer := log.NewSyncWriter(os.Stdout)
	logger := initLogger(cfg.LogLevel, writer)

	// set logger to root command
	rootCmd.SetOut(writer)
	rootCmd.SetErr(writer)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cmd.SetContext(context.WithValue(cmd.Context(), ctx.LoggerKey, logger))
		cmd.SetContext(context.WithValue(cmd.Context(), ctx.HomeDirKey, cfg.Home))

		logger.Info(
			"initialize galacticad-merkle service",
			"home", cfg.Home,
			"config", cfg.Config,
			"log_level", cfg.LogLevel,
		)
	}

	// add subcommands
	rootCmd.AddCommand(indexer.CreateIndexerCmd())

	return rootCmd
}

func initLogger(
	level string,
	writer io.Writer,
) log.Logger {
	logger := log.NewTMLogger(writer)
	logLevel, err := log.AllowLevel(level)
	if err != nil {
		level = DefaultLogLevel
		logLevel, err = log.AllowLevel(level)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	logger = log.NewFilter(logger, logLevel)

	return logger
}

func initConfig(cfg Config) (Config, error) {
	if home := viper.GetString(FlagHome); home != "" {
		cfg.Home = home
	} else {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return cfg, fmt.Errorf("failed to get user home directory: %w", err)
		}

		cfg.Home = filepath.Join(userHome, DefaultHomeSubDir)
	}

	if config := viper.GetString(FlagConfig); config != "" {
		cfg.Config = config
	} else {
		cfg.Config = filepath.Join(cfg.Home, DefaultConfigFileName)
	}

	viper.AddConfigPath(cfg.Home)
	viper.SetConfigFile(cfg.Config)

	if err := viper.ReadInConfig(); err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	// set log level
	cfg.LogLevel = viper.GetString(ViperLogLevel)

	return cfg, nil
}
