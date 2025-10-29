import sys
import time
import csv
import random
import requests
from datetime import datetime

# ======================
# CONFIGURATION
# ======================
STABILIZATION_TIMEOUT = 30  # seconds to wait for ring stabilization
STABILIZATION_CHECK_INTERVAL = 0.2
REPEATS_PER_EXPERIMENT = 3
CSV_FILENAME = f"build/network_dynamic_{datetime.now().strftime('%Y%m%d_%H%M%S')}.csv"

# ======================
# BASIC NETWORK OPS
# ======================
def join_ring(new_node, existing_node):
    return requests.post(f"http://{new_node}/join?nprime={existing_node}").status_code

def leave_ring(node):
    return requests.post(f"http://{node}/leave").status_code

def crash_node(node):
    return requests.post(f"http://{node}/sim-crash").status_code

def get_info(node):
    try:
        response = requests.get(f"http://{node}/node-info", timeout=2)
        if response.status_code == 200:
            return response.json()
    except requests.RequestException:
        return None
    return None

def traverse_ring(start_node):
    """Traverse the ring and return the list of nodes in order."""
    visited, current, start_time = [], start_node, time.time()
    while current and time.time() - start_time < STABILIZATION_TIMEOUT:
        info = get_info(current)
        if not info or "successor" not in info:
            break
        successor = info["successor"]
        if current in [n["address"] for n in visited]:
            break
        visited.append({"address": current, "successor": successor})
        current = successor
    return visited

def wait_for_ring_stabilization(start_node, expected_count, timeout=STABILIZATION_TIMEOUT):
    """Wait until the ring stabilizes with the expected node count."""
    start_time = time.time()
    while time.time() - start_time < timeout:
        ring = traverse_ring(start_node)
        if len(ring) == expected_count:
            return True
        time.sleep(STABILIZATION_CHECK_INTERVAL)
    return False

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
        for trial in range(1, REPEATS_PER_EXPERIMENT + 1):
            # Reset all nodes to single-node state (external orchestration assumed)
            print(f"\n[Grow] Starting trial {trial} for {n} nodes in {mode} mode")
            base_node = all_nodes[0]

            # Start timing
            start_time = time.time()

            # Join nodes
            joining_nodes = all_nodes[1:n]
            if mode == "sequential":
                for node in joining_nodes:
                    join_ring(node, base_node)
            else:  # burst
                for node in joining_nodes:
                    requests.post(f"http://{node}/join?nprime={base_node}")

            # Wait for stabilization
            stabilized = wait_for_ring_stabilization(base_node, n)
            duration = time.time() - start_time

            log_result(writer, "grow", 1, n, mode, duration, trial)
            print(f"[Grow] {n} nodes stabilized in {duration:.2f}s (ok={stabilized})")

def experiment_shrink(writer, all_nodes, mode="sequential"):
    """Measure time to shrink network by half (32-->16, 16-->8, etc.)."""
    for n in [32, 16, 8, 4]:
        n_end = n // 2
        for trial in range(1, REPEATS_PER_EXPERIMENT + 1):
            print(f"\n[Shrink] Trial {trial}: {n} --> {n_end} ({mode})")
            base_node = all_nodes[0]

            # Assume ring is already stabilized at size n
            leaving_nodes = random.sample(all_nodes[:n], n - n_end)

            start_time = time.time()

            if mode == "sequential":
                for node in leaving_nodes:
                    leave_ring(node)
            else:
                for node in leaving_nodes:
                    requests.post(f"http://{node}/leave")

            stabilized = wait_for_ring_stabilization(base_node, n_end)
            duration = time.time() - start_time

            log_result(writer, "shrink", n, n_end, mode, duration, trial)
            print(f"[Shrink] {n}->{n_end} stabilized in {duration:.2f}s (ok={stabilized})")

def experiment_crash_tolerance(writer, all_nodes):
    """Measure tolerance to bursts of node crashes."""
    base_node = all_nodes[0]
    for burst_size in range(1, 6):  # 1, 2, 3, 4, 5 crashes
        for trial in range(1, REPEATS_PER_EXPERIMENT + 1):
            print(f"\n[Crash] Trial {trial}, burst={burst_size}")
            crashing_nodes = random.sample(all_nodes[1:], burst_size)

            start_time = time.time()
            for node in crashing_nodes:
                crash_node(node)

            expected_remaining = len(all_nodes) - burst_size
            stabilized = wait_for_ring_stabilization(base_node, expected_remaining)
            duration = time.time() - start_time

            log_result(writer, "crash_tolerance", len(all_nodes), expected_remaining, f"burst_{burst_size}", duration, trial)
            print(f"[Crash] burst={burst_size} stabilized in {duration:.2f}s (ok={stabilized})")

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

        experiment_shrink(writer, all_nodes, mode="sequential")
        #experiment_shrink(writer, all_nodes, mode="burst")

        #experiment_crash_tolerance(writer, all_nodes)

    print(f"\n Experiments complete. Results saved to {CSV_FILENAME}")

if __name__ == "__main__":
    main()
