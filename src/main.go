package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gen2brain/beeep"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/showwin/speedtest-go/speedtest"
)

var logger = slog.Default()

const CSVHEADER string = "timestamp,interface,speed_down (mbps),speed_up (mbps),speed_jitter (ms),speedtest_id,ping_tx,ping_rx,ping_min (ms),ping_max (ms),ping_avg (ms),duration (s),error\n"

var OUTDIR string
var OUTFILE string
var OUTPATH string
var SAVEFILES bool
var TIMEOUT time.Duration
var NOTIFY bool
var ACTIVEINTERFACES []string

type SpeedResult struct {
	TestID    string
	TestHost  string
	SpeedDown float32
	SpeedUp   float32
	Jitter    time.Duration
	Duration  time.Duration
	Error     string
}

type PingResult struct {
	TestHost  string
	Sent      int
	Recv      int
	Dupe      int
	Loss      float64
	RttMin    time.Duration
	RttMax    time.Duration
	RttAvg    time.Duration
	RttStdDev time.Duration
	Duration  time.Duration
	Error     string
}

type TestResult struct {
	Timestamp time.Time
	Interface string

	SpeedStats *SpeedResult
	// SpeedTestID   string
	// SpeedTestHost string
	// SpeedDown     float32
	// SpeedUp       float32
	// Jitter        time.Duration

	PingStats *PingResult
	// PingHost  string
	// PingSent  int
	// PingRecv  int
	// RttMin    time.Duration
	// RttMax    time.Duration
	// RttAvg    time.Duration
	// RttStdDev time.Duration

	Duration time.Duration
	Error    *string
}

func (t TestResult) String() string {
	err := ""
	speed := ""
	ping := ""
	if t.Error != nil {
		err = fmt.Sprintf(
			" Error:"+
				"   %s\n",
			*t.Error,
		)
	}

	if t.SpeedStats != nil {
		speed = fmt.Sprintf(
			" SpeedTest:\n"+
				"  Duration:\t%.1f\n"+
				"  Download:\t%.2f mbps\n"+
				"  Upload:\t%.2f mbps\n"+
				"  Jitter:\t%.2f ms\n"+
				"  Test ID:\thttps://www.speedtest.net/result/%s\n",
			float64((*t.SpeedStats).Duration.Milliseconds())/1000,
			(*t.SpeedStats).SpeedDown,
			(*t.SpeedStats).SpeedUp,
			float64((*t.SpeedStats).Jitter.Microseconds())/1000,
			(*t.SpeedStats).TestID,
		)
	}

	if t.PingStats != nil {
		ping = fmt.Sprintf(
			" PingTest:\n"+
				"  Duration:\t%.1f\n"+
				"  Sent:\t%d\n"+
				"  Recv:\t%d\n"+
				"  Dupe:\t%d\n"+
				"  Lost:\t%.2f %%\n"+
				"  RttMin:\t%.2f ms\n"+
				"  RttAvg:\t%.2f ms\n"+
				"  RttMax:\t%.2f ms\n",
			float64((*t.PingStats).Duration.Milliseconds())/1000,
			(*t.PingStats).Sent,
			(*t.PingStats).Recv,
			(*t.PingStats).Dupe,
			(*t.PingStats).Loss*100,
			float64((*t.PingStats).RttMin.Microseconds())/1000,
			float64((*t.PingStats).RttMax.Microseconds())/1000,
			float64((*t.PingStats).RttAvg.Microseconds())/1000,
		)
	}

	return fmt.Sprintf(
		"========================\n"+
			"Test %s @ %s\n"+
			" Duration:\t%.1f s\n"+
			"%s"+
			"%s"+
			"%s",
		t.Interface,
		t.Timestamp.Format(time.RFC3339),
		float64(t.Duration.Milliseconds())/1000,
		speed,
		ping,
		err,
	)
}

