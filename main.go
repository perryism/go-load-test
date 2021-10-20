package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/senseyeio/roger"
	"gopkg.in/yaml.v3"
)

type Listener interface {
	Observe(Sampler)
	Exit()
}

type Sampler struct {
	name    string
	perform Perform
}

type Perform func() error

type ThreadGroup struct {
	sampler Sampler
	freq    int
}

type Timer struct {
	start    time.Time
	duration time.Duration
}

func (t *Timer) Observe(sampler Sampler) error {
	t.start = time.Now()
	err := sampler.perform()
	t.duration = time.Since(t.start)
	return err
}

type ReportEntry struct {
	Timer Timer
	err   error
}

type Report struct {
	entries []ReportEntry
}

func (r *Report) AddEntry(t Timer, err error) {
	r.entries = append(r.entries, ReportEntry{Timer: t, err: err})
}

type StandoutListener struct {
	Report
}

func (sl *StandoutListener) Observe(sampler Sampler) {
	timer := Timer{}
	err := timer.Observe(sampler)
	sl.Report.AddEntry(timer, err)
}

func (sl *StandoutListener) Exit() {
	min := time.Duration(math.MaxInt64)
	max := time.Duration(0)
	total := time.Duration(0)
	n := int64(0)
	error_count := 0
	for _, e := range sl.Report.entries {
		t := e.Timer
		if e.err != nil {
			error_count += 1
		}
		n += 1
		fmt.Println(t.duration)
		if t.duration > max {
			max = t.duration
		} else if t.duration < min {
			min = t.duration
		}
		total += t.duration
	}

	fmt.Printf("error_count: %d\n", error_count)
	fmt.Printf("min: %s\n", min)
	fmt.Printf("max: %s\n", max)
	fmt.Printf("avg: %d\n", total.Milliseconds()/n)
}

func (tg *ThreadGroup) Start(listener Listener) {
	defer listener.Exit()
	for i := 0; i < tg.freq; i++ {
		listener.Observe(tg.sampler)
	}
}

type PyServe struct {
	host string
}

func (p *PyServe) post(subpath string, data string) error {
	u, err := url.Parse(p.host)
	u.Path = path.Join(u.Path, subpath)
	r := strings.NewReader(data)
	resp, err := http.Post(u.String(), "application/x-www-form-urlencoded", r)

	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}
	//fmt.Println(rsep.StatusCode)
	defer resp.Body.Close()

	// b, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	// fmt.Println(string(b))

	return err
}

type RServe struct {
	client roger.RClient
}

func NewRServe(host string, port int64) RServe {
	rClient, err := roger.NewRClient(host, port)
	if err != nil {
		panic("Failed to connect")
	}
	return RServe{client: rClient}
}

func (r *RServe) perform(command string) error {
	_, err := r.client.Eval(command)
	if err != nil {
		fmt.Println("Command failed: " + err.Error())
	} else {
		//fmt.Println(value)
	}
	return err
}

func loadTest(sampler Sampler, freq int) {
	tg := ThreadGroup{sampler: sampler, freq: freq}
	sl := &StandoutListener{}
	tg.Start(sl)
}

type config struct {
	samplers []Sampler
}

func NewConfig(filepath string) config {
	yfile, err := ioutil.ReadFile(filepath)
	if err != nil {
		panic(err)
	}

	data := make(map[interface{}]interface{})
	err = yaml.Unmarshal(yfile, &data)

	if err != nil {
		panic(err)
	}

	conf := config{}

	for k, v := range data {
		value := v.(map[string]interface{})
		for kk, vv := range value {
			c := vv.(map[string]interface{})
			if kk == "http_post" {
				pyserve := PyServe{host: c["host"].(string)}
				conf.samplers = append(conf.samplers, Sampler{name: k.(string), perform: func() error {
					path := c["path"].(string)
					return pyserve.post(path, c["data"].(string))
				}})
			} else if kk == "rserve" {
				rserve := NewRServe(c["host"].(string), int64(c["port"].(int)))
				conf.samplers = append(conf.samplers, Sampler{name: k.(string), perform: func() error {
					return rserve.perform(c["data"].(string))
				}})
			}
		}
	}
	return conf
}

func main() {
	freq := flag.Int("freq", 10, "frequency")
	configpath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()
	conf := NewConfig(*configpath)

	for _, sampler := range conf.samplers {
		fmt.Println(sampler.name)
		loadTest(sampler, *freq)
	}
}
