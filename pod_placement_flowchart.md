# LEAD Framework Pod Placement Flow Chart

## Overview
The LEAD (Latency-aware Edge Application Deployment) Framework implements intelligent pod placement for microservices using three core algorithms and dynamic service discovery.

## Main Flow Chart

```mermaid
flowchart TD
    A[Start LEAD Framework] --> B[Initialize Configuration]
    B --> C[Create Kubernetes Client]
    C --> D[Initialize Service Discovery]
    D --> E[Start Service Discovery]
    
    E --> F{Services Discovered?}
    F -->|No| G[Use Fallback Graph]
    F -->|Yes| H[Create Service Graph]
    G --> H
    
    H --> I[Initialize Algorithms]
    I --> J[Scoring Algorithm]
    I --> K[Monitoring Algorithm]
    I --> L[Affinity Generator]
    
    J --> M[Perform Initial Analysis]
    K --> M
    L --> M
    
    M --> N[Algorithm 1: Score Paths]
    N --> O[Algorithm 3: Generate Affinity Rules]
    O --> P[Generate K8s Manifests]
    P --> Q[Start Monitoring Loop]
    
    Q --> R[Algorithm 2: Real-time Monitoring]
    R --> S{Latency Violation?}
    S -->|No| R
    S -->|Yes| T[Identify Bottlenecks]
    T --> U[Re-score Paths]
    U --> V[Scale Services]
    V --> W[Update Affinity Rules]
    W --> X[Regenerate Manifests]
    X --> R
    
    style A fill:#e1f5fe
    style N fill:#f3e5f5
    style O fill:#fff3e0
    style R fill:#e8f5e8
```

## Detailed Algorithm Flows

### Algorithm 1: Path Scoring and Prioritization

```mermaid
flowchart TD
    A1[Start Path Scoring] --> B1[Find All Paths from Gateway]
    B1 --> C1[For Each Path]
    C1 --> D1[Calculate Path Length Score]
    C1 --> E1[Calculate Pod Count Score]
    C1 --> F1[Calculate Edge Count Score]
    C1 --> G1[Calculate RPS Score]
    C1 --> H1[Calculate Network Topology Score]
    
    D1 --> I1[Combine Scores]
    E1 --> I1
    F1 --> I1
    G1 --> I1
    H1 --> I1
    
    I1 --> J1[Normalize Scores 0-100]
    J1 --> K1[Sort by Score Descending]
    K1 --> L1[Apply Weights 100, 99, 98...]
    L1 --> M1[Return Prioritized Paths]
    
    style A1 fill:#f3e5f5
    style M1 fill:#f3e5f5
```

### Algorithm 2: Real-time Monitoring and Auto-scaling

```mermaid
flowchart TD
    A2[Start Monitoring Loop] --> B2[Check Interval Timer]
    B2 --> C2[Collect Service Metrics]
    C2 --> D2{Latency > Threshold?}
    D2 -->|No| B2
    D2 -->|Yes| E2[Analyze Bottlenecks]
    
    E2 --> F2{CPU/Memory > Threshold?}
    F2 -->|No| G2[Check Next Service]
    F2 -->|Yes| H2[Add to Bottleneck List]
    G2 --> I2{More Services?}
    H2 --> I2
    I2 -->|Yes| F2
    I2 -->|No| J2{Bottlenecks Found?}
    
    J2 -->|No| B2
    J2 -->|Yes| K2[Re-run Algorithm 1]
    K2 --> L2[Scale Out Services]
    L2 --> M2[Update Service Graph]
    M2 --> N2[Regenerate Configurations]
    N2 --> B2
    
    style A2 fill:#e8f5e8
    style L2 fill:#ffebee
```

### Algorithm 3: Affinity Rule Generation

```mermaid
flowchart TD
    A3[Start Affinity Generation] --> B3[For Each Critical Path]
    B3 --> C3[Sort Services by ID]
    C3 --> D3[Iterate Service Pairs]
    D3 --> E3{Index is Even?}
    E3 -->|No| F3[Skip to Next Pair]
    E3 -->|Yes| G3[Generate Pod Affinity Rule]
    
    G3 --> H3[Create Co-location Rule]
    H3 --> I3[Generate Node Affinity]
    I3 --> J3{Same Availability Zone?}
    J3 -->|Yes| K3[Prefer Same AZ]
    J3 -->|No| L3[Use Default Placement]
    
    K3 --> M3[Add Bandwidth Preferences]
    L3 --> M3
    M3 --> N3[Validate Affinity Config]
    N3 --> O3[Add to Configuration Map]
    O3 --> F3
    F3 --> P3{More Pairs?}
    P3 -->|Yes| D3
    P3 -->|No| Q3[Merge Configurations]
    Q3 --> R3[Return Affinity Rules]
    
    style A3 fill:#fff3e0
    style R3 fill:#fff3e0
```

## Service Discovery Flow