func (t TestResult) CSV() string {
	err := ""
	speed := ",-,-,-"
	ping := ",-,-,-,-"
	if t.Error != nil {
		err = strings.ReplaceAll(*t.Error, "\n", "")
	}

	if t.SpeedStats != nil {
		speed = fmt.Sprintf(
			"%.2f,%.2f,%.2f,%s",
			(*t.SpeedStats).SpeedDown,
			(*t.SpeedStats).SpeedUp,
			float64((*t.SpeedStats).Jitter.Microseconds())/1000,
			(*t.SpeedStats).TestID,
		)
	}

	if t.PingStats != nil {
		ping = fmt.Sprintf(
			"%d,%d,%.2f,%.2f,%.2f",
			(*t.PingStats).Sent,
			(*t.PingStats).Recv,
			float64((*t.PingStats).RttMin.Microseconds())/1000,
			float64((*t.PingStats).RttMax.Microseconds())/1000,
			float64((*t.PingStats).RttAvg.Microseconds())/1000,
		)
	}

	return fmt.Sprintf(
		"%s,%s,%s,%s,%.1f,%s\n",
		t.Timestamp.Format(time.RFC3339),
		t.Interface,
		speed,
		ping,
		float64(t.Duration.Milliseconds())/1000,
		err,
	)
}

func doSpeedtest() (*SpeedResult, error) {
	start := time.Now()
	var speedtestClient = speedtest.New()
	serverList, _ := speedtestClient.FetchServers()
	targets, _ := serverList.FindServer([]int{})
	var result SpeedResult

	for _, s := range targets {
		// s.DownloadTest()
		// s.UploadTest()
		err := s.TestAll()
		if err != nil {
			return nil, err
		}
		result = SpeedResult{
			TestID:    s.ID,
			TestHost:  s.Host,
			SpeedDown: float32(s.DLSpeed.Mbps()),
			SpeedUp:   float32(s.ULSpeed.Mbps()),
			Jitter:    s.Jitter,
			Duration:  time.Since(start),
		}
		s.Context.Reset()
	}
	return &result, nil
}

func doPingtest(host string, count int) (*PingResult, error) {
	start := time.Now()
	pinger, err := probing.NewPinger(host)
	if err != nil {
		return nil, err
	}
	pinger.SetPrivileged(true)
	pinger.Count = count
	err = pinger.Run()
	if err != nil {
		return nil, err
	}
	stats := pinger.Statistics()

	return &PingResult{
		Sent:      stats.PacketsSent,
		Recv:      stats.PacketsRecv,
		Dupe:      stats.PacketsRecvDuplicates,
		Loss:      stats.PacketLoss,
		RttMin:    stats.MinRtt,
		RttMax:    stats.MaxRtt,
		RttAvg:    stats.AvgRtt,
		RttStdDev: stats.StdDevRtt,
		Duration:  time.Since(start),
	}, nil
}

func setInterface(iface string, state string) {
	exec.Command("/usr/sbin/ip", "link", "set", iface, state).Run()
}

