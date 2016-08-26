package jenkins

import (
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/yieldbot/golang-jenkins"
	"fmt"
)

type Jenkins struct {
	Host     string
	URL      string
	Username string
	Password string
	Insecure bool
}

var sampleConfig = `
	## specify host for use as an additional tag in influx
	host = jenkins1
	## specify url via a url matching:
	##  [protocol://]address[:port]
	##  e.g.
	##    http://jenkins.service.consul:8080/
	##    http://jenkins.foo.com/
	url = http://jenkins.service.consul:8080
	## specify username and password for logging in to jenkins
	## password may optionally be a jenkins generated API token
	username = admin
	password = password
	## Specify insecure to ignore SSL errors
	insecure = true
`

func (j *Jenkins) SampleConfig() string {
	return sampleConfig
}

func (j *Jenkins) Description() string {
	return "Reads metrics from a Jenkins server"
}

func (j *Jenkins) Gather(acc telegraf.Accumulator) error {
	auth := &gojenkins.Auth{
		Username: j.Username,
		ApiToken: j.Password,
	}

	client := gojenkins.NewJenkins(auth, j.URL)

	if j.Insecure {
		c := newInsecureHTTP()
		client.OverrideHTTPClient(c)
	}

	err := j.gatherQueue(acc, client)
	if err != nil {
		return err
	}

	err = j.gatherSlaves(acc, client)
	if err != nil {
		return err
	}

	err = j.gatherSlaveLabels(acc, client)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	inputs.Add("jenkins", func() telegraf.Input {
		return &Jenkins{}
	})
}

func (j *Jenkins) gatherQueue(acc telegraf.Accumulator, client *gojenkins.Jenkins) error {

	fields := make(map[string]interface{})
	tags := make(map[string]string)

	qSize := 0
	qMap := make(map[string]int)

	queue, err := client.GetQueue()
	if err != nil {
		return err
	}

	for _, item := range queue.Items {
		if item.Buildable {
			qSize++
			j, err := client.GetJobProperties(item.Task.Name)
			if err != nil {
				return err
			}
			if val, ok := qMap[j.AssignedNode]; ok {
				qMap[j.AssignedNode] = val + 1
			} else {
				qMap[j.AssignedNode] = 1
			}
		}
	}

	fmt.Println(qMap)
	fields["queue_size"] = qSize
	if j.Host != "" {
		tags["host"] = j.Host
	}
	if len(qMap) > 0 {
		for key := range qMap {
			fields[fmt.Sprintf("label_%s", key)] = qMap[key]
		}
	}
	tags["url"] = j.URL

	acc.AddFields("jenkins_queue", fields, tags)

	return nil
}

func (j *Jenkins) gatherSlaves(acc telegraf.Accumulator, client *gojenkins.Jenkins) error {
	fields := make(map[string]interface{})
	tags := make(map[string]string)

	var slaveCount = 0
	var busyCount = 0

	slaves, err := client.GetComputers()
	if err != nil {
		return err
	}

	for _, slave := range slaves {
		if slave.JnlpAgent {
			slaveCount++

			if !slave.Idle {
				busyCount++
			}
		}
	}

	fields["slave_count"] = slaveCount
	fields["slaves_busy"] = busyCount
	if j.Host != "" {
		tags["host"] = j.Host
	}
	tags["url"] = j.URL

	acc.AddFields("jenkins_slaves", fields, tags)

	return nil
}

func (j *Jenkins) gatherSlaveLabels(acc telegraf.Accumulator, client *gojenkins.Jenkins) error {
	fields := make(map[string]interface{})
	tags := make(map[string]string)

	slaves, err := client.GetComputers()
	if err != nil {
		return err
	}

	for _, slave := range slaves {
		if slave.JnlpAgent {
			conf, err := client.GetComputerConfig(slave.DisplayName)
			if err != nil {
				return err
			}
			labels := strings.Split(conf.Label, " ")
			for _, label := range labels {
				if val, ok := fields[label]; ok {
					fields[label] = val.(int) + 1
				} else {
					fields[label] = 1
				}
			}
		}
	}

	if j.Host != "" {
		tags["host"] = j.Host
	}
	tags["url"] = j.URL

	acc.AddFields("jenkins_labels", fields, tags)

	return nil
}

func newInsecureHTTP() *http.Client {
	ntls := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: ntls}
}
