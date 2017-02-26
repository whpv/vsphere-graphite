package backend

import (
	"log"
        "strings"
	"github.com/marpaia/graphite-golang"
)


type Point struct {
        VCenter   	string
        ObjectType      string
        ObjectName      string
        Group		string
	Counter		string
        Instance	string
        Rollup		string
	Value     	string
        Datastore       []string
        ESXi            string
        Cluster         string
        Network         []string
	Timestamp 	int64
}


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

func (backend *Backend) SendMetrics(metrics []Point) {
	if strings.ToLower(backend.Type) == "graphite" {
                var graphiteMetrics []graphite.Metric
                for _, point := range metrics {
                        //key := "vsphere." + vcName + "." + entityName + "." + name + "." + metricName
                        key :=  "vsphere." + point.VCenter + "." + point.ObjectType + "." + point.ObjectName + "." + point.Group + "." + point.Counter + "." + point.Rollup
                        if len(point.Instance) > 0 {
				key += "." + strings.ToLower(strings.Replace(point.Instance, ".", "_", -1))
                        }
			graphiteMetrics = append(graphiteMetrics, graphite.Metric{Name: key  , Value: point.Value, Timestamp: point.Timestamp}) 
                }
        	err := backend.carbon.SendMetrics(graphiteMetrics)
                if err != nil {
                	errlog.Println("Error sending metrics (trying to reconnect): ", err)
                        backend.carbon.Connect()
                }
	}
}
