# Connectivity Stats

A tool for creating network diagnostic logs.

## Setup

Prerequisites:
- Linux enviroment (i've only tested this in Debian)
- Golang
- iproute2
- Something to parse CSV files
- Something I'm probably forgetting.

Install:
```
cd ConnectivityStats
./install.sh
```

Running:
```
sudo ./ConnectivityStats --help
```

As a service:
```
sudo service connectivitystats@if0 start
```

## Flags

```
./ConnectivityStats [--interfaces <if0[,if1]>] [--timeout <N>] [--oneshot] [--interval <N>] [--pinghost <IP>] [--pingcount <N>] [--save] [--outfile <filename>] [--outdir <path>] [--help]

--interfaces <interface_name0[,interface_name1]>
        List of interfaces to run tests on, comma seperated.

--timeout <seconds>
        How long to wait for the network inteface to become available in seconds.

--oneshot
        Run the tests once then exit the program.

--interval <minutes>
         Interval to run tests on, in minutes. (default: 30)

--pinghost <ip>
        Host to run ICMP Echo against.

--pingcount <number>
        Number of pings to send.

--nofile
        Save data to a CSV file.

--outfile <filename>
        File to output statistics to. Default is timestamp.csv

--outdir <path>
        Directory to store files in. Default is CWD.

--help, -?
        Display this message.
```

## Warning!

**This is a hack!!!** 

Because of how I am tinkering with network interfaces, running with superuser seems to be required. It would be cool if that wasn't the case, but i'm not smart enough to do that yet. Please be careful!

Also, when the connectivity test is running, it will temporarily disconnect you from the internet. Sorry!
