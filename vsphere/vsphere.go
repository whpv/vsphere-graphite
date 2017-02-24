package vsphere

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

        "github.com/cblomart/vsphere-graphite/utils"

	"golang.org/x/net/context"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

        "github.com/marpaia/graphite-golang"
)

var stdlog, errlog *log.Logger

// VCenter description
type VCenter struct {
	Hostname     string
	Username     string
	Password     string
	MetricGroups []*MetricGroup
}

// Metric Definition
type MetricDef struct {
	Metric    string
	Instances string
	Key       int32
}

// Metric Grouping for retrieval
type MetricGroup struct {
	ObjectType string
	Metrics    []MetricDef
	Mor        []types.ManagedObjectReference
}

// Metrics description in config
type Metric struct {
        ObjectType []string
        Definition []MetricDef
}

func (vcenter *VCenter) Connect() (*govmomi.Client, error) {
	// Prepare vCenter Connections
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdlog.Println("connecting to vcenter: " + vcenter.Hostname)
	u, err := url.Parse("https://" + vcenter.Username + ":" + vcenter.Password + "@" + vcenter.Hostname + "/sdk")
	if err != nil {
		errlog.Println("Could not parse vcenter url: ", vcenter.Hostname)
		errlog.Println("Error: ", err)
		return nil, err
	}
	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		errlog.Println("Could not connect to vcenter: ", vcenter.Hostname)
		errlog.Println("Error: ", err)
		return nil, err
	}
	return client, nil
}

