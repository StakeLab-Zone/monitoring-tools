# Umee Votes Inspector

## Overview

The Umee Votes Inspector is a Python application designed to monitor and report the voting status of an operator in the Umee network. 
It periodically checks for missing votes and exposes this data as a Prometheus metric. 

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
   cd umee/umee-pricefeeder-exporter
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

Make sure to set the environment variables APIURL and OPERATOR_ADDRESS as needed.

Using Docker:
Build and run the application using Docker:

```
docker build -t umee-pricefeeder-exporter .
docker run -p 8000:8000 umee-pricefeeder-exporter
```

**Configuration**

The application can be configured using the following environment variables:

    APIURL: URL of the Umee API (default: api-umee.cosmos-spaces.cloud).
    OPERATOR_ADDRESS: Operator address to monitor (default: umeevaloper1gxgsq7jpkg0f8vfkk60c4rd30z4cgs6lyqtglf).

**Prometheus Metrics**

Access the Prometheus metrics at http://localhost:8000.
