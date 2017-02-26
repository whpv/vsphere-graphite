package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

        "github.com/cblomart/vsphere-graphite/vsphere"
        "github.com/cblomart/vsphere-graphite/config"
        "github.com/cblomart/vsphere-graphite/backend"

	"github.com/takama/daemon"

	"github.com/vmware/govmomi/vim25/types"
)

const (
	// name of the service
	name        = "vsphere-graphite"
	description = "send vsphere stats to graphite"
)

var dependencies = []string{}

var stdlog, errlog *log.Logger

// Service has embedded daemon
type Service struct {
	daemon.Daemon
}

// Informations to query about an entity
type EntityQuery struct {
	Name    string
	Entity  types.ManagedObjectReference
	Metrics []int
}

func queryVCenter(vcenter vsphere.VCenter, config config.Configuration, channel *chan []backend.Point) {
	vcenter.Query(config.Interval, config.Domain, channel)
}

// Manage by daemon commands or run the daemon
func (service *Service) Manage() (string, error) {

	usage := "Usage: myservice install | remove | start | stop | status"

	// if received any kind of command, do it
	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "install":
			return service.Install()
		case "remove":
			return service.Remove()
		case "start":
			return service.Start()
		case "stop":
			return service.Stop()
		case "status":
			return service.Status()
		default:
			return usage, nil
		}
	}

	stdlog.Println("Starting daemon:", path.Base(os.Args[0]))

	// read the configuration
	file, err := os.Open("/etc/" + path.Base(os.Args[0]) + ".json")
	if err != nil {
		return "Could not open configuration file", err
	}
	jsondec := json.NewDecoder(file)
	config := config.Configuration{}
	err = jsondec.Decode(&config)
	if err != nil {
		return "Could not decode configuration file", err
	}

	for _, vcenter := range config.VCenters {
		vcenter.Init(config.Metrics, stdlog, errlog)
	}

	err = config.Backend.Init(stdlog, errlog)
        if err != nil {
		return "Could not initialize backend", err
        }
        defer config.Backend.Disconnect()


	// Initialize Graphite
	//carbon, err := graphite.NewGraphite(config.Graphite.Hostname, config.Graphite.Port)
	//if err != nil {
	//	return "Could not initialize graphite", err
	//}
	//defer carbon.Disconnect()

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	// Set up a channel to recieve the metrics
	metrics := make(chan []backend.Point)

	// Set up a ticker to collect metrics at givent interval
	ticker := time.NewTicker(time.Second * time.Duration(config.Interval))
	defer ticker.Stop()

	// Start retriveing and sending metrics
	stdlog.Println("Retrieving metrics")
	for _, vcenter := range config.VCenters {
		go queryVCenter(*vcenter, config, &metrics)
	}
	for {
		select {
		case values := <-metrics:
			config.Backend.SendMetrics(values)
			stdlog.Printf("Sent %d logs to backend", len(values))
		case <-ticker.C:
			stdlog.Println("Retrieving metrics")
			for _, vcenter := range config.VCenters {
				go queryVCenter(*vcenter, config, &metrics)
			}
		case killSignal := <-interrupt:
			stdlog.Println("Got signal:", killSignal)
			if killSignal == os.Interrupt {
				return "Daemon was interruped by system signal", nil
			}
			return "Daemon was killed", nil
		}
	}

	// never happen, but need to complete code
	return usage, nil
}

func init() {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime)
}

func main() {
	srv, err := daemon.New(name, description, dependencies...)
	if err != nil {
		errlog.Println("Error: ", err)
		os.Exit(1)
	}
	service := &Service{srv}
	status, err := service.Manage()
	if err != nil {
		errlog.Println(status, "\nError: ", err)
		os.Exit(1)
	}
	fmt.Println(status)
}
