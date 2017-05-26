package backend

import (
	"log"
	"strings"
	"errors"
	"time"
	"strconv"

	"github.com/marpaia/graphite-golang"
	influxclient "github.com/influxdata/influxdb/client/v2"

	"github.com/olegfedoseev/opentsdb"
	"bytes"
	"github.com/pquerna/ffjson/ffjson"
	"compress/gzip"
	"net/http"
	"fmt"
	//"encoding/json"
)

type FinderStuct struct {
	Host  string
	Infos []FinderInfo
}

type FinderInfo struct {
	Path  string
	Name  string
	Type  string
	Value string
}

type Point struct {
	VCenter    string
	ObjectType string
	ObjectName string
	Group      string
	Counter    string
	Instance   string
	Rollup     string
	Value      int64
	Datastore  []string
	ESXi       string
	Cluster    string
	Network    []string
	Timestamp  int64
}

//Storage backend
type Backend struct {
	ApiKey    string
	MetricUrl string
	FinderUrl string
	Hostname  string
	Port      int
	Database  string
	Username  string
	Password  string
	Type      string
	NoArray   bool
	carbon    *graphite.Graphite
	influx    influxclient.Client
	opentsdb  *opentsdb.Client
}

var stdlog, errlog *log.Logger
var carbon graphite.Graphite

func (backend *Backend) Init(standardLogs *log.Logger, errorLogs *log.Logger) error {
	stdlog = standardLogs
	errlog = errorLogs
	switch backendType := strings.ToLower(backend.Type); backendType {
	case "graphite":
		// Initialize Graphite
		stdlog.Println("Intializing " + backendType + " backend")
		carbon, err := graphite.NewGraphite(backend.Hostname, backend.Port)
		if err != nil {
			errlog.Println("Error connecting to graphite")
			return err
		}
		backend.carbon = carbon
		return nil
	case "influxdb":
		//Initialize Influx DB
		stdlog.Println("Intializing " + backendType + " backend")
		influxclt, err := influxclient.NewHTTPClient(influxclient.HTTPConfig{
			Addr:     "http://" + backend.Hostname + ":" + strconv.Itoa(backend.Port),
			Username: backend.Username,
			Password: backend.Password,
		})
		if err != nil {
			errlog.Println("Error connecting to InfluxDB")
			return err
		}
		backend.influx = influxclt
		return nil
	case "opentsdb":
		c, err := opentsdb.NewClient(backend.Hostname, 1, 10*time.Second)
		if err != nil {
			errlog.Println("Error connecting to opentsdb")
			return err
		}
		backend.opentsdb = c
		return nil
	case "kong":
		return nil
	default:
		errlog.Println("Backend " + backendType + " unknown.")
		return errors.New("Backend " + backendType + " unknown.")
	}
}

func (backend *Backend) Disconnect() {

	switch backendType := strings.ToLower(backend.Type); backendType {
	case "graphite":
		// Disconnect from graphite
		stdlog.Println("Disconnecting from " + backendType)
		backend.carbon.Disconnect()
	case "influxdb":
		// Disconnect from influxdb
		stdlog.Println("Disconnecting from " + backendType)
		backend.influx.Close()
	case "opentsdb":
		stdlog.Println("Disconnecting from " + backendType)
	case "kong":

	default:
		errlog.Println("Backend " + backendType + " unknown.")
	}
}

