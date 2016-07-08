package jenkins

import (
	"crypto/tls"
	"net/http"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/yieldbot/golang-jenkins"
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

	queue, err := client.GetQueue()
	if err != nil {
		return err
	}

	for _, item := range queue.Items {
		if item.Buildable {
			qSize++
		}
	}

	fields["queue_size"] = qSize
	if j.Host != "" {
		tags["host"] = j.Host
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

func newInsecureHTTP() *http.Client {
	ntls := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: ntls}
}
