import sys
import time
import csv
import random
import requests
from datetime import datetime

# ======================
# CONFIGURATION
# ======================
STABILIZATION_TIMEOUT = 15  # seconds to wait for ring stabilization
STABILIZATION_CHECK_INTERVAL = 0.2
REPEATS_PER_EXPERIMENT = 3
CSV_FILENAME = f"build/network_dynamic.csv"

# ======================
# BASIC NETWORK OPS
# ======================
def _post(url, timeout=2):
    try:
        resp = requests.post(url, timeout=timeout)
        return resp.status_code == 200, resp.status_code
    except requests.RequestException as e:
        print(f"[{time.strftime('%H:%M:%S')}] POST {url} failed: {e}", flush=True)
        return False, f"ERR:{e}"

def _get_json(url, timeout=2):
    try:
        resp = requests.get(url, timeout=timeout)
        if resp.status_code == 200:
            return True, resp.json()
        return False, f"HTTP {resp.status_code}"
    except requests.RequestException as e:
        print(f"[{time.strftime('%H:%M:%S')}] GET {url} failed: {e}", flush=True)
        return False, f"ERR:{e}"

def join_ring(new_node, existing_node):
    ok, status = _post(f"http://{new_node}/join?nprime={existing_node}")
    if not ok:
        print(f"JOIN failed for {new_node} --> {status}", flush=True)
        sys.exit(1)
    return ok

def leave_ring(node):
    ok, _ = _post(f"http://{node}/leave")
    # Ignore failure so we can call leave on a failed/already left node
    return ok

def crash_node(node):
    ok, status = _post(f"http://{node}/sim-crash")
    if not ok:
        print(f"CRASH failed for {node} --> {status}", flush=True)
        sys.exit(1)
    return ok

def recover_node(node):
    ok, status = _post(f"http://{node}/sim-recover")
    if not ok:
        print(f"RECOVER failed for {node} --> {status}", flush=True)
        sys.exit(1)
    return ok

def get_info(node):
    ok, data = _get_json(f"http://{node}/node-info")
    return data if ok else None    

def reset_network(nodes):
    # Recover failed nodes so they can process the leave request
    for node in nodes:
        # Recover the node so it can start processing requests again
        recover_node(node)
        leave_ring(node)
        recover_node(node)

def traverse_ring(start_node):
    """Traverse the ring and return the list of nodes in order."""
    visited, current, start_time = [], start_node, time.time()
    while current and time.time() - start_time < STABILIZATION_TIMEOUT:
        info = get_info(current)
        if not info or "successor" not in info:
            break
        currentId = info["node_hash"]
        successor = info["successor"]
        if current in [n["address"] for n in visited]:
            break
        visited.append({"id": currentId, "address": current, "successor": successor})

        current = successor
    return visited

def wait_for_ring_stabilization(start_node, expected_count, timeout=STABILIZATION_TIMEOUT):
    """Wait until the ring stabilizes with the expected node count."""
    start_time = time.time()
    while time.time() - start_time < timeout:
        ring = traverse_ring(start_node)
        if len(ring) == expected_count:
            return True, ring
        time.sleep(STABILIZATION_CHECK_INTERVAL)

    print(f"Ring failed to stabilize: {len(ring)} != {expected_count}")
    return False, ring

def join_nodes(nodes):
    """Create a network of nodes by joining them in a ring."""
    base_node = nodes[0]
    for node in nodes[1:]:
        join_ring(node, base_node)
        time.sleep(0.2) 

# ======================
# EXPERIMENT HELPERS
# ======================
def log_result(writer, experiment, n_start, n_end, mode, duration, trial):
    writer.writerow({
        "timestamp": datetime.now().isoformat(),
        "experiment": experiment,
        "n_start": n_start,
        "n_end": n_end,
        "mode": mode,
        "duration_sec": round(duration, 3),
        "trial": trial
    })

# ======================
# EXPERIMENTS
# ======================
def experiment_grow(writer, all_nodes, mode="sequential"):
    """Measure time to grow network from 1 --> N nodes."""

    

    for n in [2, 4, 8, 16, 32]:

        if len(all_nodes) < n:
            print("Not enough nodes to run grow experiment.")
            print(f"All nodes: len={len(all_nodes)}")
            sys.exit(1)

        for trial in range(1, REPEATS_PER_EXPERIMENT + 1):
            print(f"\n==== Grow Trial {trial} ====")
            
            print(f"[Grow] From 1 to {n} nodes ({mode} mode)")

            participating_nodes = random.sample(all_nodes, n)

            # Start timing
            start_time = time.time()

            # Join nodes
            join_nodes(participating_nodes)

            # Wait for stabilization
            stabilized, ring = wait_for_ring_stabilization(participating_nodes[0], n)
            duration = time.time() - start_time

            log_result(writer, "grow", 1, n, mode, duration, trial)
            print(f"[Grow] {n} nodes stabilized in {duration:.2f}s (ok={stabilized})\n")

            if not stabilized:
                print(f"[Grow] {n} nodes failed to stabilize, stopping experiment")
                print(f"Partial ring: {ring}")
                reset_network(participating_nodes)
                sys.exit(1)

            sys.exit(0) # DEBUG
            
            reset_network(participating_nodes)

