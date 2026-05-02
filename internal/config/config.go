// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package config

import "fmt"

// Config holds all runtime configuration derived from secrets + defaults.
type Config struct {
	DevName     string
	DevHostname string
	CFDomain    string
	TunnelName  string
	TunnelToken string
	TunnelUUID  string
}

func New(devName, hostname, tunnelName, domain string) *Config {
	if hostname == "" {
		hostname = devName + "-dev-machine"
	}
	if domain == "" {
		domain = devName + ".ipv1337.dev"
	}
	if tunnelName == "" {
		tunnelName = "dev-tunnel-" + devName
	}
	return &Config{
		DevName:     devName,
		DevHostname: hostname,
		CFDomain:    domain,
		TunnelName:  tunnelName,
	}
}

func (c *Config) Validate() error {
	if c.DevName == "" {
		return fmt.Errorf("DEV_NAME is required")
	}
	return nil
}
