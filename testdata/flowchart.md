# Graphs and flowcharts

A mermaid fence is drawn where it stands, in the flow of the document.

## A pipeline

```mermaid
graph LR
  push --> build --> test
  test -->|pass| deploy
  test -->|fail| notify
```

## A decision tree

```mermaid
flowchart TD
  A[push to main] --> B[build]
  B --> C{tests pass?}
  C -->|yes| D[deploy]
  C -->|no| E[notify the team]
```

## Shapes, directions, weights

| Fence | Draws |
| --- | --- |
| `graph LR` | left to right |
| `flowchart TD` | top to bottom |
| `A[x]` `B(y)` `C{z}` | square, rounded, decision |
| `-->` `---` `-.->` | arrow, line, dashed |

- edges keep their labels, and two edges between the same ranks never overlap
- a diagram too wide for the pane is printed as source, never clipped
