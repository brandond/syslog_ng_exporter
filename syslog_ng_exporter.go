package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

const (
	namespace = "syslog-ng" // For Prometheus metrics.
)

var (
	listeningAddress = flag.String("telemetry.address", ":9577", "Address on which to expose metrics.")
	metricsEndpoint  = flag.String("telemetry.endpoint", "/metrics", "Path under which to expose metrics.")
	socketPath       = flag.String("socket_path", "/var/lib/syslog-ng/syslog-ng.ctl", "Path to syslog-ng control socket.")
	showVersion      = flag.Bool("version", false, "Print version information.")
)

type Exporter struct {
	sockPath string
	mutex    sync.Mutex

	up             *prometheus.Desc
	scrapeFailures prometheus.Counter
	srcProcessed   *prometheus.GaugeVec
	dstProcessed   *prometheus.GaugeVec
	dstDropped     *prometheus.GaugeVec
	dstStored      *prometheus.GaugeVec
}

type Stat struct {
	name     string
	id       string
	instance string
	state    string
	metric   string
	value    float64
}

func NewExporter(path string) *Exporter {
	return &Exporter{
		sockPath: path,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could the syslog-ng server be reached.",
			nil,
			nil),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while scraping syslog-ng.",
		}),
		srcProcessed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "source_messages_processed_total",
			Help:      "Number of messages processed by this source.",
		},
			[]string{"name", "id", "instance"},
		),
		dstProcessed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "destination_messages_processed_total",
			Help:      "Number of messages processed by this destination.",
		},
			[]string{"name", "id", "instance"},
		),
		dstDropped: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "destination_messages_dropped_total",
			Help:      "Number of messages dropped by this destination.",
		},
			[]string{"name", "id", "instance"},
		),
		dstStored: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "destination_messages_stored_total",
			Help:      "Number of messages stored by this destination.",
		},
			[]string{"name", "id", "instance"},
		),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	e.scrapeFailures.Describe(ch)
	e.srcProcessed.Describe(ch)
	e.dstProcessed.Describe(ch)
	e.dstDropped.Describe(ch)
	e.dstStored.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if err := e.collect(ch); err != nil {
		log.Errorf("Error scraping syslog-ng: %s", err)
		e.scrapeFailures.Inc()
		e.scrapeFailures.Collect(ch)
	}
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	conn, err := net.Dial("unix", e.sockPath)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		return fmt.Errorf("Error connecting to syslog-ng: %v", err)
	}

	defer conn.Close()

	_, err = conn.Write([]byte("STATS\n\n\n"))
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		return fmt.Errorf("Error writing to syslog-ng: %v", err)
	}

	var buff bytes.Buffer
	_, err = io.Copy(&buff, conn)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		return fmt.Errorf("Error reading from syslog-ng: %v", err)
	}

	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1)

	for {
		line, err := buff.ReadString('\n')
		if err != nil {
			break
		}

		part := strings.SplitN(line, ";", 6)
		val, err := strconv.ParseFloat(part[5], 64)

		if err != nil || len(part[0]) < 4 {
			continue
		}

		stat := Stat{part[0], part[1], part[2], part[3], part[4], val}

		switch stat.name[0:4] {
		case "src.":
			switch stat.metric {
			case "processed":
				e.srcProcessed.WithLabelValues(stat.name, stat.id, stat.instance).Set(stat.value)
			}

		case "dst.":
			switch stat.metric {
			case "dropped":
				e.dstDropped.WithLabelValues(stat.name, stat.id, stat.instance).Set(stat.value)
			case "processed":
				e.dstProcessed.WithLabelValues(stat.name, stat.id, stat.instance).Set(stat.value)
			case "stored":
				e.dstStored.WithLabelValues(stat.name, stat.id, stat.instance).Set(stat.value)
			}
		}
	}

	return nil
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("syslog_ng_exporter"))
		os.Exit(0)
	}

	exporter := NewExporter(*socketPath)
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("syslog_ng_exporter"))

	log.Infoln("Starting syslog_ng_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Starting server: %s", *listeningAddress)

	http.Handle(*metricsEndpoint, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
			<head><title>Syslog-NG Exporter</title></head>
			<body>
			<h1>Syslog-NG Exporter</h1>
			<p><a href='` + *metricsEndpoint + `'>Metrics</a></p>
			</body>
			</html>`))
		if err != nil {
			log.Warnf("Failed sending response: %v", err)
		}
	})
	log.Fatal(http.ListenAndServe(*listeningAddress, nil))
}
