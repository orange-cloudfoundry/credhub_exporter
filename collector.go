package main

import (
	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"time"
)

const (
	beginCertificate = "-----BEGIN CERTIFICATE-----"
	endCertificate   = "-----END CERTIFICATE-----"
)

type certificate struct {
	format   string
	index    int
	notAfter *time.Time
}

type credential struct {
	createdAt time.Time
	name      string
	path      string
	id        string
	credtype  string
	certs     []certificate
}

// CredhubCollector -
type CredhubCollector struct {
	filters                   []*regexp.Regexp
	cli                       *credhub.CredHub
	nameLike                  string
	path                      string
	credentialMetrics         *prometheus.GaugeVec
	certificateExpiresMetrics *prometheus.GaugeVec
	scrapeErrorMetric         prometheus.Gauge
	lastScrapeTimestampMetric prometheus.Gauge
	flushCache                bool
}

// NewCredhubCollector -
func NewCredhubCollector(
	deployment string,
	environment string,
	filters []*regexp.Regexp,
	cli *credhub.CredHub) *CredhubCollector {

	credentialMetrics = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "credhub",
			Subsystem:   "credential",
			Name:        "created_at",
			Help:        "Number of seconds since 1970 since last rotation of credhub credential",
			ConstLabels: prometheus.Labels{"environment": environment, "deployment": deployment},
		},
		[]string{"path", "name", "id", "type"},
	)

	certificateExpiresMetrics = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "credhub",
			Subsystem:   "certificate",
			Name:        "expires_at",
			Help:        "Number of seconds since 1970 until certificate will expire",
			ConstLabels: prometheus.Labels{"environment": environment, "deployment": deployment},
		},
		[]string{"path", "name", "id", "index"},
	)

	scrapeErrorMetric := promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "credhub",
			Subsystem:   "",
			Name:        "last_scrap_error",
			Help:        "Whether the last scrape of Applications metrics from Credhub resulted in an error (1 for error, 0 for success)",
			ConstLabels: prometheus.Labels{"environment": environment, "deployment": deployment},
		},
	)

	lastScrapeTimestampMetric := promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "credhub",
			Subsystem:   "",
			Name:        "last_scrape_timestamp",
			Help:        "Number of seconds since 1970 since last scrape of metrics from credhub.",
			ConstLabels: prometheus.Labels{"environment": environment, "deployment": deployment},
		},
	)

	return &CredhubCollector{
		cli:                       cli,
		filters:                   filters,
		nameLike:                  "",
		path:                      "",
		credentialMetrics:         credentialMetrics,
		certificateExpiresMetrics: certificateExpiresMetrics,
		scrapeErrorMetric:         scrapeErrorMetric,
		lastScrapeTimestampMetric: lastScrapeTimestampMetric,
	}
}

func (c CredhubCollector) filterNameLike(name string) {
	c.nameLike = name
}

func (c CredhubCollector) filterPath(path string) {
	c.path = path
}

func (c CredhubCollector) processCertificates(info *credential, format string, content string) {
	data := []byte(content)
	for idx := 1; len(data) != 0; idx++ {
		block, rest := pem.Decode(data)
		dataStr := strings.TrimSpace(string(rest))
		data = []byte(dataStr)
		if block == nil || block.Bytes == nil {
			log.Errorf("error while reading certificate '%s': invalid pem decode", info.path)
			info.certs = append(info.certs, certificate{
				index: idx,
			})
			return
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			log.Errorf("error while reading certificate '%s' : %s", info.path, err.Error())
			info.certs = append(info.certs, certificate{
				index: idx,
			})
			return
		}
		info.certs = append(info.certs, certificate{
			format:   format,
			index:    idx,
			notAfter: &cert.NotAfter,
		})
	}
}

func (c CredhubCollector) searchCertificate(info *credential, cred credentials.Credential) {
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
		certs = append(certs, certificate)
		raw = raw[stop+len(endCertificate) : len(raw)-1]
	}
	c.processCertificates(info, "%s", strings.Join(certs, "\n"))
}

func (c CredhubCollector) filterCertificates(info *credential, cred credentials.Credential) {
	for _, r := range c.filters {
		if r.MatchString(info.path) {
			log.Debugf("regexp match : '%s'", info.name)
			c.searchCertificate(info, cred)
		}
	}
}

func (c CredhubCollector) run(interval time.Duration) {
	go func() {
		for {
			results, err := c.search()
			if err != nil {
				c.lastScrapeTimestampMetric.Set(float64(time.Now().Unix()))
				c.scrapeErrorMetric.Set(1.0)
			} else {
				creds, errCount := c.analyze(results)
				c.writeMetrics(creds, errCount)
			}
			time.Sleep(interval)
		}
	}()
}

func (c CredhubCollector) update() {
}

func (c CredhubCollector) search() (credentials.FindResults, error) {
	var (
		results credentials.FindResults
		err     error
	)
	log.Debugf("searching creadhub credentials")
	if c.nameLike != "" {
		results, err = c.cli.FindByPartialName(c.nameLike)
	} else if c.path != "" {
		results, err = c.cli.FindByPath(c.path)
	} else {
		results, err = c.cli.FindByPartialName("")
	}
	if err != nil {
		log.Errorf("Error fetching credentials from credhub: %s", err.Error())
	}
	return results, err
}

func (c CredhubCollector) analyze(results credentials.FindResults) ([]credential, int) {
	errors := 0
	creds := []credential{}

	log.Debugf("analyzing %d found credentials", len(results.Credentials))
	for _, cred := range results.Credentials {
		log.Debugf("reading credential '%s'", cred.Name)
		cred, err := c.cli.GetLatestVersion(cred.Name)
		if err != nil {
			log.Errorf("Error fetching credential '%s' from credhub: %s", cred.Name, err.Error())
			errors++
			continue
		}

		datetime, _ := time.Parse(time.RFC3339, cred.VersionCreatedAt)
		parts := strings.Split(cred.Name, "/")
		info := credential{
			createdAt: datetime,
			name:      parts[len(parts)-1],
			path:      cred.Name,
			id:        cred.Id,
			credtype:  cred.Type,
			certs:     []certificate{},
		}

		if cred.Type == "certificate" {
			var data credentials.Certificate
			bytes, _ := cred.MarshalJSON()
			_ = json.Unmarshal(bytes, &data)
			c.processCertificates(&info, "%s-cert", data.Value.Certificate)
			c.processCertificates(&info, "%s-ca", data.Value.Ca)
		} else {
			c.filterCertificates(&info, cred)
		}

		creds = append(creds, info)
	}

	return creds, errors
}

func (c CredhubCollector) writeMetrics(creds []credential, errors int) {
	log.Debugf("writing metrics for  %d analyzed credentials", len(creds))
	c.scrapeErrorMetric.Set(float64(errors))
	c.lastScrapeTimestampMetric.Set(float64(time.Now().Unix()))
	c.credentialMetrics.Reset()
	c.certificateExpiresMetrics.Reset()

	for _, cred := range creds {
		c.credentialMetrics.WithLabelValues(cred.path, cred.name, cred.id, cred.credtype).Set(float64(cred.createdAt.Unix()))
		for _, cert := range cred.certs {
			if cert.notAfter == nil {
				c.scrapeErrorMetric.Add(1.0)
				continue
			}
			index := fmt.Sprintf("%d", cert.index)
			name := fmt.Sprintf(cert.format, cred.name)
			certificateExpiresMetrics.
				WithLabelValues(cred.path, name, cred.id, index).
				Set(float64(cert.notAfter.Unix()))
		}
	}
}

// Local Variables:
// ispell-local-dictionary: "american"
// End:
