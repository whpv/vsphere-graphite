package config

import (
        "github.com/whpv/vsphere-graphite/vsphere"
        "github.com/whpv/vsphere-graphite/backend"
)

// Configuration
type Configuration struct {
	Debug    bool
	VCenters []*vsphere.VCenter
	Metrics  []vsphere.Metric
	Interval int
	Domain   string
        Backend  backend.Backend
}
