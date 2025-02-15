import argparse
import time
from typing import List, Dict, Optional
import requests
from prometheus_client import start_http_server, Gauge


# Prometheus metrics
BLOCKS_SIGNED = Gauge(
    'cometbft_validator_blocks_signed_total',
    'Number of blocks signed by validator in the last 100 blocks',
    ['validator_address']
)

MISSED_BLOCKS = Gauge(
    'cometbft_validator_blocks_missed_total',
    'Number of blocks missed by validator in the last 100 blocks',
    ['validator_address']
)

SIGNING_PERCENTAGE = Gauge(
    'cometbft_validator_signing_percentage',
    'Percentage of blocks signed by validator in the last 100 blocks',
    ['validator_address']
)


class ValidatorMonitor:
    def __init__(self, rpc_endpoints: List[str], addresses: List[str], check_interval: int):
        self.rpc_endpoints = rpc_endpoints
        self.current_rpc_index = 0
        self.addresses = addresses
        self.check_interval = check_interval
        # Initialize Prometheus metrics
        for addr in addresses:
            BLOCKS_SIGNED.labels(validator_address=addr).set(0)
            MISSED_BLOCKS.labels(validator_address=addr).set(0)
            SIGNING_PERCENTAGE.labels(validator_address=addr).set(0)


    def get_active_rpc(self) -> str:
        return self.rpc_endpoints[self.current_rpc_index]


    def switch_rpc(self) -> str:
        """Switch to next RPC endpoint in the list"""
        self.current_rpc_index = (self.current_rpc_index + 1) % len(self.rpc_endpoints)
        new_rpc = self.get_active_rpc()
        print(f"🔄 Switching to RPC endpoint: {new_rpc}")
        return new_rpc


    def make_request(self, url: str, params: Optional[Dict] = None) -> Optional[Dict]:
        """Make HTTP request with failover support"""
        for _ in range(len(self.rpc_endpoints)):
            try:
                response = requests.get(url, params=params)
                response.raise_for_status()
                return response.json()
            except requests.exceptions.RequestException as e:
                print(f"❌ Error with RPC {self.get_active_rpc()}: {e}")
                self.switch_rpc()
                # Update URL with new RPC endpoint
                url = url.replace(url.split('/')[2], self.get_active_rpc().split('/')[2])
        return None


    def get_validator_set(self, height: int) -> Optional[Dict]:
        url = f"{self.get_active_rpc()}/validators"
        params = {"height": height, "per_page": 100}
        return self.make_request(url, params)


    def get_block(self, height: Optional[int] = None) -> Optional[Dict]:
        url = f"{self.get_active_rpc()}/block"
        params = {"height": height} if height is not None else None
        return self.make_request(url, params)


    def find_validator_index(self, validator_address: str, validators: List[Dict]) -> Optional[int]:
        for idx, validator in enumerate(validators):
            if validator.get('address') == validator_address:
                return idx
        return None


    def update_metrics(self, addr: str, signed: int, total: int):
        """Update Prometheus metrics for a validator"""
        try:
            percentage = (signed / total) * 100
            BLOCKS_SIGNED.labels(validator_address=addr).set(signed)
            MISSED_BLOCKS.labels(validator_address=addr).set(total - signed)
            SIGNING_PERCENTAGE.labels(validator_address=addr).set(percentage)
        except Exception as e:
            print(f"❌ Error updating metrics for {addr}: {e}")


    def monitor_validators(self):
        print("🔄 Starting monitoring loop...")
        
        while True:
            try:
                latest_block = self.get_block()
                if not latest_block:
                    time.sleep(self.check_interval)
                    continue

                latest_height = int(latest_block['result']['block']['header']['height'])
                print(f"\n📊 Analyzing blocks up to height {latest_height}")
                
                total_blocks = 100
                blocks_checked = 0
                start_height = max(1, latest_height - total_blocks + 1)  # +1 to include latest block

                # Initialize counters for each validator
                signed_counts = {addr: 0 for addr in self.addresses}

                # Check last 100 blocks
                for height in range(start_height, latest_height + 1):
                    block = self.get_block(height)
                    if not block:
                        continue

                    blocks_checked += 1
                    if blocks_checked > total_blocks:
                        break  # Ensure we don't check more than 100 blocks

                    val_set = self.get_validator_set(height)
                    if not val_set or 'result' not in val_set or 'validators' not in val_set['result']:
                        continue

                    # Initialize signed validators set
                    signed_vals = set()

                    # Check signatures in the block
                    try:
                        commit = block['result']['block']['last_commit']
                        signatures = commit.get('signatures', [])
                        for sig in signatures:
                            if isinstance(sig, dict):
                                validator_addr = sig.get('validator_address')
                                if validator_addr in self.addresses:
                                    block_id_flag = sig.get('block_id_flag', 0)
                                    if block_id_flag > 1:  # Validator signed the block
                                        signed_vals.add(validator_addr)
                                        
                    except Exception as e:
                        print(f"❌ Error processing signatures: {e}")

                    # Update signed counts
                    for addr in self.addresses:
                        if addr in signed_vals:
                            signed_counts[addr] += 1

                # Update metrics and print summary
                print("\n📋 Signing Summary (last 100 blocks):")
                print("=" * 50)
                
                actual_total_blocks = min(blocks_checked, total_blocks)  # Use actual number of blocks checked
                
                for addr in self.addresses:
                    signed = min(signed_counts[addr], actual_total_blocks)  # Ensure we don't exceed total blocks
                    percentage = (signed / actual_total_blocks) * 100 if actual_total_blocks > 0 else 0

                    # Update Prometheus metrics
                    self.update_metrics(addr, signed, actual_total_blocks)

                    # Print status with color-coded emoji
                    status_emoji = "🟢" if percentage >= 95 else "🟡" if percentage >= 80 else "🔴"
                    print(f"{status_emoji} Validator {addr[:10]}...{addr[-8:]}")
                    print(f"   ├─ Signed: {signed}/{actual_total_blocks} blocks")
                    print(f"   └─ Uptime: {percentage:.2f}%")

            except Exception as e:
                print(f"❌ Error in monitoring loop: {e}")

            print(f"\n⏳ Waiting {self.check_interval} seconds...")
            time.sleep(self.check_interval)


def main():
    parser = argparse.ArgumentParser(description='Monitor CometBFT validator block signing')
    parser.add_argument('--rpc', required=True,
                      help='Comma-separated list of CometBFT RPC endpoints')
    parser.add_argument('--addresses', required=True,
                      help='Comma-separated list of validator addresses to monitor')
    parser.add_argument('--port', type=int, default=2112,
                      help='Prometheus metrics port')
    parser.add_argument('--host', default='0.0.0.0',
                      help='Host interface to listen on (default: 0.0.0.0)')
    parser.add_argument('--interval', type=int, default=60,
                      help='Check interval in seconds')

    args = parser.parse_args()
    addresses = [addr.strip() for addr in args.addresses.split(',')]
    rpc_endpoints = [rpc.strip() for rpc in args.rpc.split(',')]

    # Start Prometheus metrics server
    try:
        start_http_server(args.port, args.host)
        print(f"📈 Started metrics server on {args.host}:{args.port}")
    except Exception as e:
        print(f"❌ Failed to start metrics server: {e}")
        return

    print(f"👀 Monitoring validators: {[f'{addr[:10]}...{addr[-8:]}' for addr in addresses]}")
    print(f"🌐 Using RPC endpoints: {rpc_endpoints}")
    monitor = ValidatorMonitor(rpc_endpoints, addresses, args.interval)
    monitor.monitor_validators()


if __name__ == '__main__':
    main()
