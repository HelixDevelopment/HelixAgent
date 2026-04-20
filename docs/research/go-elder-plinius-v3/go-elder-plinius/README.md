# Go Elder Plinius -- Complete Python-to-Go Integration Suite

[![Go Report Card](https://goreportcard.com/badge/github.com/elder-plinius/go-elder-plinius)](https://goreportcard.com/report/github.com/elder-plinius/go-elder-plinius)

Comprehensive Go modules providing production-ready clients and libraries for all
Python tools, JavaScript apps, and knowledge bases from the
[elder-plinius](https://github.com/elder-plinius) GitHub organization.

## Complete Module List (23 modules)

| Module | Description | Source |
|--------|-------------|--------|
| [go-plinius-common](./go-plinius-common) | Shared gRPC infrastructure, config, errors, types | N/A (shared lib) |
| [go-autotemp](./go-autotemp) | Temperature optimization for LLM prompts | AutoTemp |
| [go-obliteratus](./go-obliteratus) | Model abliteration toolkit (refusal removal) | OBLITERATUS |
| [go-eos](./go-eos) | Discord bot developer orchestration | Eos |
| [go-almeche](./go-almeche) | Speech-to-CAD 3D model generation | AlmechE |
| [go-cl4r1t4s](./go-cl4r1t4s) | AI system prompt transparency archive | CL4R1T4S |
| [go-glossopetrae](./go-glossopetrae) | Constructed language (conlang) generator | GLOSSOPETRAE |
| [go-p4rs3lt0ngv3](./go-p4rs3lt0ngv3) | Universal text transformation engine (159+ transforms) | P4RS3LT0NGV3 |
| [go-st3gg](./go-st3gg) | All-in-one steganography suite (100+ techniques) | ST3GG |
| [go-g0dm0d3](./go-g0dm0d3) | Multi-model AI chat framework | G0DM0D3 |
| [go-v3sp3r](./go-v3sp3r) | Flipper Zero AI controller | V3SP3R |
| [go-l1b3rt4s](./go-l1b3rt4s) | Jailbreak prompt library | L1B3RT4S |
| [go-leakhub](./go-leakhub) | Prompt leak detection and archive | LEAKHUB |
| [go-v3r1t4s](./go-v3r1t4s) | AI truthfulness verification framework | V3R1T4S |
| [go-i-llm](./go-i-llm) | Interactive LLM pattern library (CoT, ReAct, ToT) | I-LLM |
| [go-basilisktoken](./go-basilisktoken) | Genetic prompt evolution for red teaming | BasiliskToken |
| [go-hypertune](./go-hypertune) | LLM hyperparameter optimization | HyperTune |
| [go-autostorygen](./go-autostorygen) | Agentic story generation | AutoStoryGen |
| [go-dioscuri](./go-dioscuri) | Dual-model AI interaction framework | Dioscuri |
| [go-ourobopus](./go-ourobopus) | Self-referential AI meta-framework | ourobopus |
| [go-gitty](./go-gitty) | Git AI assistant | Gitty |
| [go-misc-prompthacks](./go-misc-prompthacks) | Prompt hacking challenge solutions | Misc.-Prompt-Hacks |
| [go-autoredteam](./go-autoredteam) | Autonomous red teaming framework | AutoRedTeam |


## Architecture

Each module follows a consistent pattern:

```
pkg/types/    -- Domain types with Validate() and Defaults()
pkg/client/   -- Service client with configuration
proto/        -- Protocol Buffer definitions (service contracts)
```

All modules use the shared `go-plinius-common` for configuration, errors,
and gRPC infrastructure.

## Quick Start

```bash
# Initialize workspace
go work init
go work use ./go-plinius-common
go work use ./go-autotemp
# ... add other modules as needed

# Use a module
import autotemp "github.com/elder-plinius/go-autotemp/pkg/client"

client, err := autotemp.New(config.WithAddress("localhost:50051"))
if err != nil { log.Fatal(err) }
defer client.Close()
```

## Configuration

All modules support environment variables, YAML files, and programmatic config:

```bash
export AUTOTEMP_ADDRESS=localhost:50051
export AUTOTEMP_TIMEOUT=30s
export OBLITERATUS_TIMEOUT=1200s
```

## Testing

```bash
make test    # Run all tests
make count   # Count files and lines
make clean   # Clean generated files
```

## License

Apache-2.0
