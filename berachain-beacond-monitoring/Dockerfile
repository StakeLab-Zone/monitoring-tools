FROM python:3.10-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY beacond-monitor.py .

# Run the script
ENTRYPOINT ["python", "beacond-monitor.py"]