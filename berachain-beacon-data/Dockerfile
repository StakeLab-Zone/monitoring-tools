FROM python:3.10-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    default-libmysqlclient-dev \
    gcc \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first to leverage Docker cache
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy the ingestion script and validator mappings
COPY beacon-ingestion.py .
COPY validators.yaml .

FROM python:3.10-slim
WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    default-libmysqlclient-dev \
    gcc \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first to leverage Docker cache
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy the ingestion script and default validator mappings
COPY beacon-ingestion.py .
COPY validators.yaml /app/default-validators.yaml

# Create entrypoint script that handles Kubernetes termination gracefully
# and checks for custom validators.yaml
RUN echo '#!/bin/bash\n\
trap "exit" TERM INT\n\
\n\
# Set default validators file path\n\
VALIDATORS_FILE=${VALIDATORS_FILE:-"/app/validators.yaml"}\n\
\n\
# Check if custom validators file exists in config directory\n\
if [ -f "/app/config/validators.yaml" ]; then\n\
    VALIDATORS_FILE="/app/config/validators.yaml"\n\
fi\n\
\n\
python beacon-ingestion.py \
    --db-type mysql \
    --host "${MYSQL_HOST}" \
    --user "${MYSQL_USER}" \
    --password "${MYSQL_PASSWORD}" \
    --database "${MYSQL_DATABASE}" \
    --port "${MYSQL_PORT}" \
    --rpc-url "${RPC_URL}" \
    --batch-size "${BATCH_SIZE:-100}" \
    --workers "${WORKERS:-5}" \
    --pool-size "${POOL_SIZE:-10}" \
    --validator-mappings "${VALIDATORS_FILE}"' > /app/entrypoint.sh \
    && chmod +x /app/entrypoint.sh

# Create directory for potential ConfigMap mounting
RUN mkdir -p /app/config

ENTRYPOINT ["/app/entrypoint.sh"]