func waitForInterfaceUp(netiface string) bool {
	end := time.Now().Add(TIMEOUT)
	for time.Now().Before(end) {
		iface, err := net.InterfaceByName(netiface)
		if err != nil {
			logger.Error(err.Error())
			return false
		}
		addrs, err := iface.Addrs()
		if err != nil {
			logger.Error(err.Error())
		}
		if iface.Flags&net.FlagRunning != 0 && iface.Flags&net.FlagBroadcast != 0 && len(addrs) > 0 {
			time.Sleep(5 * time.Second)
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func waitForInterfaceDown(netiface string) bool {
	end := time.Now().Add(TIMEOUT)
	for time.Now().Before(end) {
		iface, err := net.InterfaceByName(netiface)
		if err != nil {
			logger.Error(err.Error())
			return false
		}
		addrs, err := iface.Addrs()
		if err != nil {
			logger.Error(err.Error())
		}
		if iface.Flags&net.FlagUp == 0 && iface.Flags&net.FlagRunning == 0 && len(addrs) == 0 {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func testInterface(iface string, host string, pingCount int) TestResult {
	timestamp := time.Now()
	logger.Info(fmt.Sprintf("Testing: %s", iface))
	setInterface(iface, "up")
	logger.Info(fmt.Sprintf("Waiting for %s to go up.", iface))
	if !waitForInterfaceUp(iface) {
		code := "Interface did not go UP!"
		return TestResult{
			Timestamp: timestamp,
			Interface: iface,
			Duration:  time.Since(timestamp),
			Error:     &code,
		}
	}

	logger.Info(fmt.Sprintf("Running ping test on %s.", iface))
	statsPing, err := doPingtest(host, pingCount)
	if err != nil {
		logger.Error(err.Error())
		code := err.Error()
		return TestResult{
			Timestamp: timestamp,
			Interface: iface,
			Duration:  time.Since(timestamp),
			Error:     &code,
		}
	}

	logger.Info(fmt.Sprintf("Running speed test on %s.", iface))
	statsSpeed, err := doSpeedtest()
	if err != nil {
		logger.Error(err.Error())
		code := err.Error()
		return TestResult{
			Timestamp: timestamp,
			Interface: iface,
			Duration:  time.Since(timestamp),
			Error:     &code,
		}
	}

	result := TestResult{
		Timestamp: timestamp,
		Interface: iface,

		SpeedStats: statsSpeed,
		PingStats:  statsPing,

		Duration: time.Since(timestamp),
		Error:    nil,
	}
	return result
}

func appendFile(path string, data string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	if _, err := f.Write([]byte(data)); err != nil {
		logger.Error(err.Error())
		return err
	}
	if err := f.Close(); err != nil {
		logger.Error(err.Error())
		return err
	}
	f.Close()
	return nil
}

func runTest(interfaces []string, host string, pingCount int) {
	if NOTIFY {
		beeep.Alert("ConnectivityStats", fmt.Sprintf("A Connectivity measurement is running for %s.\nYour internet connection will be temporarily unavailable.", strings.Join(interfaces, ", ")), "")
	}

	activeInterfaces := []string{}
	availableInterfaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, iface := range availableInterfaces {
		if iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagUp != 0 {
			activeInterfaces = append(activeInterfaces, iface.Name)
			setInterface(iface.Name, "down")
			waitForInterfaceDown(iface.Name)
		}
	}

	time.Sleep(15 * time.Second)

	for _, iface := range interfaces {
		result := testInterface(iface, host, pingCount)

		for _, line := range strings.Split(result.String(), "\n") {
			logger.Info(line)
		}

		if SAVEFILES {
			appendFile(OUTFILE, result.CSV())
		}

		setInterface(iface, "down")
		waitForInterfaceDown(iface)

	}

	for _, iface := range activeInterfaces {
		setInterface(iface, "up")
	}

	if NOTIFY {
		beeep.Alert("ConnectivityStats", fmt.Sprintf("The Connectivity measurement for %s is finished.", strings.Join(interfaces, ", ")), "")
	}
}

func cleanup() {
	logger.Info("Got call to exit! Ending mesurements and restoring network interfaces...")
	for _, iface := range ACTIVEINTERFACES {
		setInterface(iface, "up")
	}
	os.Exit(0)
}

func main() {
	var selectedInterfaces []string
	var interval int = 30
	var host string = "1.1.1.1"
	var pingCount int = 25
	var oneshot bool = false

	OUTDIR = "."
	OUTFILE = fmt.Sprintf("%s.csv", time.Now().Format(time.RFC3339))
	OUTPATH = path.Join(OUTDIR, OUTFILE)
	SAVEFILES = false
	TIMEOUT = time.Duration(60 * time.Second)
	NOTIFY = false

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	go func() {
		sig := <-sigs
		if sig == syscall.SIGTERM {
			logger.Info("\nReceived signal:", sig)
			cleanup()
			done <- true
		}
	}()

	usr, err := user.Current()

	if err != nil && usr.Uid != "0" {
		logger.Warn("Not running as root. This may cause issues.")
	}

	availableInterfaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	if len(os.Args) <= 1 {
		help()
		return
	}

	args := os.Args[1:]

	for index, value := range args {
		if value == "--interval" {
			iv, err := strconv.Atoi(args[index+1])
			if err != nil {
				panic(err)
			}
			interval = iv
		}

		if value == "--oneshot" {
			interval = -1
		}

		if value == "--interfaces" {
			str := args[index+1]

			if strings.Compare(str, "all") == 0 {
				str := args[index+1]
				if strings.Contains(str, ":") {
					selectedInterfaces = strings.Split(str, "--")
				} else {
					selectedInterfaces = strings.Split(str, ",")
				}
			} else if strings.Contains(str, ":") {
				selectedInterfaces = strings.Split(str, "--")
			} else {
				selectedInterfaces = strings.Split(str, ",")
			}
		}

		if value == "--timeout" {
			ts, err := strconv.Atoi(args[index+1])
			if err != nil {
				panic(err)
			}
			TIMEOUT = time.Duration(ts) * time.Second
		}

		if value == "--pingcount" {
			pc, err := strconv.Atoi(args[index+1])
			if err != nil {
				panic(err)
			}
			pingCount = pc
		}

		if value == "--pinghost" {
			host = args[index+1]
		}

		if value == "--outfile" {
			str := args[index+1]
			if strings.Compare(str[0:1], "/") == 0 {
				OUTDIR = "/"
				OUTFILE = str[1:]
			}

			SAVEFILES = true
		}

		if value == "--outdir" {
			OUTDIR = args[index+1]
			SAVEFILES = true
		}

		if value == "--save" {
			SAVEFILES = true
		}

		if value == "--notify" {
			NOTIFY = true
		}

		if value == "--help" || value == "-?" {
			help()
			return
		}
	}

	OUTPATH = path.Join(OUTDIR, OUTFILE)

	if SAVEFILES {
		_, err := os.Stat(OUTPATH)
		if os.IsNotExist(err) {
			appendFile(OUTPATH, CSVHEADER)
		}
	}

	var interfaces []string = []string{}

	if selectedInterfaces != nil {
		for _, iface := range selectedInterfaces {
			for _, available := range availableInterfaces {
				if strings.Compare(iface, available.Name) == 0 {
					interfaces = append(interfaces, iface)
					continue
				}
			}
		}
	} else {
		for _, available := range availableInterfaces {
			if available.Flags&net.FlagBroadcast != 0 && available.Flags&net.FlagLoopback == 0 {
				interfaces = append(interfaces, available.Name)
				continue
			}
		}
	}

	if oneshot || interval <= 0 {
		logger.Info(fmt.Sprintf("Testing interfaces %s.", strings.Join(interfaces, ", ")))
		if SAVEFILES {
			logger.Info(fmt.Sprintf("Saving data to %s", OUTPATH))
		}
		runTest(interfaces, host, pingCount)
	} else {
		logger.Info(fmt.Sprintf("Testing interfaces %s every %d minutes.", strings.Join(interfaces, ", "), interval))
		if SAVEFILES {
			logger.Info(fmt.Sprintf("Saving data to %s", OUTPATH))
		}
		ticker := time.NewTicker(time.Duration(interval) * time.Minute)

		runTest(interfaces, host, pingCount)

		for range ticker.C {
			runTest(interfaces, host, pingCount)
		}
	}

	for _, iface := range interfaces {
		setInterface(iface, "up")
	}

	<-done

}

func help() {
	fmt.Println("ConectivityStats, by SplitPixl.")
	fmt.Println()
	fmt.Println("ConnectivityStats [--interfaces <if0[,if1]>] [--timeout <N>] [--oneshot] [--interval <N>] [--pinghost <IP>] [--pingcount <N>] [--save] [--outfile <filename>] [--outdir <path>] [--help]")
	fmt.Println()
	fmt.Println("--interfaces <interface_name0[,interface_name1]>")
	fmt.Println("\tList of interfaces to run tests on, delimited by \",\" or \"--\". Use \"all\" for all available interfaces.")
	fmt.Println()
	fmt.Println("--timeout <seconds>")
	fmt.Println("\tHow long to wait for the network inteface to become available in seconds.")
	fmt.Println()
	fmt.Println("--oneshot")
	fmt.Println("\tRun the tests once then exit the program.")
	fmt.Println()
	fmt.Println("--interval <minutes>")
	fmt.Println("\t Interval to run tests on, in minutes. (default: 30)")
	fmt.Println()
	fmt.Println("--pinghost <ip>")
	fmt.Println("\tHost to run ICMP Echo against.")
	fmt.Println()
	fmt.Println("--pingcount <number>")
	fmt.Println("\tNumber of pings to send.")
	fmt.Println()
	fmt.Println("--nofile")
	fmt.Println("\tSave data to a CSV file.")
	fmt.Println()
	fmt.Println("--outfile <filename>")
	fmt.Println("\tFile to output statistics to. Default is timestamp.csv")
	fmt.Println()
	fmt.Println("--outdir <path>")
	fmt.Println("\tDirectory to store files in. Default is CWD.")
	fmt.Println()
	fmt.Println("--notify")
	fmt.Println("\tShow a notification in your GUI when tests are running. (if supported!)")
	fmt.Println()
	fmt.Println("--help, -?")
	fmt.Println("\tDisplay this message.")
	fmt.Println()
}
