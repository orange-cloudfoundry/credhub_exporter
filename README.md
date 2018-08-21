# Credhub Prometheus Exporter [![Build Status](https://travis-ci.org/orange-cloudfoundry/credhub_exporter.png)](https://travis-ci.org/orange-cloudfoundry/credhub_exporter)

A [Prometheus][prometheus] exporter for [Credhub][credhub]. The exporter currently only exports metrics related to credhub objects, including [certificate](https://github.com/cloudfoundry-incubator/credhub/blob/master/docs/credential-types.md) objects (it does not yet provide metrics about the credhub server health such as error rates, response times, or total number of entries).


## Installation

### Binaries

Download the already existing [binaries][binaries] for your platform:

```bash
$ ./credhub_exporter <flags>
```

### From source

Using the standard `go install` (you must have [Go][golang] already installed in your local machine):

```bash
$ go install github.com/orange-cloudfoundry/credhub_exporter
$ credhub_exporter <flags>
```

### Docker

To run the credhub exporter as a Docker container, run:

```bash
$ docker run -p 9358:9358 orangeopensource/credhub-exporter <flags>
```

<!-- ### BOSH -->

<!-- This exporter can be deployed using the [Prometheus BOSH Release][prometheus-boshrelease]. -->

## Usage

### UAA Client

In order to connect to the [Credhub API][credhub_api] a `client-id` and `client-secret` must be provided. The `client-id` must have the `creadhub.read` authority.

For example, to create a new `client-id` and `client-secret` with the right permissions:

```bash
uaac target https://<YOUR UAA URL> --skip-ssl-validation
uaac token client get <YOUR ADMIN CLIENT ID> -s <YOUR ADMIN CLIENT SECRET>
uaac client add prometheus-credhub \
  --name prometheus-credhub \
  --secret prometheus-credhub-client-secret \
  --authorized_grant_types client_credentials,refresh_token \
  --authorities creadhub.read,creadhub.write
```

### Flags

| Flag / Environment Variable                                                 | Required | Default    | Description                                                                                                                                                                                                                           |
| ---------------------------                                                 | -------- | -------    | -----------                                                                                                                                                                                                                           |
| `credhub.api_url`<br />`CREDHUB_EXPORTER_API_URL`                           | Yes      |            | Credhub API URL                                                                                                                                                                                                                       |
| `credhub.client-id`<br />`CREDHUB_EXPORTER_CLIENT_ID`                       | Yes      |            | Credhub Client ID (must have the `credhub.read` scope)                                                                                                                                                                                |
| `credhub.client-secret`<br />`CREDHUB_EXPORTER_CLIENT_SECRET`               | Yes      |            | Credhub Client Secret                                                                                                                                                                                                                 |
| `credhub.proxy`<br />`CREDHUB_EXPORTER_PROXY`                               | No       |            | Socks proxy to open before connecting to credub                                                                                                                                                                                       |
| `credhub.ca-certs-path`<br />`CREDHUB_EXPORTER_CA_CERTS_PATH`               | No       |            | Path to CA certificate to use when connecting credhub                                                                                                                                                                                 |
| `filters.name-like`<br />`CREDHUB_EXPORTER_FILTER_NAMELIKE`                 | No       |            | Fetch from server credentials whose name contains the [query string](https://credhub-api.cfapps.io/#find-credentials) (fetch all credentials when empty)                                                                                                                                             |
| `filters.path`<br />`CREDHUB_EXPORTER_FILTER_PATH`                          | No       |            | Fetch from server credentials that exist under the provided path (ignored when `--filters.name-like` is not empty)                                                                                                                                  |
| `filters.generic-certificates`<br />`CREDHUB_EXPORTER_GENERIC_CERTIFICATES` | No       | `[]`       | Json list of \<regexp\> to match against name of certificate objects fetched from server. Only certificate objects whose name match at least one regexp will have an associated metric emitted.                                                                                                                                               |
| `metrics.deployment-name`<br />`CREDHUB_EXPORTER_METRICS_DEPLOYMENT`        | Yes      |            | Credhub Bosh Deployment Name to be reported as the `deployment` metric label                                                                                                                                                          |
| `metrics.namespace`<br />`CREDHUB_EXPORTER_METRICS_NAMESPACE`               | No       | `credhub`  | Metrics Namespace                                                                                                                                                                                                                     |
| `metrics.environment`<br />`CREDHUB_EXPORTER_METRICS_ENVIRONMENT`           | Yes      |            | Credhub `environment` label to be attached to metrics                                                                                                                                                                                 |
| `skip-ssl-verify`<br />`CREDHUB_EXPORTER_SKIP_SSL_VERIFY`                   | No       | `false`    | Disable SSL Verify                                                                                                                                                                                                                    |
| `web.listen-address`<br />`CREDHUB_EXPORTER_WEB_LISTEN_ADDRESS`             | No       | `:9358`    | Address to listen on for web interface and telemetry                                                                                                                                                                                  |
| `web.telemetry-path`<br />`CREDHUB_EXPORTER_WEB_TELEMETRY_PATH`             | No       | `/metrics` | Path under which to expose Prometheus metrics                                                                                                                                                                                         |
| `web.auth.username`<br />`CREDHUB_EXPORTER_WEB_AUTH_USERNAME`               | No       |            | Username for web interface basic auth                                                                                                                                                                                                 |
| `web.auth.password`<br />`CREDHUB_EXPORTER_WEB_AUTH_PASSWORD`               | No       |            | Password for web interface basic auth                                                                                                                                                                                                 |
| `web.tls.cert_file`<br />`CREDHUB_EXPORTER_WEB_TLS_CERTFILE`                | No       |            | Path to a file that contains the TLS certificate (PEM format). If the certificate is signed by a certificate authority, the file should be the concatenation of the server's certificate, any intermediates, and the CA's certificate |
| `web.tls.key_file`<br />`CREDHUB_EXPORTER_WEB_TLS_KEYFILE`                  | No       |            | Path to a file that contains the TLS private key (PEM format)                                                                                                                                                                         |


### Metrics

The exporter returns the following credhub objects metrics:

| Metric                                     | Description                                                            | Labels                                                   |
| ------                                     | -----------                                                            | ------                                                   |
| *metrics.namespace*_credential_created_at  | Unix timestamp of the creation of the last version of a given credential | `deployment`, `environment`, `id`, `name`, `path`          |
| *metrics.namespace*_certificate_expires_at | Unix timestamp of the expiration time of a given certificate                   | `deployment`, `environment`, `id`, `name`, `path`, `index` |
| *metrics.namespace*_last_scrap_error       | Number of credentials that the exporter failed to read during last scrape      | `deployment`, `environment`                                |

## Contributing

Refer to the [contributing guidelines][contributing].

## License

Apache License 2.0, see [LICENSE][license].

[binaries]: https://github.com/orange-cloudfoundry/credhub_exporter/releases
[credhub]: https://github.com/cloudfoundry-incubator/credhub
[credhub_api]: https://credhub-api.cfapps.io/
[cloudfoundry]: https://www.cloudfoundry.org/
[contributing]: https://github.com/orange-cloudfoundry/credhub_exporter/blob/master/CONTRIBUTING.md
[faq]: https://github.com/bosh-prometheus/credhub_exporter/blob/master/FAQ.md
[golang]: https://golang.org/
[license]: https://github.com/orange-cloudfoundry/credhub_exporter/blob/master/LICENSE
[prometheus]: https://prometheus.io/
[prometheus-boshrelease]: https://github.com/bosh-prometheus/prometheus-boshrelease
