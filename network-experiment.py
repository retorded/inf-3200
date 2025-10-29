#!/usr/bin/env python3
import sys
import time
import random
import requests

# === GLOBAL CONFIGURATION ===
MAX_WAIT = 30          # Seconds to wait for stabilization
STEP_DELAY = 1         # Delay between joins in graceful mode
TIMEOUT = 120          # Total experiment timeout
GRACEFUL_MODE = False  # Join slowly if True
PRINT_RING_ON_SUCCESS = True


# === UTILITY WRAPPERS ===
def log(msg):
    print(f"[{time.strftime('%H:%M:%S')}] {msg}", flush=True)

def http_post(url):
    try:
        return requests.post(url, timeout=2)
    except requests.RequestException as e:
        log(f"POST {url} failed: {e}")
        return None

def http_get(url):
    try:
        return requests.get(url, timeout=2)
    except requests.RequestException as e:
        log(f"GET {url} failed: {e}")
        return None


# === CHORD API HELPERS ===
def join_ring(new_node, existing_node):
    resp = http_post(f"http://{new_node}/join?nprime={existing_node}")
    log(f"{new_node} JOIN → {resp.status_code if resp else 'FAIL'}")

def leave_ring(node):
    resp = http_post(f"http://{node}/leave")
    log(f"{node} LEAVE → {resp.status_code if resp else 'FAIL'}")
    return resp and resp.status_code == 200

def crash_node(node):
    resp = http_post(f"http://{node}/sim-crash")
    log(f"{node} CRASH → {resp.status_code if resp else 'FAIL'}")
    return resp and resp.status_code == 200

def recover_node(node):
    resp = http_post(f"http://{node}/sim-recover")
    log(f"{node} RECOVER → {resp.status_code if resp else 'FAIL'}")
    return resp and resp.status_code == 200

def get_info(node):
    resp = http_get(f"http://{node}/node-info")
    if resp and resp.status_code == 200:
        return resp.json()
    return {}

# === RING INSPECTION ===
def traverse_ring(start_node):
    """Traverse the Chord ring starting from 'start_node'."""
    log(f"Traversing ring from {start_node}...")
    visited, current = [], start_node
    start_time = time.time()

    while time.time() - start_time < MAX_WAIT:
        info = get_info(current)
        if not info or "successor" not in info:
            log(f"ERROR: no info from {current}")
            break

        successor = info["successor"]
        node_hash = info["node_hash"]
        visited.append({"address": current, "hash": node_hash, "successor": successor})

        if successor in [n["address"] for n in visited]:
            break  # completed cycle
        current = successor
    return visited

def wait_for_ring_stabilization(start_node, expected_count, timeout=MAX_WAIT):
    """Poll until the ring has the expected node count."""
    start = time.time()
    while time.time() - start < timeout:
        ring = traverse_ring(start_node)
        if len(ring) == expected_count:
            log(f"Ring stabilized with {expected_count} nodes.")
            return ring
        log(f"Waiting for stabilization: {len(ring)}/{expected_count} nodes...")
        time.sleep(2)
    return None

def print_ring(ring):
    log(f"=== Ring ({len(ring)} nodes) ===")
    for n in ring:
        print(f"{n['hash']:>6} ({n['address']}) --> {n['successor']}")
    print()


# === TEST PHASES ===
def join_all_nodes(hosts):
    log("=== Phase 1: Joining nodes ===")
    for node in hosts[1:]:
        join_ring(node, hosts[0])
        if GRACEFUL_MODE:
            time.sleep(STEP_DELAY)

def verify_full_ring(hosts):
    log("=== Phase 2: Verifying ring ===")
    ring = wait_for_ring_stabilization(hosts[0], len(hosts))
    if not ring:
        log("ERROR: Ring did not stabilize after join.")
        sys.exit(1)
    if PRINT_RING_ON_SUCCESS:
        print_ring(ring)
    return ring

def test_graceful_leave(hosts):
    log("=== Phase 3: Graceful Leave ===")
    leaving_node = random.choice(hosts[1:])
    log(f"Node leaving: {leaving_node}")
    leave_ring(leaving_node)
    ring = wait_for_ring_stabilization(hosts[0], len(hosts) - 1)
    if not ring:
        log("ERROR: Ring unstable after leave.")
        sys.exit(1)
    if any(n["address"] == leaving_node for n in ring):
        log(f"ERROR: {leaving_node} still in ring after leave.")
        sys.exit(1)
    log(f"{leaving_node} successfully removed.")
    print_ring(ring)
    return [h for h in hosts if h != leaving_node]

def test_crash_recovery(hosts):
    log("=== Phase 4: Crash & Recovery ===")
    node_to_crash = random.choice(hosts[1:])
    log(f"Crashing node {node_to_crash}...")
    crash_node(node_to_crash)
    post_crash_ring = wait_for_ring_stabilization(hosts[0], len(hosts) - 1)
    if not post_crash_ring:
        log("ERROR: Ring did not stabilize after crash.")
        sys.exit(1)
    print_ring(post_crash_ring)

    log(f"Recovering node {node_to_crash}...")
    recover_node(node_to_crash)
    recovered_ring = wait_for_ring_stabilization(node_to_crash, len(hosts))
    if not recovered_ring:
        log("ERROR: Ring failed to recover after node rejoin.")
        sys.exit(1)
    print_ring(recovered_ring)

def run_experiment(hosts):
    start_time = time.time()
    join_all_nodes(hosts)
    verify_full_ring(hosts)
    alive_hosts = test_graceful_leave(hosts)
    test_crash_recovery(alive_hosts)
    log(f"Experiment completed in {round(time.time() - start_time, 1)}s")


# === MAIN ===
if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 chord_test.py <host:port> [<host:port> ...]")
        sys.exit(1)
    run_experiment(sys.argv[1:])
