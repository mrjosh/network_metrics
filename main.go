package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	ping "github.com/digineo/go-ping"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	err              error
	logger           *zap.Logger
	debug            *bool
	networkIPAddress = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_address_entries",
			Help: "Current network ip address entries",
		},
		[]string{"interface", "ip_address"},
	)
	pingGuage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_ping",
			Help: "Current network ip address ping",
		},
		[]string{"interface", "ip_address"},
	)
)

func configureNetworkIPAddress(ifaceName, ifaceIP string) (string, error) {

	// Select network adapter that using 192.168.0.50
	// and it will be connect to port 0 (dynamically generate an unused port number)
	//addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", ifaceIP))
	//if err != nil {
	//return err
	//}

	dialer := &net.Dialer{}

	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dialer.Dial(network, addr)
		return conn, err
	}

	transport := &http.Transport{DialContext: dialContext}
	client := &http.Client{
		Transport: transport,
	}

	// http request
	response, err := client.Get("https://icanhazip.com") // get my IP address
	if err != nil {
		return "", err
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	dataStr := strings.TrimRight(string(data), "\r\n")

	networkIPAddress.WithLabelValues(ifaceName, dataStr)
	return dataStr, nil
}

func main() {

	ifaceName := flag.String("ifname", "", "Network interface name")
	ifaceIP := flag.String("ifip", "", "Network interface ip")
	debug = flag.Bool("debug", false, "Debug logger")
	flag.Parse()

	logger, err = zap.NewProduction()
	if err != nil {
		log.Println(err)
		return
	}
	defer logger.Sync()

	logWithZap("prometheus_network_exporter starting...")

	r := prometheus.NewRegistry()
	r.MustRegister(networkIPAddress)
	r.MustRegister(pingGuage)

	currentNetworkIP, err := configureNetworkIPAddress(*ifaceName, *ifaceIP)
	if err != nil {
		logWithZap(err.Error())
	}

	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for {
			<-ticker.C
			if currentNetworkIP, err = configureNetworkIPAddress(*ifaceName, *ifaceIP); err != nil {
				logWithZap(err.Error())
			}
			logWithZap("Configured Network IPAddress")
		}
	}()

	go func(currentNetworkIP string) {

		for {
			<-ticker.C

			logWithZap(fmt.Sprintf("Pinging: %s", currentNetworkIP))

			pingIPAddr, err := net.ResolveIPAddr("ip4", currentNetworkIP)
			if err != nil {
				logWithZap(err.Error())
			}

			pinger, err := ping.New("0.0.0.0", "")
			if err != nil {
				logWithZap(err.Error())
				return
			}
			defer pinger.Close()

			rtt, err := pinger.PingAttempts(pingIPAddr, time.Second*5, 5)
			if err != nil {
				logWithZap(err.Error())
				os.Exit(1)
			}

			pingGuage.WithLabelValues(*ifaceName, currentNetworkIP).Set(float64(rtt))
			logWithZap("Configured Network IPAddress Ping")

		}

	}(currentNetworkIP)

	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})

	http.Handle("/metrics", handler)
	http.HandleFunc("/reboot-wireguard", func(w http.ResponseWriter, r *http.Request) {

	})

	logWithZap(fmt.Sprintf(
		"prometheus_network_exporter server listening on [%s]",
		"0.0.0.0:9091",
	))
	log.Fatal(http.ListenAndServe(":9091", nil))
}

func logWithZap(msg string, fields ...zap.Field) {
	if *debug {
		logger.Info(msg, fields...)
	}
}
