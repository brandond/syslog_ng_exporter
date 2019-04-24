package main

import (
	"fmt"
	"net"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	statsLevel1 = `SourceName;SourceId;SourceInstance;State;Type;Number
destination;d_spol;;a;processed;0
src.internal;s_sys#2;;a;processed;4
src.internal;s_sys#2;;a;stamp;1556092662
center;;received;a;processed;4
src.unix-dgram;s_sys#0;/run/systemd/journal/syslog;a;processed;16
src.unix-dgram;s_sys#0;/run/systemd/journal/syslog;a;stamp;1556092680
destination;d_mesg;;a;processed;7
destination;d_mail;;a;processed;0
destination;d_auth;;a;processed;13
destination;d_mlal;;a;processed;0
center;;queued;a;processed;20
src.none;;;a;processed;0
src.none;;;a;stamp;0
destination;d_cron;;a;processed;0
global;payload_reallocs;;a;processed;0
global;sdata_updates;;a;processed;0
dst.file;d_mesg#0;/var/log/messages;a;dropped;0
dst.file;d_mesg#0;/var/log/messages;a;processed;7
dst.file;d_mesg#0;/var/log/messages;a;stored;0
src.file;s_sys#1;/dev/kmsg;a;processed;0
src.file;s_sys#1;/dev/kmsg;a;stamp;0
destination;d_boot;;a;processed;0
destination;d_kern;;a;processed;0
global;msg_clones;;a;processed;0
source;s_sys;;a;processed;4
dst.file;d_auth#0;/var/log/secure;a;dropped;0
dst.file;d_auth#0;/var/log/secure;a;processed;13
dst.file;d_auth#0;/var/log/secure;a;stored;0
.`
	statsLevel2 = `SourceName;SourceId;SourceInstance;State;Type;Number
dst.file;d_mesg#0;/var/log/messages;a;dropped;0
dst.file;d_mesg#0;/var/log/messages;a;processed;610
dst.file;d_mesg#0;/var/log/messages;a;stored;0
destination;d_spol;;a;processed;0
src.internal;s_sys#2;;a;processed;72
src.internal;s_sys#2;;a;stamp;1556092051
center;;received;a;processed;72
src.unix-dgram;s_sys#0;/run/systemd/journal/syslog;a;processed;675
src.unix-dgram;s_sys#0;/run/systemd/journal/syslog;a;stamp;1556092606
destination;d_mesg;;a;processed;610
destination;d_mail;;a;processed;0
destination;d_auth;;a;processed;51
destination;d_mlal;;a;processed;0
center;;queued;a;processed;797
src.none;;;a;processed;0
src.none;;;a;stamp;0
destination;d_cron;;a;processed;111
global;payload_reallocs;;a;processed;88
global;sdata_updates;;a;processed;0
dst.file;d_kern#0;/var/log/kern;o;dropped;0
dst.file;d_kern#0;/var/log/kern;o;processed;25
dst.file;d_kern#0;/var/log/kern;o;stored;0
src.host;;l261767-vm;d;processed;772
src.host;;l261767-vm;d;stamp;1556092606
dst.file;d_cron#0;/var/log/cron;o;dropped;0
dst.file;d_cron#0;/var/log/cron;o;processed;111
dst.file;d_cron#0;/var/log/cron;o;stored;0
src.file;s_sys#1;/dev/kmsg;a;processed;25
src.file;s_sys#1;/dev/kmsg;a;stamp;1556091325
destination;d_boot;;a;processed;0
destination;d_kern;;a;processed;25
global;msg_clones;;a;processed;0
source;s_sys;;a;processed;72
dst.file;d_auth#0;/var/log/secure;a;dropped;0
dst.file;d_auth#0;/var/log/secure;a;processed;51
dst.file;d_auth#0;/var/log/secure;a;stored;0
.`
	metricCountLevel1 = 11
	metricCountLevel2 = 18
)

func acceptAndSend(sock net.Listener, response string) error {
	conn, err := sock.Accept()
	if err != nil {
		return fmt.Errorf("accept failed: %v", err)
	}

	defer conn.Close()

	_, err = conn.Write([]byte(response))
	if err != nil {
		return fmt.Errorf("write failed: %v", err)
	}

	return nil
}

func checkSyslogNGStats(t *testing.T, stats string, metricCount int) {
	sock, err := net.Listen("unix", "@test")
	if err != nil {
		t.Errorf("Failed to listen on test socket: %v", err)
	}

	defer sock.Close()
	go acceptAndSend(sock, stats)

	e := NewExporter("@test")
	ch := make(chan prometheus.Metric)

	go func() {
		defer close(ch)
		e.collect(ch)
	}()

	for i := 1; i <= metricCount; i++ {
		m := <-ch
		if m == nil {
			t.Error("Expected metric but got nil")
		} else {
			t.Logf("Got metric: %v", m)
		}
	}

	extraMetrics := 0

	for <-ch != nil {
		extraMetrics++
	}

	if extraMetrics > 0 {
		t.Errorf("expected end of stats, got %d extra metrics", extraMetrics)
	}

}

func TestStatsLevel1(t *testing.T) {
	checkSyslogNGStats(t, statsLevel1, metricCountLevel1)
}

func TestStatsLevel2(t *testing.T) {
	checkSyslogNGStats(t, statsLevel2, metricCountLevel2)
}
