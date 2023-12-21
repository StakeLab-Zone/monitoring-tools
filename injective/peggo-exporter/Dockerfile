FROM python:3.9-slim

WORKDIR /usr/src/app

COPY . .

RUN pip install --no-cache-dir -r requirements.txt

EXPOSE 8000

# Set default environment variables (can be overridden)
ENV API_ENDPOINT=https://lcd.injective.network

CMD ["python", "./injective-peggo-prometheus-exporter.py"]
