#!/usr/bin/env python3
# chord-benchmark.py

import argparse
import time
import requests
import csv
import sys
import random
import string
from datetime import datetime

def generate_random_key(length=8):
    """Generate a random string key for better distribution"""
    return ''.join(random.choices(string.ascii_letters + string.digits, k=length))

def benchmark_dht(servers, operations=1000):
    """Run PUT and GET operations, return throughput"""

# Parse server list
    server_list = servers.split(',')

    print(f"Using {len(server_list)} servers, testing {operations} operations of PUT and GET")

    
    
    # Generate random test data for better distribution
    test_data = []
    for i in range(operations):
        key = generate_random_key()
        value = f"value_{i}"
        test_data.append((key, value))
    
     # Test PUT operations - randomly select server for each request
    put_start = time.time()
    for key, value in test_data:
        try:
            # Randomly select a server for this request
            server = random.choice(server_list)
            base_url = f"http://{server}"
            
            response = requests.put(f"{base_url}/storage/{key}", data=value, timeout=5)
            if response.status_code != 200:
                print(f"PUT failed for {key} on {server}: {response.status_code}", file=sys.stderr)
                sys.exit(1)
                
        except Exception as e:
            print(f"PUT error for {key} on {server}: {e}", file=sys.stderr)
            sys.exit(1)
    put_end = time.time()
    
    # Test GET operations - randomly select server for each request
    get_start = time.time()
    for key, _ in test_data:
        try:
            # Randomly select a server for this request
            server = random.choice(server_list)
            base_url = f"http://{server}"
            
            response = requests.get(f"{base_url}/storage/{key}", timeout=5)
            if response.status_code != 200:
                print(f"GET failed for {key} on {server}: {response.status_code}", file=sys.stderr)
                sys.exit(1)
        except Exception as e:
            print(f"GET error for {key} on {server}: {e}", file=sys.stderr)
            sys.exit(1)
    get_end = time.time()
    
    # Calculate throughput
    put_time = put_end - put_start
    get_time = get_end - get_start
    throughput_put = operations / put_time
    throughput_get = operations / get_time

    return throughput_put, throughput_get

def main():
    parser = argparse.ArgumentParser(description='DHT Throughput Benchmark')
    parser.add_argument('--network-size', type=int, required=True)
    parser.add_argument('--trial', type=int, required=True)
    parser.add_argument('--operations', type=int, default=1000)
    parser.add_argument('--csv-file', default='build/benchmark.csv')
    parser.add_argument('--servers', required=True, help='Comma-separated list of server addresses')
    args = parser.parse_args()
    
    # Run benchmark
    throughput_put, throughput_get = benchmark_dht(args.servers, args.operations)
    
    # Append to CSV
    with open(args.csv_file, 'a', newline='') as f:
        writer = csv.writer(f)
        writer.writerow([
            datetime.now().isoformat(),
            args.network_size,
            args.trial,
            args.operations,
            throughput_put,
            throughput_get,
        ])
    
    print(f"Network size: {args.network_size}, Trial: {args.trial}, Throughput (PUT, GET): ({throughput_put:.2f}, {throughput_get:.2f})  ops/sec")

if __name__ == "__main__":
    main()