```mermaid
flowchart TD
    SD1[Start Service Discovery] --> SD2[Get Current Pods from K8s]
    SD2 --> SD3[Group Pods by Service Name]
    SD3 --> SD4[Create Service Nodes]
    SD4 --> SD5[Determine Gateway Service]
    SD5 --> SD6[Add Hotel Reservation Dependencies]
    
    SD6 --> SD7[Start Pod Event Watcher]
    SD7 --> SD8[Start Periodic Refresh]
    SD8 --> SD9[Listen for Pod Events]
    
    SD9 --> SD10{Pod Event Received?}
    SD10 -->|Yes| SD11[Handle Pod Event]
    SD10 -->|No| SD12{Refresh Timer?}
    SD11 --> SD13[Refresh Service Graph]
    SD12 -->|Yes| SD13
    SD12 -->|No| SD9
    SD13 --> SD14[Notify Update Callback]
    SD14 --> SD9
    
    style SD1 fill:#e3f2fd
    style SD13 fill:#e3f2fd
```

## Pod Placement Decision Matrix

```mermaid
flowchart TD
    PM1[Pod Placement Request] --> PM2{Service Type?}
    PM2 -->|Frontend| PM3[High Priority Weight 100]
    PM2 -->|Core Service| PM4[Medium-High Priority 80-99]
    PM2 -->|Support Service| PM5[Medium Priority 60-79]
    PM2 -->|Database| PM6[Infrastructure Priority 40-59]
    
    PM3 --> PM7[Apply Affinity Rules]
    PM4 --> PM7
    PM5 --> PM7
    PM6 --> PM7
    
    PM7 --> PM8{Co-location Required?}
    PM8 -->|Yes| PM9[Prefer Same Node/AZ]
    PM8 -->|No| PM10[Distribute Across Nodes]
    
    PM9 --> PM11[Apply Network Topology]
    PM10 --> PM11
    
    PM11 --> PM12{Resource Constraints?}
    PM12 -->|Yes| PM13[Filter Suitable Nodes]
    PM12 -->|No| PM14[Use All Available Nodes]
    
    PM13 --> PM15[Final Placement Decision]
    PM14 --> PM15
    PM15 --> PM16[Create/Update Pod]
    
    style PM1 fill:#f1f8e9
    style PM16 fill:#f1f8e9
```

## Network Topology Considerations

```mermaid
flowchart TD
    NT1[Network Topology Analysis] --> NT2[Bandwidth Weight: 40%]
    NT1 --> NT3[Hops Weight: 30%]
    NT1 --> NT4[Geo Distance Weight: 20%]
    NT1 --> NT5[Availability Zone Weight: 10%]
    
    NT2 --> NT6[Calculate Service Score]
    NT3 --> NT6
    NT4 --> NT6
    NT5 --> NT6
    
    NT6 --> NT7{High Network Score?}
    NT7 -->|Yes| NT8[Prefer Co-location]
    NT7 -->|No| NT9[Allow Distribution]
    
    NT8 --> NT10[Generate Strong Affinity]
    NT9 --> NT11[Generate Weak Affinity]
    
    NT10 --> NT12[Apply to Pod Placement]
    NT11 --> NT12
    
    style NT1 fill:#fce4ec
    style NT12 fill:#fce4ec
```

## Key Components and Their Roles

### 1. **Service Discovery**
- Dynamically discovers running pods and services
- Creates service graph with dependencies
- Monitors pod lifecycle events
- Estimates network topology and performance metrics

### 2. **Scoring Algorithm (Algorithm 1)**
- Evaluates all possible paths from gateway to leaf services
- Considers path length, pod count, edge count, RPS, and network topology
- Assigns priority weights (100, 99, 98...) to critical paths
- Normalizes scores to 0-100 range

### 3. **Monitoring Algorithm (Algorithm 2)**
- Continuously monitors service metrics (CPU, memory, latency)
- Detects latency violations and resource bottlenecks
- Triggers automatic scaling when thresholds are exceeded
- Re-runs path scoring after scaling events

### 4. **Affinity Rule Generator (Algorithm 3)**
- Generates Kubernetes affinity rules for co-location
- Processes service pairs from critical paths
- Creates pod affinity and node affinity rules
- Considers network topology for optimal placement

### 5. **Kubernetes Integration**
- Generates deployment and service manifests
- Applies affinity rules to pod specifications
- Monitors cluster state and pod events
- Manages scaling operations

## Configuration Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| MonitoringInterval | 15s | How often to check service metrics |
| ResourceThreshold | 75% | CPU/Memory threshold for scaling |
| LatencyThreshold | 150ms | Network latency threshold |
| BandwidthWeight | 0.4 | Weight for bandwidth in scoring |
| HopsWeight | 0.3 | Weight for network hops |
| GeoDistanceWeight | 0.2 | Weight for geographic distance |
| AvailabilityZoneWeight | 0.1 | Weight for AZ preference |

## Hotel Reservation Service Dependencies

The framework is specifically designed for the Hotel Reservation benchmark with these service dependencies:

- **Frontend** → Search, User, Recommendation, Reservation
- **Search** → Profile, Geographic, Rate
- **User** → Rate

This dependency graph influences pod placement decisions to minimize inter-service communication latency.