func (backend *Backend) SendMetrics(metrics []Point) {
	switch backendType := strings.ToLower(backend.Type); backendType {
	case "opentsdb":
		var tsdbMetrics opentsdb.DataPoints
		for _, point := range metrics {
			//key := "vsphere." + vcName + "." + entityName + "." + name + "." + metricName
			/*key :=  "vsphere." + point.VCenter + "." + point.ObjectType + "." + point.ObjectName + "." + point.Group + "." + point.Counter + "." + point.Rollup
			if len(point.Instance) > 0 {
				key += "." + strings.ToLower(strings.Replace(point.Instance, ".", "_", -1))
			}*/

			tags := opentsdb.Tags{}

			tags["host"] = point.VCenter
			tags[point.ObjectType] = point.ObjectName
			if len(point.Instance) > 0 {
				tags["instance"] = strings.ToLower(strings.Replace(point.Instance, ".", "_", -1))
			}

			tsdbMetrics = append(tsdbMetrics, &opentsdb.DataPoint{
				Metric:    point.Group + "." + point.Counter + "." + point.Rollup,
				Value:     strconv.FormatInt(point.Value, 10),
				Timestamp: point.Timestamp,
				Tags:      tags})
		}
		//b, _:= json.Marshal(tsdbMetrics)
		//fmt.Println(string(b))
		postman := opentsdb.NewPostman(10 * time.Second)
		backend.opentsdb.Send(postman, tsdbMetrics)
		postman = nil
		tsdbMetrics = nil
		//err := backend.carbon.SendMetrics(graphiteMetrics)
		//if err != nil {
		//	errlog.Println("Error sending metrics (trying to reconnect): ", err)
		//	backend.carbon.Connect()
		//}
	case "graphite":
		var graphiteMetrics []graphite.Metric
		for _, point := range metrics {
			//key := "vsphere." + vcName + "." + entityName + "." + name + "." + metricName
			key := "vsphere." + point.VCenter + "." + point.ObjectType + "." + point.ObjectName + "." + point.Group + "." + point.Counter + "." + point.Rollup
			if len(point.Instance) > 0 {
				key += "." + strings.ToLower(strings.Replace(point.Instance, ".", "_", -1))
			}
			graphiteMetrics = append(graphiteMetrics, graphite.Metric{Name: key, Value: strconv.FormatInt(point.Value, 10), Timestamp: point.Timestamp})
		}
		err := backend.carbon.SendMetrics(graphiteMetrics)
		if err != nil {
			errlog.Println("Error sending metrics (trying to reconnect): ", err)
			backend.carbon.Connect()
		}
	case "influxdb":
		//Influx batch points
		bp, err := influxclient.NewBatchPoints(influxclient.BatchPointsConfig{
			Database:  backend.Database,
			Precision: "s",
		})
		if err != nil {
			errlog.Println("Error creating influx batchpoint")
			errlog.Println(err)
			return
		}
		for _, point := range metrics {
			key := point.Group + "_" + point.Counter + "_" + point.Rollup
			tags := map[string]string{}
			tags["vcenter"] = point.VCenter
			tags["type"] = point.ObjectType
			tags["name"] = point.ObjectName
			if backend.NoArray {
				if len(point.Datastore) > 0 {
					tags["datastore"] = point.Datastore[0]
				} else {
					tags["datastore"] = ""
				}
			} else {
				tags["datastore"] = strings.Join(point.Datastore, "\\,")
			}
			if backend.NoArray {
				if len(point.Network) > 0 {
					tags["network"] = point.Network[0]
				} else {
					tags["network"] = ""
				}
			} else {
				tags["network"] = strings.Join(point.Network, "\\,")
			}
			tags["host"] = point.ESXi
			tags["cluster"] = point.Cluster
			tags["instance"] = point.Instance
			fields := make(map[string]interface{})
			fields["Value"] = point.Value
			pt, err := influxclient.NewPoint(key, tags, fields, time.Unix(point.Timestamp, 0))
			if err != nil {
				errlog.Println("Could not create influxdb point")
				errlog.Println(err)
				continue
			}
			bp.AddPoint(pt)
		}
		err = backend.influx.Write(bp)
		if err != nil {
			errlog.Println("Error sending metrics: ", err)
		}
	case "kong":
		var tsdbMetrics opentsdb.DataPoints
		var host string
		for _, point := range metrics {
			//key := "vsphere." + vcName + "." + entityName + "." + name + "." + metricName
			/*key :=  "vsphere." + point.VCenter + "." + point.ObjectType + "." + point.ObjectName + "." + point.Group + "." + point.Counter + "." + point.Rollup
			if len(point.Instance) > 0 {
				key += "." + strings.ToLower(strings.Replace(point.Instance, ".", "_", -1))
			}*/

			tags := opentsdb.Tags{}
			if host == "" {
				host = point.VCenter
			}

			tags["host"] = point.VCenter
			tags[point.ObjectType] = point.ObjectName
			if len(point.Instance) > 0 {
				tags["instance"] = strings.ToLower(strings.Replace(point.Instance, ".", "_", -1))
			}

			tsdbMetrics = append(tsdbMetrics, &opentsdb.DataPoint{
				Metric:    point.Group + "." + point.Counter + "." + point.Rollup,
				Value:     strconv.FormatInt(point.Value, 10),
				Timestamp: point.Timestamp,
				Tags:      tags})
		}
		//b, _:= json.Marshal(tsdbMetrics)
		//fmt.Println(string(b))

		url := fmt.Sprintf("%s?api_key=%s&host=%s", backend.MetricUrl, backend.ApiKey, host)
		backend.SendNetrics2tsdb(tsdbMetrics, url)

		tsdbMetrics = nil
		//err := backend.carbon.SendMetrics(graphiteMetrics)
		//if err != nil {
		//	errlog.Println("Error sending metrics (trying to reconnect): ", err)
		//	backend.carbon.Connect()
		//}

	default:
		errlog.Println("Backend " + backendType + " unknown.")
	}
}

func (backend *Backend) SendNetrics2tsdb(values opentsdb.DataPoints, url string) (error) {
	var buffer bytes.Buffer

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}
	writer := gzip.NewWriter(&buffer)

	if err := ffjson.NewEncoder(writer).Encode(values); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, &buffer)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (backend *Backend) SendFinder(values []FinderInfo, url string) (error) {
	var buffer bytes.Buffer

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}
	writer := gzip.NewWriter(&buffer)

	if err := ffjson.NewEncoder(writer).Encode(values); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, &buffer)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}