// Initialise vcenter
func (vcenter *VCenter) Init(metrics []Metric, standardLogs *log.Logger, errorLogs *log.Logger) {
        stdlog = standardLogs
        errlog = errorLogs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := vcenter.Connect()
	defer client.Logout(ctx)
	if err != nil {
		errlog.Println("Could not connect to vcenter: ", vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}
	var perfmanager mo.PerformanceManager
	err = client.RetrieveOne(ctx, *client.ServiceContent.PerfManager, nil, &perfmanager)
	if err != nil {
		errlog.Println("Could not get performance manager")
		errlog.Println("Error: ", err)
		return
	}
	for _, perf := range perfmanager.PerfCounter {
		groupinfo := perf.GroupInfo.GetElementDescription()
		nameinfo := perf.NameInfo.GetElementDescription()
		identifier := groupinfo.Key + "." + nameinfo.Key + "." + fmt.Sprint(perf.RollupType)
		for _, metric := range metrics {
			for _, metricdef := range metric.Definition {
				if metricdef.Metric == identifier {
					metricd := MetricDef{Metric: metricdef.Metric, Instances: metricdef.Instances, Key: perf.Key}
					for _, mtype := range metric.ObjectType {
						added := false
						for _, metricgroup := range vcenter.MetricGroups {
							if metricgroup.ObjectType == mtype {
								metricgroup.Metrics = append(metricgroup.Metrics, metricd)
								stdlog.Println("Appended metric " + metricd.Metric + " identified by " + strconv.FormatInt(int64(metricd.Key), 10) + " to vcenter " + vcenter.Hostname + " for " + mtype)
								added = true
								break
							}
						}
						if added == false {
							metricgroup := MetricGroup{ObjectType: mtype, Metrics: []MetricDef{metricd}}
							vcenter.MetricGroups = append(vcenter.MetricGroups, &metricgroup)
							stdlog.Println("Appended metric group with " + metricd.Metric + " identified by " + strconv.FormatInt(int64(metricd.Key), 10) + " to vcenter " + vcenter.Hostname + " for " + mtype)
						}
					}
				}
			}
		}
	}
}


// Query a vcenter
func (vcenter *VCenter) Query(interval int, domain string, channel *chan []graphite.Metric) {
	stdlog.Println("Setting up query inventory of vcenter: ", vcenter.Hostname)

	// Create the contect
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the client
	client, err := vcenter.Connect()
	if err != nil {
		errlog.Println("Could not connect to vcenter: ", vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

        // wait to be properly connected to defer logout
	defer client.Logout(ctx)


	// Create the view manager
	var viewManager mo.ViewManager
	err = client.RetrieveOne(ctx, *client.ServiceContent.ViewManager, nil, &viewManager)
	if err != nil {
		errlog.Println("Could not get view manager from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	// Get the Datacenters from root folder
	var rootFolder mo.Folder
	err = client.RetrieveOne(ctx, client.ServiceContent.RootFolder, nil, &rootFolder)
	if err != nil {
		errlog.Println("Could not get root folder from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	datacenters := []types.ManagedObjectReference{}
	for _, child := range rootFolder.ChildEntity {
		if child.Type == "Datacenter" {
			datacenters = append(datacenters, child)
		}
	}

	// Get intresting object types from specified queries
	objectTypes := []string{}
	for _, group := range vcenter.MetricGroups {
		objectTypes = append(objectTypes, group.ObjectType)
	}

	// Loop trought datacenters and create the intersting object reference list
	mors := []types.ManagedObjectReference{}
	for _, datacenter := range datacenters {
		// Create the CreateContentView request
		req := types.CreateContainerView{This: viewManager.Reference(), Container: datacenter, Type: objectTypes, Recursive: true}
		res, err := methods.CreateContainerView(ctx, client.RoundTripper, &req)
		if err != nil {
			errlog.Println("Could not create container view from vcenter: " + vcenter.Hostname)
			errlog.Println("Error: ", err)
			continue
		}
		// Retrieve the created ContentView
		var containerView mo.ContainerView
		err = client.RetrieveOne(ctx, res.Returnval, nil, &containerView)
		if err != nil {
			errlog.Println("Could not get container view from vcenter: " + vcenter.Hostname)
			errlog.Println("Error: ", err)
			continue
		}
		// Add found object to object list
		mors = append(mors, containerView.View...)
	}

	// get object names
	objects := []mo.ManagedEntity{}

	//object for propery collection
	propSpec := &types.PropertySpec{Type: "ManagedEntity", PathSet: []string{"name"}}
	var objectSet []types.ObjectSpec
	for _, mor := range mors {
		objectSet = append(objectSet, types.ObjectSpec{Obj: mor, Skip: types.NewBool(false)})
	}

	//retrieve name property
	propreq := types.RetrieveProperties{SpecSet: []types.PropertyFilterSpec{{ObjectSet: objectSet, PropSet: []types.PropertySpec{*propSpec}}}}
	propres, err := client.PropertyCollector().RetrieveProperties(ctx, propreq)
	if err != nil {
		errlog.Println("Could not retrieve object names from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	//load retrieved properties
	err = mo.LoadRetrievePropertiesResponse(propres, &objects)
	if err != nil {
		errlog.Println("Could not retrieve object names from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	//create a map to resolve object names
	morToName := make(map[types.ManagedObjectReference]string)
	for _, object := range objects {
		morToName[object.Self] = object.Name
	}

	//create a map to resolve metric names
	metricToName := make(map[int32]string)
	for _, metricgroup := range vcenter.MetricGroups {
		for _, metricdef := range metricgroup.Metrics {
			metricToName[metricdef.Key] = metricdef.Metric
		}
	}

	// Create Queries from interesting objects and requested metrics

	queries := []types.PerfQuerySpec{}

	// Common parameters
	intervalId := int32(20)
	endTime := time.Now().Add(time.Duration(-1) * time.Second)
	startTime := endTime.Add(time.Duration(-interval) * time.Second)

	// Parse objects
	for _, mor := range mors {
		metricIds := []types.PerfMetricId{}
		for _, metricgroup := range vcenter.MetricGroups {
			if metricgroup.ObjectType == mor.Type {
				for _, metricdef := range metricgroup.Metrics {
					metricIds = append(metricIds, types.PerfMetricId{CounterId: metricdef.Key, Instance: metricdef.Instances})
				}
			}
		}
		queries = append(queries, types.PerfQuerySpec{Entity: mor, StartTime: &startTime, EndTime: &endTime, MetricId: metricIds, IntervalId: intervalId})
	}

	// Query the performances
	perfreq := types.QueryPerf{This: *client.ServiceContent.PerfManager, QuerySpec: queries}
	perfres, err := methods.QueryPerf(ctx, client.RoundTripper, &perfreq)
	if err != nil {
		errlog.Println("Could not request perfs from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	// Get the result
	values := []graphite.Metric{}
	vcName := strings.Replace(vcenter.Hostname, domain, "", -1)
	for _, base := range perfres.Returnval {
		pem := base.(*types.PerfEntityMetric)
		entityName := strings.ToLower(pem.Entity.Type)
		name := strings.ToLower(strings.Replace(morToName[pem.Entity], domain, "", -1))
		for _, baseserie := range pem.Value {
			serie := baseserie.(*types.PerfMetricIntSeries)
			metricName := strings.ToLower(metricToName[serie.Id.CounterId])
			instanceName := serie.Id.Instance
			key := "vsphere." + vcName + "." + entityName + "." + name + "." + metricName
			if len(instanceName) > 0 {
				key += "." + strings.ToLower(strings.Replace(instanceName, ".", "_", -1))
			}
			var value int64 = -1
			if strings.HasSuffix(metricName, ".average") {
				value = utils.Average(serie.Value...)
			} else if strings.HasSuffix(metricName, ".maximum") {
				value = utils.Max(serie.Value...)
			} else if strings.HasSuffix(metricName, ".minimum") {
				value = utils.Min(serie.Value...)
			} else if strings.HasSuffix(metricName, ".latest") {
				value = serie.Value[len(serie.Value)-1]
			} else if strings.HasSuffix(metricName, ".summation") {
				value = utils.Sum(serie.Value...)
			}
			values = append(values, graphite.NewMetric(key, strconv.FormatInt(value, 10), endTime.Unix()))
			//stdlog.Printf( key + "\t" + strconv.FormatInt(value,10))
		}
	}
	*channel <- values
}
