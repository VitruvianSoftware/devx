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
