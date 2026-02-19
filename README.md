# OpenShift Performance Dashboard

A web-based dashboard for visualizing and analyzing OpenShift performance metrics collected from kube-burner test runs. The dashboard provides interactive charts and detailed views of performance data organized by jobs, workloads, and metrics.

## Features

- **Multi-level Navigation**: Browse performance data organized by jobs and workloads
- **Multiple Metric Charts**: Display separate charts for each QuantilesMeasurement file (e.g., `podLatencyQuantilesMeasurement`, `svcLatencyQuantilesMeasurement`)
- **Interactive Charts**: 
  - Zoom and pan functionality for detailed analysis
  - Click on data points to view detailed job execution information
  - Individual controls (quantile and metric selectors) for each chart
- **Performance Metrics**: Visualize P99, P95, P50, Min, Max, and Average latency metrics
- **Time Series Analysis**: Track performance trends over time with date-based x-axis
- **Job Summary Details**: View comprehensive job execution details including configuration and metadata

## Project Structure

```
ocp-perf-dash/
├── main.go                 # Main application code
├── go.mod                  # Go module dependencies
├── Makefile               # Build and containerization targets
├── Containerfile          # Container image definition
├── static/                # Static web assets
│   ├── css/
│   │   └── style.css     # Dashboard styling
│   ├── js/
│   │   └── charts.js     # Chart rendering and interaction logic
│   └── img/               # Images (logos, etc.)
├── templates/             # HTML templates
│   ├── jobs.html         # Job listing page
│   └── job_detail.html   # Job/workload detail page with charts
└── test-data/            # Sample test data (optional)
```

## Prerequisites

- Go 1.24 or later
- (Optional) Podman or Docker for container builds

## Installation

### Building from Source

1. Clone the repository:
```bash
git clone <repository-url>
cd ocp-perf-dash
```

2. Build the binary:
```bash
make build
# or
go build -o _output/ocp-perf-dash main.go
```

The binary will be created in the `_output/` directory.

### Building Container Image

Build a container image using Podman:
```bash
make container
```

Or with Docker:
```bash
CONTAINER_CMD=docker make container
```

## Usage

### Running the Dashboard

The dashboard requires a directory containing performance test results. The expected directory structure is:

```
results/
└── <job-name>/
    └── <workload-name>/
        └── <run-identifier>/
            ├── jobSummary.json
            ├── <metric-name>QuantilesMeasurement*.json
            └── ...
```

#### Command Line Options

- `--results-dir`: Path to the directory holding results (default: `results`)
- `--port`: Port to listen on (default: `8080`)

#### Examples

Run with default settings:
```bash
./_output/ocp-perf-dash
```

Run with custom results directory and port:
```bash
./_output/ocp-perf-dash --results-dir /path/to/results --port 9090
```

Run in a container:
```bash
podman run -p 8080:8080 -v /path/to/results:/results:ro \
  quay.io/rsevilla/ocp-perf-dash:latest \
  --results-dir /results --port 8080
```

### Accessing the Dashboard

Once running, open your browser and navigate to:
```
http://localhost:8080
```

## Features in Detail

### Job and Workload Navigation

- **Job List**: View all available performance test jobs
- **Workload Selection**: When a job contains multiple workloads, select from a list
- **Automatic Detection**: The dashboard automatically detects workload directories by looking for `metrics-*` subdirectories

### Chart Features

- **Multiple Charts**: Each QuantilesMeasurement file gets its own chart group
- **Individual Controls**: Each chart has its own:
  - Quantile selector (e.g., Ready, LoadBalancer, PodScheduled)
  - Metric selector (P99, P95, P50, Min, Max, Average)
- **Zoom and Pan**: 
  - Drag to zoom on the x-axis
  - Shift+drag to pan
  - Reset zoom button for each chart
- **Interactive Data Points**: Click on any data point to view detailed job execution information

### Job Summary Modal

Clicking on a chart data point opens a modal showing:
- Run timestamp
- Job configuration details
- General job metadata
- All fields from the job summary

## Development

### Building

```bash
# Build binary
make build

# Build container
make container

# Clean build artifacts
make clean
```

### Dependencies

The project uses:
- [kube-burner](https://github.com/kube-burner/kube-burner) for job summary structure
- [Chart.js](https://www.chartjs.org/) for chart rendering (loaded via CDN)
- Go standard library for HTTP server and file operations

### Code Structure

- `main.go`: HTTP handlers, data loading, and chart data preparation
- `static/js/charts.js`: Client-side chart initialization and interaction
- `templates/`: HTML templates for job listing and detail pages
- `static/css/style.css`: Dashboard styling

## Configuration

The dashboard can be configured via command-line flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--results-dir` | `results` | Path to directory containing performance test results |
| `--port` | `8080` | HTTP server port |

## Contributing

Contributions are welcome! Please ensure:
- Code follows Go best practices
- HTML/CSS/JavaScript is properly formatted
- New features include appropriate documentation

## License

Apache 2.0

## Acknowledgments

- Built for monitoring OpenShift cluster performance
- Uses kube-burner for performance test execution and data collection
