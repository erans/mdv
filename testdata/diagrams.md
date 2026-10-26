# Sequence and state

## Sequence

```mermaid
sequenceDiagram
  participant A as Browser
  participant B as Server
  A->>B: GET /readme
  loop every request
    B->>B: check cache
    Note right of B: hit rate 94%
  end
  alt cached
    B-->>A: 200 cached
  else cold
    B-->>A: 200 fresh
  end
```

## State

```mermaid
stateDiagram-v2
  [*] --> Idle
  Idle --> Parsing : fence
  Parsing --> Drawing
  Drawing --> Idle : done
  Parsing --> Failed : bad syntax
```
