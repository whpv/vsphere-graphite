package config

import (
        "github.com/cblomart/vsphere-graphite/vsphere"
        "github.com/cblomart/vsphere-graphite/backend"
)

// Configuration
type Configuration struct {
	VCenters []*vsphere.VCenter
	Metrics  []vsphere.Metric
	Interval int
	Domain   string
        Backend  backend.Backend
}