def experiment_shrink(writer, all_nodes, mode="sequential"):
    """Measure time to shrink network by half (32-->16, 16-->8, etc.)"""

    for n in [32, 16, 8, 4, 2]:
        n_end = n // 2
        participating_nodes = all_nodes[:n]

        for trial in range(1, REPEATS_PER_EXPERIMENT + 1):
            print(f"\n\n==== Shrink Trial {trial} ====")
            print(f"[Shrink] From {n} to {n_end} nodes ({mode} mode)")

            join_nodes(participating_nodes)

            # Wait for stabilization
            stabilized, ring = wait_for_ring_stabilization(participating_nodes[0], n)
            if not stabilized:
                print(f"[Shrink] {n} nodes failed to join, stopping experiment")
                for node in ring:
                    print(f"-- {node['id']} ({node['address']}) --> {node['successor']}")
                sys.exit(1)

            leaving_nodes = random.sample(participating_nodes, n - n_end)

            # Pick a base node to start traversal that is not a leaving node
            remaining_nodes = [node for node in participating_nodes if node not in leaving_nodes]
            start_node = random.choice(remaining_nodes)

            # Start timing
            start_time = time.time()

            if mode == "sequential":
                for node in leaving_nodes:
                    leave_ring(node)
            elif mode == "burst":
                # TODO: Implement burst mode
                pass
            else:
                raise ValueError(f"Unknown mode: {mode}")

            # Wait for stabilization
            stabilized, ring = wait_for_ring_stabilization(start_node, n_end)
            duration = time.time() - start_time

            log_result(writer, "shrink", n, n_end, mode, duration, trial)
            print(f"[Shrink] {n}->{n_end} stabilized in {duration:.2f}s (ok={stabilized})\n")

            if not stabilized:
                print(f"[Shrink] {n}->{n_end} failed to stabilize, stopping experiment")
                expected_ring = [start_node] + remaining_nodes
                print(f"Expected ring: {expected_ring}")
                print(f"Partial ring:")
                for node in ring:
                    print(f"-- {node['id']} ({node['address']}) --> {node['successor']}")
                sys.exit(1)

            # Reset all nodes involved back to single-node state
            reset_network(participating_nodes)

def experiment_crash_tolerance(writer, all_nodes, mode="sequential"):
    """Measure network tolerance to bursts of node crashes."""

    if len(all_nodes) < 32:
        print("Need at least 32 nodes for crash tolerance experiment.")
        sys.exit(1)

    # Always start with full stable network
    participating_nodes = all_nodes[:32]

    #for burst_size in range(1, 31):  # crash bursts
    for burst_size in [1, 2, 4, 8, 16, 32]:
        for trial in range(1, REPEATS_PER_EXPERIMENT + 1):
            print(f"\n==== Crash Tolerance Trial {trial}, burst={burst_size} ====")

            # Join all participating nodes to the ring
            join_nodes(participating_nodes)

            # Wait for stabilization
            stabilized, full_ring = wait_for_ring_stabilization(participating_nodes[1], len(participating_nodes))
            if not stabilized:
                print(f"[Crash Tolerance] {len(participating_nodes)} nodes failed to join, stopping experiment")
                print(f"Partial ring: {full_ring}")
                sys.exit(1)

            print(f"Time check at stable ring: {time.time()}")
            
            # Pick nodes to crash
            crashing_nodes = random.sample(participating_nodes, burst_size)
            living_nodes = [n for n in participating_nodes if n not in crashing_nodes]

            time.sleep(3) # Wait for finger tables to stabilize before crashing nodes

            start_time = time.time()

            # Send crash requests
            if mode == "sequential":
                for node in crashing_nodes:
                    crash_node(node)
            else:
                raise ValueError(f"Unknown mode: {mode}")

            # Wait for network stabilization around remaining nodes
            expected_remaining = len(living_nodes)
            stabilized, ring = wait_for_ring_stabilization(living_nodes[0], expected_remaining)
            duration = time.time() - start_time

            log_result(writer, "crash_tolerance", len(participating_nodes), expected_remaining, f"burst_{burst_size}", duration, trial)
            print(f"[Crash] Burst={burst_size} stabilized in {duration:.2f}s (ok={stabilized})")

            if not stabilized:
                print(f"[Crash Tolerance] {len(participating_nodes)}->{expected_remaining} failed to stabilize, stopping experiment")
                print(f"Full ring before crash:")
                for node in full_ring:
                    print(f"-- {node['id']} ({node['address']}) --> {node['successor']}")
                print(f"\nPartial ring after crash:")
                for node in ring:
                    print(f"-- {node['id']} ({node['address']}) --> {node['successor']}")

                # Get info on nodes that failed to join
                ring_node_addresses = [node['address'] for node in ring]
                failed_to_join = [node for node in living_nodes if node not in ring_node_addresses]
                print(f"Failed to join: {failed_to_join}")
                print(f"Crashed nodes: {crashing_nodes}")
                #reset_network(participating_nodes)
                sys.exit(1)

            # Recover all crashed nodes to reset network for next trial
            for node in crashing_nodes:
                recover_node(node)
            
            print(f"Successfully crashed {burst_size} nodes and recovered again. {crashing_nodes}")
            
            # Reset network to original state
            reset_network(participating_nodes)
            
            #sys.exit(0)
            


            


# ======================
# MAIN
# ======================
def main():
    if len(sys.argv) < 2:
        print("Usage: python3 network_experiments.py <host:port> [<host:port> ...]")
        sys.exit(1)

    all_nodes = sys.argv[1:]

    print("Running experiments on nodes:", all_nodes)

    with open(CSV_FILENAME, "w", newline="") as csvfile:
        fieldnames = ["timestamp", "experiment", "n_start", "n_end", "mode", "duration_sec", "trial"]
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        writer.writeheader()

        # Run experiments
        experiment_grow(writer, all_nodes, mode="sequential")
        #experiment_grow(writer, all_nodes, mode="burst")

        #experiment_shrink(writer, all_nodes, mode="sequential")
        #experiment_shrink(writer, all_nodes, mode="burst")

        #experiment_crash_tolerance(writer, all_nodes)

    print(f"\n Experiments complete. Results saved to {CSV_FILENAME}")

if __name__ == "__main__":
    main()
