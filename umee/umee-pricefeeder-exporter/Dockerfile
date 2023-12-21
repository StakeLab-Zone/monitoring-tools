FROM python:3.9-slim

WORKDIR /usr/src/app

COPY . .

RUN pip install --no-cache-dir -r requirements.txt

EXPOSE 8000

# Run the application when the container launches
CMD ["python", "./umee-pricefeeder-exporter.py"]
