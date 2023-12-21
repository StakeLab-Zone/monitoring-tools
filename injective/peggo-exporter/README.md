# Peggo Injective exporter

This peggo exporter is exposing (with Prometheus format) the number of events missed.

## Usage

Build the docker image:

`docker build -t peggo-exporter:latest .`

Run the docker image:

`docker run -p 8000:8000 -e ORCHESTRATOR_ADDRESS=inj1xvnlzs3xw2fy3y2jzj3kck0n532jx9kltvmm8k peggo-exporter:0.0.1 -e ENV API_ENDPOINT=https://lcd.injective.network peggo-exporter:latest`

## TODO

- Helm chart for Kubernetes
- Grafana dashboard, alerts examples