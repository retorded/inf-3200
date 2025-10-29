import sys
import requests
import time
import random

def join_ring(new_node, existing_ring_node):
    response = requests.post(f"http://{new_node}/join?nprime={existing_ring_node}")
    print(f"{new_node} JOIN response: {response.status_code}.")

def get_info(node):
    response = requests.get(f"http://{node}/node-info")
    info = {}
    if response.status_code == 200:
        info = response.json()
    return info

def leave_ring(node):
    response = requests.post(f"http://{node}/leave")
    print(f"{node} LEAVE response: {response.status_code}.")
    return response.status_code == 200

def crash_node(node):
    response = requests.post(f"http://{node}/sim-crash")
    print(f"{node} CRASH response: {response.status_code}.")
    return response.status_code == 200

def recover_node(node):
    response = requests.post(f"http://{node}/sim-recover")
    print(f"{node} RECOVER response: {response.status_code}.")
    return response.status_code == 200

def traverse_ring(start_node):
    """Traverse the ring and return list of nodes in order"""
    print(f"\n=== Traversing ring starting from {start_node} ===")
    
    visited_nodes = []
    current_node = start_node
    start_time = time.time()
    max_traversal_time = 30  # 30 seconds max for traversal
    
    while current_node and time.time() - start_time < max_traversal_time:
        info = get_info(current_node)
        
        if not info or "node_hash" not in info or "successor" not in info:
            print(f"ERROR: Could not get info from {current_node}")
            break
            
        node_hash = info["node_hash"]
        successor = info["successor"]
        
        visited_nodes.append({
            "address": current_node,
            "hash": node_hash,
            "successor": successor
        })
        
        # Check if we've completed the ring
        if successor in [node["address"] for node in visited_nodes]:
            #print(f"Ring completed! Back to {successor}")
            break
            
        current_node = successor

    return visited_nodes

def wait_for_ring_stabilization(start_node, expected_count, timeout=30):
    """Wait for ring to stabilize with expected node count"""
    start_time = time.time()
    while time.time() - start_time < timeout:
        ring = traverse_ring(start_node)
        if len(ring) == expected_count:
            return ring
        print(f"Waiting for ring to stabilize: {len(ring)}/{expected_count} nodes")
        time.sleep(2)
    return None

if len(sys.argv) < 2:
    print("Usage: python3 network_experiment.py <host:port> [<host:port> ...]")
    sys.exit(1)


host_ports = sys.argv[1:]
graceful_test = False # Pauses for one second between each request for nodes to join the network. Set to false for proper benchmarking
max_test_time = 120 # time in seconds before experiment times out

print("Received host:port pairs:", host_ports)

# Phase 1: Join all nodes to the ring
print("\n=== PHASE 1: Joining nodes to ring ===")
for node in host_ports[1:]:
    join_ring(node, host_ports[0])
    if graceful_test:
        time.sleep(1)

# Phase 2: Initial ring traversal
print("\n=== PHASE 2: Initial ring traversal ===")
start_time = time.time()
traversed = False
while not traversed:
    initial_ring = traverse_ring(host_ports[0])
    if len(initial_ring) == len(host_ports):
        print("SUCCESS:All nodes successfully joined the ring!")
        for node in initial_ring:
            print(f"{node['hash']} ({node['address']}) --> {node['successor']}")
        traversed = True
    else:
        print(f"ERROR: Ring incomplete: {len(initial_ring)}/{len(host_ports)} nodes")
        print("Elapsed time:", time.time() - start_time, "seconds")
        time.sleep(2)

# Phase 3: Test node leaving
print("\n=== PHASE 3: Testing node leave ===")
if len(host_ports) < 2:
    print("ERROR: Stopping test - need at least 2 nodes")
    sys.exit(1)

# Pick a random node to leave (not the first one)
node_to_leave = random.choice(host_ports[1:])
print(f"Asking {node_to_leave} to leave the ring...")

if leave_ring(node_to_leave):
    print(f"{node_to_leave} successfully left the ring")
else:
    print(f"Failed to make {node_to_leave} leave the ring")


# Phase 4: Verify ring after leave
print("\n=== PHASE 4: Verifying ring after leave ===")
final_ring = traverse_ring(host_ports[0])

expected_nodes = len(host_ports) - 1
if len(final_ring) == expected_nodes:
    print(f"Ring is healthy after leave! Contains {len(final_ring)} nodes (expected {expected_nodes})")
    for node in final_ring:
        print(f"{node['hash']} ({node['address']}) --> {node['successor']}")
else:
    print(f"Ring unhealthy after leave: {len(final_ring)} nodes (expected {expected_nodes})")
    
# Check if the leaving node is no longer in the ring
leaving_node_in_ring = any(node["address"] == node_to_leave for node in final_ring)
if not leaving_node_in_ring:
    print(f"{node_to_leave} successfully removed from ring")
else:
    print(f"ERROR: {node_to_leave} still present in ring after leave")
    sys.exit(1)

# Phase 5: Test node crash
print("\n=== PHASE 5: Testing node crash ===")
# Pick a different node to crash (not the one that left)
available_nodes = [node for node in host_ports if node != node_to_leave]
if len(available_nodes) > 1:
    node_to_crash = random.choice(available_nodes[1:])  # Don't crash the first available
    print(f"Asking {node_to_crash} to crash...")
    
    if crash_node(node_to_crash):
        print(f"{node_to_crash} successfully crashed")

        # Traverse ring and see if it is connected and without the crashed node
        crashed_ring = [node for node in available_nodes if node != node_to_crash]
        
        # Wait for ring to stabilize after crash
        print("Waiting for ring to stabilize after crash...")
        ring = wait_for_ring_stabilization(crashed_ring[0], len(crashed_ring))
        if ring:
            print("SUCCESS: Ring stabilized after crash!")
            for node in ring:
                print(f"{node['hash']} ({node['address']}) --> {node['successor']}")
            
        else:
            print("ERROR: Ring failed to stabilize after crash")
            print("Skipping recovery test due to crash stabilization failure")
            print("\n=== Experiment completed ===")
            sys.exit(1)
    else:
        print(f"Failed to make {node_to_crash} crash")
        print("Skipping recovery test due to crash failure")
        print("\n=== Experiment completed ===")
        sys.exit(1)
else:
    print("Skipping crash test - not enough nodes available")
    print("\n=== Experiment completed ===")
    sys.exit(0)

# Phase 6: Recover node and verify ring
print("\n=== PHASE 6: Recovering node and verifying ring ===")
print(f"Asking {node_to_crash} to recover...")
if recover_node(node_to_crash):
    print(f"{node_to_crash} successfully recovered")
else:
    print(f"Failed to make {node_to_crash} recover")
    print("\n=== Experiment completed ===")
    sys.exit(1)

# Test to see how long it takes for the ring to stabilize after a node recovers
print("Waiting for ring to stabilize after recovery...")
ring = wait_for_ring_stabilization(node_to_crash, len(available_nodes))  # Start from recovered node
if ring:
    print("SUCCESS: All nodes successfully stabilized after recovery!")
    for node in ring:
        print(f"Node:{node['hash']}--> {node['address']}")
else:
    print("ERROR: Ring failed to stabilize after recovery")

print("\n=== Experiment completed ===")