---
title: claustro config
weight: 6
---

# claustro config

View and modify claustro configuration.

## Interactive Subcommands

```bash
claustro config languages    # Configure image languages
claustro config tools        # Configure tool groups
claustro config mcp          # Configure MCP servers
claustro config defaults     # Configure resource defaults
claustro config firewall     # Configure firewall settings
claustro config git          # Configure git integration
```

## Get/Set

```bash
claustro config get <path>
claustro config set <path> <value>
```

## Examples

```bash
claustro config get image.languages.go
claustro config set defaults.resources.cpus 8
claustro config set image.languages.rust true
```
