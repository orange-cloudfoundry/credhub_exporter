package main

import (
	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"regexp"
	"strings"
	"time"
)

const (
	beginCertificate = "-----BEGIN CERTIFICATE-----"
	endCertificate   = "-----END CERTIFICATE-----"
)

// CredhubCollector -
type CredhubCollector struct {
	filters                   []*regexp.Regexp
	cli                       *credhub.CredHub
	credentialMetrics         *prometheus.GaugeVec
	certificateExpiresMetrics *prometheus.GaugeVec
	scrapeErrorMetric         prometheus.Gauge
}

// NewCredhubCollector -
func NewCredhubCollector(
	director string,
	environment string,
	filters []*regexp.Regexp,
	cli *credhub.CredHub) *CredhubCollector {

	credentialMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "credhub",
			Subsystem:   "credential",
			Name:        "created_at",
			Help:        "Credhub credential generation unix timestamp (seconds from epoch)",
			ConstLabels: prometheus.Labels{"environment": environment, "director": director},
		},
		[]string{"path", "name", "id"},
	)

	certificateExpiresMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "credhub",
			Subsystem:   "certificate",
			Name:        "expires_at",
			Help:        "Credhub certificate expire unix timestamp (seconds from epoch)",
			ConstLabels: prometheus.Labels{"environment": environment, "director": director},
		},
		[]string{"path", "name", "id", "index"},
	)

	scrapeErrorMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "credhub",
			Subsystem:   "",
			Name:        "last_scrap_error",
			Help:        "Whether the last scrape of Applications metrics from Credhub resulted in an error (1 for error, 0 for success)",
			ConstLabels: prometheus.Labels{"environment": environment, "director": director},
		},
	)

	return &CredhubCollector{
		cli:                       cli,
		filters:                   filters,
		credentialMetrics:         credentialMetrics,
		certificateExpiresMetrics: certificateExpiresMetrics,
		scrapeErrorMetric:         scrapeErrorMetric,
	}
}

func (c CredhubCollector) processCertificates(path string, name string, id string, certificates string) error {
	data := []byte(certificates)
	for idx := 1; len(data) != 0; idx++ {
		block, rest := pem.Decode(data)
		data = rest
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			c.scrapeErrorMetric.Add(1.0)
			log.Errorf("error while reading certificate '%s' : %s", path, err.Error())
			return err
		}
		c.certificateExpiresMetrics.WithLabelValues(path, name, id, fmt.Sprintf("%d", idx)).Set(float64(cert.NotAfter.Unix()))
	}
	return nil
}

func (c CredhubCollector) searchCertificate(name string, cred credentials.Credential) error {
	log.Debugf("searching for certificates in credential '%s'", cred.Name)
	bytes, _ := cred.MarshalJSON()
	raw := string(bytes)
	raw = strings.Replace(raw, "\\n", "\n", -1)
	certs := []string{}
	for start := 0; start != -1; {
		start := strings.Index(raw, beginCertificate)
		stop := strings.Index(raw, endCertificate)
		if start == -1 || stop == -1 {
			break
		}
		certificate := raw[start : stop+len(endCertificate)]
		log.Debugf("extract certificate: [%s]", certificate)
		certs = append(certs, certificate)
		raw = raw[stop+len(endCertificate) : len(raw)-1]
	}
	return c.processCertificates(cred.Name, name, cred.Id, strings.Join(certs, "\n"))
}

func (c CredhubCollector) filterCertificates(name string, cred credentials.Credential) {
	for _, r := range c.filters {
		if r.MatchString(cred.Name) {
			log.Debugf("regexp match : '%s'", cred.Name)
			c.searchCertificate(name, cred)
		}
	}
}

func (c CredhubCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debugf("collecting credhub metrics")
	c.scrapeErrorMetric.Set(0.0)

	results, err := c.cli.FindByPartialName("")
	if err != nil {
		log.Errorf("Error fethings credentials from credhub: %s", err.Error())
		c.scrapeErrorMetric.Set(1.0)
		return
	}
	log.Debugf("found %d metrics", len(results.Credentials))

	for _, cred := range results.Credentials {
		log.Debugf("reading credential '%s'", cred.Name)
		cred, err := c.cli.GetLatestVersion(cred.Name)
		if err != nil {
			c.scrapeErrorMetric.Add(1.0)
			log.Errorf("Error fethings credential '%s' from credhub: %s", cred.Name, err.Error())
			continue
		}

		datetime, _ := time.Parse(time.RFC3339, cred.VersionCreatedAt)
		parts := strings.Split(cred.Name, "/")
		name := parts[len(parts)-1]
		c.credentialMetrics.WithLabelValues(cred.Name, name, cred.Id).Set(float64(datetime.Unix()))

		if cred.Type == "certificate" {
			var data credentials.Certificate
			bytes, _ := cred.MarshalJSON()
			_ = json.Unmarshal(bytes, &data)
			c.processCertificates(cred.Name, name+"-cert", cred.Id, data.Value.Certificate)
			c.processCertificates(cred.Name, name+"-ca", cred.Id, data.Value.Ca)
		} else {
			c.filterCertificates(name, cred)
		}
	}

	c.credentialMetrics.Collect(ch)
	c.certificateExpiresMetrics.Collect(ch)
	c.scrapeErrorMetric.Collect(ch)
}

func (c CredhubCollector) Describe(ch chan<- *prometheus.Desc) {
	c.credentialMetrics.Describe(ch)
	c.certificateExpiresMetrics.Describe(ch)
}
