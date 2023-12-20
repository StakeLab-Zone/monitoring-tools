import requests
import json
import sys

# Slack webhook URL
SLACK_WEBHOOK_URL = "ADD_YOUR_SLACK_WEBHOOK_HERE"

def send_slack_message(message):
    """Send a message to a Slack channel."""
    headers = {'Content-Type': 'application/json'}
    payload = {'text': message}
    response = requests.post(SLACK_WEBHOOK_URL, headers=headers, data=json.dumps(payload))
    return response

def print_error(r):
    print("Error on req ", r.status_code)
    sys.exit(-1)

def get_req(url):
    r = requests.get(url=url)
    if r.status_code == 200:
        data = r.json()
        if len(data['aggregate_votes']) > 0:
            return data
        return get_req(url)
    print_error(r)

def get_moniker(validators, valaddr):
    for validator in validators:
        if validator['operator_address'] == valaddr:
            return validator['description']['moniker']
    return None

def get_validators(APIURL):
    r = requests.get(url=f"https://{APIURL}/cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED")
    if r.status_code == 200:
        data = r.json()
        return data['validators']
    print_error(r)

def get_accepted_denoms(APIURL):
    r = requests.get(url=f"https://{APIURL}/umee/oracle/v1/params")
    if r.status_code == 200:
        data = r.json()
        return data['params']['accept_list']
    print_error(r)

def votes_inspector(APIURL):
    votes = f"https://{APIURL}/umee/oracle/v1/validators/aggregate_votes"

    denoms_resp = get_accepted_denoms(APIURL)
    accepted_denoms = [denom['symbol_denom'].upper() for denom in denoms_resp]
    accepted_denoms = list(set(accepted_denoms))

    validators = get_validators(APIURL=APIURL)

    # Get the total denoms exists in app
    votes_resp = get_req(votes)
    aggregate_votes = votes_resp['aggregate_votes']

    # Check for Your val specifically
    valoper_address = 'umeevaloper_to_complete'
    your_moniker = get_moniker(validators, valoper_address)
    if your_moniker != 'YOUR_MONIKER':
        print(f"StakeLab address not found or moniker mismatch. Found moniker: {your_moniker}")
        return

    for vote in aggregate_votes:
        if vote['voter'] == valoper_address:
            exchange_rate_tuples = [tuple['denom'] for tuple in vote['exchange_rate_tuples']]
            if len(exchange_rate_tuples) != len(accepted_denoms):
                message = f"❌ {your_moniker} is missing the votes for some denoms."
                #send_slack_message(message)
                print(message)
                for denom in accepted_denoms:
                    if denom not in exchange_rate_tuples:
                        print(f"{your_moniker} is missing votes for denom: {denom}")
            else:
                print(f"✅ {your_moniker} has submitted the correct votes: {len(exchange_rate_tuples)}")
            break
    else:
        message = f"❌ {your_moniker} did not submit any votes."
        send_slack_message(message)
        print(message)

if __name__ == "__main__":
    # You can replace the API endpoint here to use yours.
    APIURL = "api-umee.cosmos-spaces.cloud"
    print(f"Used API: {APIURL}")
    votes_inspector(APIURL=APIURL)
