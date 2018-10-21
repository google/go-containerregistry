// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

const cmdName = "ko"

type ExtensionHandler interface {
	Lookup(filename string) (string, error)
	Execute(executablePath string, cmdArgs, environment []string) error
}

type defaultExtensionHandler struct{}

func (h *defaultExtensionHandler) Lookup(filename string) (string, error) {
	// if on Windows, append the "exe" extension
	// to the filename that we are looking up.
	if runtime.GOOS == "windows" {
		filename = filename + ".exe"
	}
	return exec.LookPath(filename)
}

func (h *defaultExtensionHandler) Execute(executablePath string, cmdArgs, environment []string) error {
	return syscall.Exec(executablePath, cmdArgs, environment)
}

func handleExtensions(handler ExtensionHandler, env, cmdArgs []string) error {
	remainingArgs := []string(nil)

	for idx := range cmdArgs {
		if strings.HasPrefix(cmdArgs[idx], "-") {
			break
		}
		remainingArgs = append(remainingArgs, strings.Replace(cmdArgs[idx], "-", "_", -1))
	}

	foundBinaryPath := ""

	for len(remainingArgs) > 0 {
		path, err := handler.Lookup(fmt.Sprintf("%s-%s", cmdName, strings.Join(remainingArgs, "-")))
		if err != nil || len(path) == 0 {
			remainingArgs = remainingArgs[:len(remainingArgs)-1]
			continue
		}

		foundBinaryPath = path
		break
	}

	if len(foundBinaryPath) == 0 {
		return nil
	}

	if err := handler.Execute(foundBinaryPath, append([]string{foundBinaryPath}, cmdArgs[len(remainingArgs):]...), env); err != nil {
		return err
	}

	return nil
}
