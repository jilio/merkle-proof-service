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
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// AppName is the name of the service
	AppName = "merkle-proof-service"

	// Version is the version of the service
	Version = ""

	// GitCommit is the git commit of the service
	GitCommit = ""

	// GoVersion is the go version of the service
	GoVersion = fmt.Sprintf("go version %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// BuildDeps is the build dependencies of the service
	BuildDeps = depsFromBuildInfo()
)

func depsFromBuildInfo() (deps []buildDep) {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}

	for _, dep := range buildInfo.Deps {
		deps = append(deps, buildDep{dep})
	}

	return
}

type buildDep struct {
	*debug.Module
}

func (d buildDep) String() string {
	if d.Replace != nil {
		return fmt.Sprintf("%s@%s => %s@%s", d.Path, d.Version, d.Replace.Path, d.Replace.Version)
	}

	return fmt.Sprintf("%s@%s", d.Path, d.Version)
}

func (d buildDep) MarshalJSON() ([]byte, error)      { return json.Marshal(d.String()) }
func (d buildDep) MarshalYAML() (interface{}, error) { return d.String(), nil }

func fullVersion() string {
	// print the full version information including the build deps
	lines := []string{
		fmt.Sprintf("%s: %s", AppName, Version),
		fmt.Sprintf("git commit: %s", GitCommit),
		GoVersion,
	}

	lines = append(lines, "build deps:")
	for _, dep := range BuildDeps {
		lines = append(lines, "  - "+dep.String())
	}

	return strings.Join(lines, "\n")
}

// CreateVersion creates the version command
func CreateVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of the service",
	}

	// add --long flag
	cmd.Flags().Bool("long", false, "Print the full version information including the build deps")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		// print the version information
		if cmd.Flag("long").Value.String() == "true" {
			// print the full version information including the build deps
			fmt.Println(fullVersion())
		} else {
			// print the short version information
			fmt.Println(Version)
		}
	}

	return cmd
}
