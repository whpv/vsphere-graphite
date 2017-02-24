package backend

import (
	"log"
        "strings"
	"github.com/marpaia/graphite-golang"
)


//Storage backend
type Backend struct {
        Hostname string
        Port     int
        Database string
        Username string
        Password string
        Type     string
        carbon   *graphite.Graphite
}

var	stdlog, errlog *log.Logger
var	carbon graphite.Graphite

func (backend *Backend) Init(standardLogs *log.Logger, errorLogs *log.Logger)  error {
        stdlog := standardLogs
        stderr := errorLogs
        if strings.ToLower(backend.Type) == "graphite" {
	        stdlog.Println("Initializing graphite backend")
	        // Initialize Graphite
        	carbon, err := graphite.NewGraphite(backend.Hostname, backend.Port)
        	if err != nil {
                        stderr.Println("Error connecting to graphite")
                	return err
        	}
		backend.carbon = carbon
        }
	return nil
}

func (backend *Backend) Disconnect() {
        if strings.ToLower(backend.Type) == "graphite" {
		stdlog.Println("Disconnecting from graphite")
		backend.carbon.Disconnect()
        }
}

func (backend *Backend) SendMetrics(metrics []graphite.Metric) {
	if strings.ToLower(backend.Type) == "graphite" {
        	err := backend.carbon.SendMetrics(metrics)
                if err != nil {
                	errlog.Println("Error sending metrics (trying to reconnect): ", err)
                        backend.carbon.Connect()
                }
	}
}
