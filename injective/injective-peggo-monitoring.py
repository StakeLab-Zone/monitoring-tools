import json
import requests

def send_slack_message(webhook_url, message):
    payload = {'text': message}
    response = requests.post(webhook_url, data=json.dumps(payload), headers={'Content-Type': 'application/json'})
    if response.status_code != 200:
        print(f"Failed to send message to Slack. Status code: {response.status_code}, response: {response.text}")

def main():

    orchestrator_inj_address = 'injYOUR_ORCHESTRATOR_ADDRESS'
    slack_webhook_url = "ADD_YOUR_SLACK_WEBHOOK_HERE"


    print("1. Check pending batches to be confirmed")
    batch_response = requests.get(f"https://lcd.injective.network/peggy/v1/batch/last?address={orchestrator_inj_address}")
    if batch_response.status_code != 200:
        print(f"Failed to fetch batch data. Status code: {batch_response.status_code}")
        print(f"Response: {batch_response.text}")
        return
    batch_data = batch_response.json()
    if 'batch' in batch_data and batch_data['batch'] is not None:
        if len(batch_data['batch']) == 0:
            print("(O) No pending batches")
        else:
            print("(X) result:")
            print(batch_data)
    else:
        print("No 'batch' data found in response.")

    print("\n2. Check pending valsets to be confirmed")
    valsets_response = requests.get(f"https://lcd.injective.network/peggy/v1/valset/last?address={orchestrator_inj_address}")
    if valsets_response.status_code != 200:
        print(f"Failed to fetch valsets data. Status code: {valsets_response.status_code}")
        print(f"Response: {valsets_response.text}")
        return
    valsets_data = valsets_response.json()
    if 'valsets' in valsets_data and valsets_data['valsets'] is not None:
        if len(valsets_data['valsets']) == 0:
            print("(O) No Pending Valsets")
        else:
            print("(X) result:")
            print(valsets_data)
    else:
        print("No 'valsets' data found in response.")

    print("\n3. Check latest event broadcasted by peggo is up to date")
    lon_response = requests.get("https://lcd.injective.network/peggy/v1/module_state")
    lce_response = requests.get(f"https://lcd.injective.network/peggy/v1/oracle/event/{orchestrator_inj_address}")

    if lon_response.status_code == 200 and lce_response.status_code == 200:
        lon_data = lon_response.json()
        lce_data = lce_response.json()

        lon = int(lon_data['state']['last_observed_nonce']) if 'state' in lon_data and 'last_observed_nonce' in lon_data['state'] else None
        lce = int(lce_data['last_claim_event']['ethereum_event_nonce']) if 'last_claim_event' in lce_data and 'ethereum_event_nonce' in lce_data['last_claim_event'] else None

        if lon is not None and lce is not None:
            if lon == lce:
                print("✅ your peggo is up to date")
            elif abs(lon - lce) > 20:
                message = f"❌ Peggo Injective is late by at least 20 events. last_observed_nonce:{lon}, last_claim_event.ethereum_event_nonce:{lce}"
                print(message)
                #send_slack_message(slack_webhook_url, message)
            else:
                message = f"❌ check Peggo injective last_observed_nonce:{lon}, last_claim_event.ethereum_event_nonce:{lce}"
                print(message)
                #send_slack_message(slack_webhook_url, message)
        else:
            print("Required data not found in the responses.")
    else:
        if lon_response.status_code != 200:
            print(f"Failed to fetch data for last observed nonce. Status code: {lon_response.status_code}")
        if lce_response.status_code != 200:
            print(f"Failed to fetch data for last claim event. Status code: {lce_response.status_code}")

if __name__ == "__main__":
    main()
