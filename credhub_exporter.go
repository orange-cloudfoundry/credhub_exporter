package main

import (
	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/auth"
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
)

var (
	apiURL = kingpin.Flag(
		"credhub.api-url", "Credhub API URL ($CREDHUB_EXPORTER_API_URL)",
	).Envar("CREDHUB_EXPORTER_API_URL").Required().String()

	// authURL = kingpin.Flag(
	// 	"credhub.auth-url", "Credhub Authentication URL ($CREDHUB_EXPORTER_AUTH_URL)",
	// ).Envar("CREDHUB_EXPORTER_AUTH_URL").Required().String()

	clientID = kingpin.Flag(
		"credhub.client-id", "Credhub Client ID ($CREDHUB_EXPORTER_CLIENT_ID)",
	).Envar("CREDHUB_EXPORTER_CLIENT_ID").String()

	clientSecret = kingpin.Flag(
		"credhub.client-secret", "Credhub Client Secret ($CREDHUB_EXPORTER_CLIENT_SECRET)",
	).Envar("CREDHUB_EXPORTER_CLIENT_SECRET").Required().String()

	caCertPath = kingpin.Flag(
		"credhub.ca-cert-path", "Credhub Client CA certificate path ($CREDHUB_EXPORTER_CA_CERT_PATH)",
	).Envar("CREDHUB_EXPORTER_CA_CERT_PATH").String()

	credhubProxy = kingpin.Flag(
		"credhub.proxy", "Credhub Client Secret ($CREDHUB_EXPORTER_CLIENT_SECRET)",
	).Envar("CREDHUB_EXPORTER_CLIENT_SECRET").Default("").String()

	genericCertificateFilter = kingpin.Flag(
		"filters.generic-certificates", "Json list of <regexp> to match generic credentials paths that may contains certificates",
	).Default("[]").String()

	metricsEnvironment = kingpin.Flag(
		"metrics.environment", "Environment label to be attached to metrics ($CF_EXPORTER_METRICS_ENVIRONMENT)",
	).Envar("CF_EXPORTER_METRICS_ENVIRONMENT").Required().String()

	skipSSLValidation = kingpin.Flag(
		"skip-ssl-verify", "Disable SSL Verify ($CF_EXPORTER_SKIP_SSL_VERIFY)",
	).Envar("CF_EXPORTER_SKIP_SSL_VERIFY").Default("false").Bool()

	listenAddress = kingpin.Flag(
		"web.listen-address", "Address to listen on for web interface and telemetry ($CF_EXPORTER_WEB_LISTEN_ADDRESS)",
	).Envar("CF_EXPORTER_WEB_LISTEN_ADDRESS").Default(":9357").String()

	metricsPath = kingpin.Flag(
		"web.telemetry-path", "Path under which to expose Prometheus metrics ($CF_EXPORTER_WEB_TELEMETRY_PATH)",
	).Envar("CF_EXPORTER_WEB_TELEMETRY_PATH").Default("/metrics").String()

	authUsername = kingpin.Flag(
		"web.auth.username", "Username for web interface basic auth ($CF_EXPORTER_WEB_AUTH_USERNAME)",
	).Envar("CF_EXPORTER_WEB_AUTH_USERNAME").String()

	authPassword = kingpin.Flag(
		"web.auth.password", "Password for web interface basic auth ($CF_EXPORTER_WEB_AUTH_PASSWORD)",
	).Envar("CF_EXPORTER_WEB_AUTH_PASSWORD").String()

	tlsCertFile = kingpin.Flag(
		"web.tls.cert_file", "Path to a file that contains the TLS certificate (PEM format). If the certificate is signed by a certificate authority, the file should be the concatenation of the server's certificate, any intermediates, and the CA's certificate ($CF_EXPORTER_WEB_TLS_CERTFILE)",
	).Envar("CF_EXPORTER_WEB_TLS_KEYFILE").ExistingFile()

	tlsKeyFile = kingpin.Flag(
		"web.tls.key_file", "Path to a file that contains the TLS private key (PEM format) ($CF_EXPORTER_WEB_TLS_KEYFILE)",
	).Envar("CF_EXPORTER_WEB_TLS_KEYFILE").ExistingFile()

	metricsNamespace = kingpin.Flag(
		"metrics.namespace", "Metrics Namespace ($CF_EXPORTER_METRICS_NAMESPACE)",
	).Envar("CF_EXPORTER_METRICS_NAMESPACE").Default("credhub").String()
)

func init() {
	prometheus.MustRegister(version.NewCollector(*metricsNamespace))
}

type basicAuthHandler struct {
	handler  http.HandlerFunc
	username string
	password string
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok || username != h.username || password != h.password {
		log.Errorf("Invalid HTTP auth from `%s`", r.RemoteAddr)
		w.Header().Set("WWW-Authenticate", "Basic realm=\"metrics\"")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.handler(w, r)
}

func prometheusHandler() http.Handler {
	handler := prometheus.Handler()

	if *authUsername != "" && *authPassword != "" {
		handler = &basicAuthHandler{
			handler:  prometheus.Handler().ServeHTTP,
			username: *authUsername,
			password: *authPassword,
		}
	}

	return handler
}

var (
	credentialMetrics         *prometheus.GaugeVec
	certificateExpiresMetrics *prometheus.GaugeVec
)

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("credhub_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting credhub_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	if len(*credhubProxy) != 0 {
		_ = os.Setenv("CREDHUB_PROXY", *credhubProxy)
	}

	credhubCli, err := credhub.New(*apiURL,
		credhub.SkipTLSValidation(*skipSSLValidation),
		credhub.Auth(auth.Uaa(
			*clientID,
			*clientSecret,
			"",
			"",
			"",
			"",
			true,
		)))

	if len(*caCertPath) != 0 {
		b, err := ioutil.ReadFile(*caCertPath)
		if err != nil {
			log.Errorf("unable to read file '%s' : %s", *caCertPath, err.Error())
			os.Exit(1)
		}
		credhub.CaCerts(string(b))(credhubCli)
	}

	if err != nil {
		log.Errorf("Error creating Credhub client: %s", err.Error())
		os.Exit(1)
	}

	regexps := []string{}
	if err = json.Unmarshal([]byte(*genericCertificateFilter), &regexps); err != nil {
		log.Errorf("invalid json in --filters.generic-certificates parameter : %s", err.Error())
		os.Exit(1)
	}

	filters := []*regexp.Regexp{}
	for _, r := range regexps {
		exp, err := regexp.Compile(r)
		if err != nil {
			log.Errorf("could not compile given regexp '%s' : %s", r, err.Error())
			os.Exit(1)
		}
		filters = append(filters, exp)
	}

	// todo cacert
	credhubCollector := NewCredhubCollector("sph-xmt", *metricsEnvironment, filters, credhubCli)
	prometheus.MustRegister(credhubCollector)

	handler := prometheusHandler()
	http.Handle(*metricsPath, handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Credhub Exporter</title></head>
             <body>
             <h1>Credhub Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	if *tlsCertFile != "" && *tlsKeyFile != "" {
		log.Infoln("Listening TLS on", *listenAddress)
		log.Fatal(http.ListenAndServeTLS(*listenAddress, *tlsCertFile, *tlsKeyFile, nil))
	} else {
		log.Infoln("Listening on", *listenAddress)
		log.Fatal(http.ListenAndServe(*listenAddress, nil))
	}
}
