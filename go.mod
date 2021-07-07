module github.com/orange-cloudfoundry/credhub_exporter

go 1.15

require (
	code.cloudfoundry.org/credhub-cli v0.0.0-20210705130154-e13c0e0f2a0a
	github.com/alecthomas/units v0.0.0-20210208195552-ff826a37aa15 // indirect
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.29.0
	github.com/prometheus/procfs v0.7.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)
