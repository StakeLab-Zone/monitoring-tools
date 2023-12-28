# Peggo Injective exporter

This peggo exporter is exposing (with Prometheus format) the number of events missed.

## Features

- **Vote Monitoring**: Checks for missing votes from a specified operator.
- **Prometheus Integration**: Exposes the count of missing votes as a metric for Prometheus.
- **Scheduled Execution**: Runs the inspection every 5 minutes.
- **Logging**: Detailed logging of operations and errors.

## Getting Started

### Prerequisites

- Python 3.9 or later.
- Docker (optional for containerized deployment).

### Installation

**Clone the Repository:**
   
   ```
   git clone https://github.com/StakeLab-Zone/monitoring-tools.git
   cd injective/peggo-exporter
  ```
    
**Install Dependencies:**

If you are not using Docker, install the dependencies manually:
  ```
  pip install -r requirements.txt
  ```

**Usage**

Direct Execution:

Run the script directly using Python:
  ```
  python umee-pricefeeder-exporter.py
  ```

Make sure to set the environment variables API_ENDPOINT and ORCHESTRATOR_ADDRESS as needed. CHECK_INTERVAL is optional.

Using Docker:

Build and run the application using Docker:

```
docker build -t peggo-exporter:latest .
docker run -p 8000:8000 peggo-exporter:latest
```

**Configuration**

The application can be configured using the following environment variables:

    API_ENDPOINT: URL of the Umee API (default: https://lcd.injective.network).
    ORCHESTRATOR_ADDRESS: Orchestrator address to monitor (default: inj1xvnlzs3xw2fy3y2jzj3kck0n532jx9kltvmm8k).
    CHECK_INTERVAL: Check interval in seconds (default: 300).

**Kubernetes**

For Kubernetes users, here's an Helm chart [here](https://github.com/StakeLab-Zone/StakeLab/tree/main/Charts/peggo-exporter).

**Prometheus Metrics**

Access the Prometheus metrics at http://localhost:8000.

**Grafana dashboard**

![Peggo Injective Grafana Dashboard](../injective/grafana/peggo-injective-grafana.jpg)

## TODO

- Grafana dashboard, alerts examples
