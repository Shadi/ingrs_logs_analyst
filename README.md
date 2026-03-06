# ingrs_logs_analyst

Analyse ingress-nginx access logs. I discovered that I got a lot of spam on one of the sites in the cluster and the original
goal of this was to find out the source of these reqests, after that I improved it further to act as a lightweight server-side traffic analysis.

Serves a web UI for exploring requests by site, path, IP, and status code with IP geolocation via IP2Location.

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--source` | `-s` | `k8s` | Log source: `k8s` or `file` |
| `--file` | `-f` | `access.log` | Log file path (when `--source=file`) |
| `--namespace` | `-n` | `ingress-nginx` | Kubernetes namespace (when `--source=k8s`) |
| `--deployment` | `-d` | `ingress-nginx-controller` | Deployment name (when `--source=k8s`) |
| `--kubeconfig` | `-k` | `~/.kube/config` | Path to kubeconfig file (when `--source=k8s`) |
| `--ipdb` | `-i` | `IP2LOCATION-DB11.BIN` | Path to IP2Location `.BIN` database |

## IP2Location

For mapping IP address to a country the library [github.com/ip2location/ip2location-go](https://github.com/ip2location/ip2location-go) is used, you can sign up and get a lite DB for free to use it with
this program.

## Running

### Go

```sh
# from k8s (default)
go run . --ipdb IP2LOCATION-DB11.BIN

# from a log file
go run . --source file --file access.log --ipdb IP2LOCATION-DB11.BIN
```

### Podman

```sh
# build
podman build -t ingrs_logs_analyst .

# from k8s
podman run -p 8080:8080 \
  -v ~/.kube/config:/root/.kube/config:ro \
  -v /path/to/IP2LOCATION.BIN:/data/IP2LOCATION.BIN:ro \
  ingrs_logs_analyst --ipdb /data/IP2LOCATION.BIN

# from a log file
podman run -p 8080:8080 \
  -v /path/to/access.log:/data/access.log:ro \
  -v /path/to/IP2LOCATION.BIN:/data/IP2LOCATION.BIN:ro \
  ingrs_logs_analyst --source file --file /data/access.log --ipdb /data/IP2LOCATION.BIN
```

The web UI is available at `http://localhost:8080`.
