package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "syslog_ng" // For Prometheus metrics.
)

var (
	app              = kingpin.New("syslog_ng_exporter", "A Syslog-NG exporter for Prometheus")
	showVersion      = app.Flag("version", "Print version information.").Bool()
	listeningAddress = app.Flag("telemetry.address", "Address on which to expose metrics.").Default(":9577").String()
	metricsEndpoint  = app.Flag("telemetry.endpoint", "Path under which to expose metrics.").Default("/metrics").String()
	socketPath       = app.Flag("socket.path", "Path to syslog-ng control socket.").Default("/var/lib/syslog-ng/syslog-ng.ctl").String()
	insecure         = kingpin.Flag("insecure", "Ignore server certificate if using https.").Bool() // add insecure for non https
	gracefulStop     = make(chan os.Signal)                                                         // gracefully stop the system
)

type Exporter struct {
	sockPath string
	mutex    sync.Mutex
	client   *http.Client

	srcProcessed   *prometheus.Desc
	dstProcessed   *prometheus.Desc
	dstDropped     *prometheus.Desc
	dstStored      *prometheus.Desc
	dstWritten     *prometheus.Desc
	dstMemory      *prometheus.Desc
	dstCPU         *prometheus.Desc
	up             *prometheus.Desc
	scrapeSuccess  prometheus.Counter
	scrapeFailures prometheus.Counter
}

type Stat struct {
	objectType string
	id         string
	instance   string
	state      string
	metric     string
	value      float64
}

func NewExporter(path string) *Exporter {
	return &Exporter{
		sockPath: path,
		srcProcessed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "source_messages_processed", "total"),
			"Number of messages processed by this source.",
			[]string{"type", "id", "source"},
			nil),
		dstProcessed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "destination_messages_processed", "total"),
			"Number of messages processed by this destination.",
			[]string{"type", "id", "destination"},
			nil),
		dstDropped: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "destination_messages_dropped", "total"),
			"Number of messages dropped by this destination due to store overflow.",
			[]string{"type", "id", "destination"},
			nil),
		dstStored: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "destination_messages_stored", "total"),
			"Number of messages currently stored for this destination.",
			[]string{"type", "id", "destination"},
			nil),
		dstWritten: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "destination_messages_written", "total"),
			"Number of messages successfully written by this destination.",
			[]string{"type", "id", "destination"},
			nil),
		dstMemory: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "destination_bytes_stored", "total"),
			"Bytes of memory currently used to store messages for this destination.",
			[]string{"type", "id", "destination"},
			nil),
		dstCPU: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "destination_bytes_processed", "total"),
			"Bytes of cpu currently used to process messages for this destination.",
			[]string{"type", "id", "destination"},
			nil),
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Reads 1 if the syslog-ng server could be reached, else 0.",
			nil,
			nil),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while scraping syslog-ng.",
		}),
		scrapeSuccess: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_success_total",
			Help:      "Number of successfully scrape metrics for syslog-ng.",
		}),
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: *insecure},
			},
		},
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.srcProcessed
	ch <- e.dstProcessed
	ch <- e.dstDropped
	ch <- e.dstStored
	ch <- e.dstWritten
	ch <- e.dstMemory
	ch <- e.dstCPU
	ch <- e.up
	e.scrapeFailures.Describe(ch)
	e.scrapeSuccess.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if err := e.collect(ch); err != nil {
		log.Errorf("Error scraping syslog-ng: %s", err)
		e.scrapeFailures.Inc()
		e.scrapeFailures.Collect(ch)
	}

	// collect successful metrics
	e.scrapeSuccess.Inc()
	e.scrapeSuccess.Collect(ch)
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	conn, err := net.Dial("unix", e.sockPath)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		return fmt.Errorf("Error connecting to syslog-ng: %v", err)
	}

	defer conn.Close()

	// set 30s connection deadline
	err = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		return fmt.Errorf("Failed to set conn deadline: %v", err)
	}

	_, err = conn.Write([]byte("STATS\n"))
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		return fmt.Errorf("Error writing to control socket: %v", err)
	}

	buff := bufio.NewReader(conn)

	_, err = buff.ReadString('\n')
	if err != nil {
		return fmt.Errorf("Error reading header from control socket: %v", err)
	}

	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1)

	for {
		line, err := buff.ReadString('\n')

		if err != nil || line[0] == '.' {
			log.Debug("Reached end of STATS output")
			break
		}

		stat, err := parseStatLine(line)
		if err != nil {
			log.Debugf("Skipping STATS line: %v", err)
			continue
		}

		log.Debugf("Successfully parsed STATS line: %v", stat)

		switch stat.objectType[0:4] {
		case "src.", "sour":
			switch stat.metric {
			case "processed":
				ch <- prometheus.MustNewConstMetric(e.srcProcessed, prometheus.CounterValue,
					stat.value, stat.objectType, stat.id, stat.instance)
			}

		case "dst.", "dest":
			switch stat.metric {
			case "dropped":
				ch <- prometheus.MustNewConstMetric(e.dstDropped, prometheus.CounterValue,
					stat.value, stat.objectType, stat.id, stat.instance)
			case "processed":
				ch <- prometheus.MustNewConstMetric(e.dstProcessed, prometheus.CounterValue,
					stat.value, stat.objectType, stat.id, stat.instance)
			case "written":
				ch <- prometheus.MustNewConstMetric(e.dstWritten, prometheus.CounterValue,
					stat.value, stat.objectType, stat.id, stat.instance)
			case "stored", "queued":
				ch <- prometheus.MustNewConstMetric(e.dstStored, prometheus.GaugeValue,
					stat.value, stat.objectType, stat.id, stat.instance)
			case "memory_usage":
				ch <- prometheus.MustNewConstMetric(e.dstMemory, prometheus.GaugeValue,
					stat.value, stat.objectType, stat.id, stat.instance)
			case "cpu_usage":
				ch <- prometheus.MustNewConstMetric(e.dstMemory, prometheus.GaugeValue,
					stat.value, stat.objectType, stat.id, stat.instance)
			}
		}
	}

	return nil
}

func parseStatLine(line string) (Stat, error) {
	statlen := 7
	part := strings.SplitN(strings.TrimSpace(line), ";", statlen)
	if len(part) < statlen {
		return Stat{}, fmt.Errorf("insufficient parts: %d < 6", len(part))
	}

	if len(part[0]) < 4 {
		return Stat{}, fmt.Errorf("invalid name: %s", part[0])
	}

	val, err := strconv.ParseFloat(part[5], 64)
	if err != nil {
		return Stat{}, err
	}

	stat := Stat{part[0], part[1], part[2], part[3], part[4], val}

	return stat, nil
}

func main() {
	log.AddFlags(app)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	log.Infoln("Starting syslog_ng_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Starting server: %s", *listeningAddress)

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("syslog_ng_exporter"))
		os.Exit(0)
	}

	exporter := NewExporter(*socketPath)
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("syslog_ng_exporter"))

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
