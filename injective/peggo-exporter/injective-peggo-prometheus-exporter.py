import os
import logging
import requests
from prometheus_client import start_http_server, Gauge
import time

# Initialize logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

# Initialize a Prometheus Gauge
LATENCY_GAUGE = Gauge('injective_peggo_latency', 'Orchestrator latency of the Injective Peggo')
CHECK_INTERVAL = int(os.getenv('CHECK_INTERVAL', 300))

def main():
    # Read environment variables
    orchestrator_inj_address = os.getenv('ORCHESTRATOR_ADDRESS', 'inj1xvnlzs3xw2fy3y2jzj3kck0n532jx9kltvmm8k')
    api_base_url = os.getenv('API_ENDPOINT', 'https://lcd.injective.network/')

    batch_response = requests.get(f"{api_base_url}/peggy/v1/batch/last?address={orchestrator_inj_address}")
    if batch_response.status_code != 200:
        logging.error(f"Failed to fetch batch data. Status code: {batch_response.status_code}")
        logging.error(f"Response: {batch_response.text}")
        return
    batch_data = batch_response.json()
    if 'batch' in batch_data and batch_data['batch'] is not None:
        if len(batch_data['batch']) == 0:
            logging.info(f"No pending batches")
        else:
            logging.info(f"No pending batches")
            logging.info(f"{batch_data}")
    else:
        logging.error(f"No 'batch' data found in response")

    valsets_response = requests.get(f"{api_base_url}/peggy/v1/valset/last?address={orchestrator_inj_address}")
    if valsets_response.status_code != 200:
        logging.error(f"Failed to fetch valsets data. Status code: {valsets_response.status_code}")
        logging.error(f"Response: {valsets_response.text}")
        return
    valsets_data = valsets_response.json()
    if 'valsets' in valsets_data and valsets_data['valsets'] is not None:
        if len(valsets_data['valsets']) == 0:
            logging.info("No Pending Valsets")
        else:
            logging.info("result:")
            logging.info(valsets_data)
    else:
        logging.error("No 'valsets' data found in response.")

    lon_response = requests.get(f"{api_base_url}/peggy/v1/module_state")
    lce_response = requests.get(f"{api_base_url}/peggy/v1/oracle/event/{orchestrator_inj_address}")

    if lon_response.status_code == 200 and lce_response.status_code == 200:
        lon_data = lon_response.json()
        lce_data = lce_response.json()

        lon = int(lon_data['state']['last_observed_nonce']) if 'state' in lon_data and 'last_observed_nonce' in lon_data['state'] else None
        lce = int(lce_data['last_claim_event']['ethereum_event_nonce']) if 'last_claim_event' in lce_data and 'ethereum_event_nonce' in lce_data['last_claim_event'] else None
        delay = lce - lon
        LATENCY_GAUGE.set(delay)

    else:
        if lon_response.status_code != 200:
            logging.error(f"Failed to fetch data for last observed nonce. Status code: {lon_response.status_code}")
        if lce_response.status_code != 200:
            logging.error(f"Failed to fetch data for last claim event. Status code: {lce_response.status_code}")


if __name__ == "__main__":
    start_http_server(8000)
    while True:
        main()
        logging.info(f"Sleeping for {CHECK_INTERVAL} seconds before the next run.")
        time.sleep(CHECK_INTERVAL)
