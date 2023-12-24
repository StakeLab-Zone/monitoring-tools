import requests
import json
import sys
import logging
import time
import os
from prometheus_client import Counter, start_http_server, Gauge

# Initialize Prometheus metrics
REQUESTS_TOTAL = Counter('requests_total', 'Total number of requests made')
ERRORS_TOTAL = Counter('errors_total', 'Total number of errors encountered')
UMEE_VOTES_GAUGE = Gauge('umee_missing_votes', 'Umee missing votes')
UMEE_TOTAL_VOTES_GAUGE = Gauge('umee_total_votes', 'Umee total votes')
CHECK_INTERVAL = int(os.getenv('CHECK_INTERVAL', 300))

# Initialize logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

# Environment variables
APIURL = os.getenv('APIURL', 'api-umee.cosmos-spaces.cloud')
OPERATOR_ADDRESS = os.getenv('OPERATOR_ADDRESS', 'umeevaloper1gxgsq7jpkg0f8vfkk60c4rd30z4cgs6lyqtglf')

def log_error(r):
    logging.error(f"Error on req {r.status_code}")
    ERRORS_TOTAL.inc()
    sys.exit(-1)

def get_req(url):
    r = requests.get(url=url)
    REQUESTS_TOTAL.inc()
    if r.status_code == 200:
        data = r.json()
        if len(data['aggregate_votes']) > 0:
            return data 
        return get_req(url)
    log_error(r)

def get_moniker(validators, valaddr):
    for validator in validators:
        if validator['operator_address'] == valaddr:
            return validator['description']['moniker']
    return None

def get_validators(APIURL):
    r = requests.get(url=f"https://{APIURL}/cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED")
    REQUESTS_TOTAL.inc()
    if r.status_code == 200:
        data = r.json()
        return data['validators']      
    log_error(r)

def get_accepted_denoms(APIURL):
    r = requests.get(url=f"https://{APIURL}/umee/oracle/v1/params")
    REQUESTS_TOTAL.inc()
    if r.status_code == 200:
        data = r.json()
        return data['params']['accept_list']
    log_error(r)

def votes_inspector(APIURL, operator_address):
    votes = f"https://{APIURL}/umee/oracle/v1/validators/aggregate_votes"

    denoms_resp = get_accepted_denoms(APIURL)
    accepted_denoms = [denom['symbol_denom'].upper() for denom in denoms_resp]
    accepted_denoms = list(set(accepted_denoms))

    validators = get_validators(APIURL=APIURL)

    votes_resp = get_req(votes)
    aggregate_votes = votes_resp['aggregate_votes']
    
    UMEE_TOTAL_VOTES_GAUGE.set(len(denoms_resp))

    operator_moniker = get_moniker(validators, operator_address)
    if operator_moniker is None:
        logging.info(f"Operator address not found. Address: {operator_address}")
        return

    for vote in aggregate_votes:
        if vote['voter'] == operator_address:
            exchange_rate_tuples = [tuple['denom'] for tuple in vote['exchange_rate_tuples']]
            missing_votes = len(set(accepted_denoms) - set(exchange_rate_tuples))
            UMEE_VOTES_GAUGE.set(missing_votes)
            message = f"Operator {operator_moniker} has {missing_votes} missing votes."
            logging.info(message)
            for denom in accepted_denoms:
                if denom not in exchange_rate_tuples:
                    logging.info(f"Operator {operator_moniker} is missing votes for denom: {denom}")
            break
    else:
        missing_votes = len(accepted_denoms)
        UMEE_VOTES_GAUGE.set(missing_votes)
        message = f"❌ Operator {operator_moniker} did not submit any votes."
        logging.warning(message)

if __name__ == "__main__":
    start_http_server(8000)
    logging.info(f"Used API: {APIURL}")
    logging.info(f"Monitoring Operator Address: {OPERATOR_ADDRESS}")

    while True:
        votes_inspector(APIURL=APIURL, operator_address=OPERATOR_ADDRESS)
        logging.info(f"Sleeping for {CHECK_INTERVAL} seconds before the next run.")
        time.sleep(CHECK_INTERVAL)
