# go-glossopetrae

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-glossopetrae.svg)](https://pkg.go.dev/github.com/elder-plinius/go-glossopetrae)

Constructed Language Generator -- Go library for the GLOSSOPETRAE service.

## Overview

Go implementation of the GLOSSOPETRAE linguistic engine for AI. Generates constructed languages (conlangs) with phoneme selection, syllable structure, morphology, lexicon building, translation engine, and steganographic capabilities.

## Installation

```bash
go get github.com/elder-plinius/go-glossopetrae
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    glossopetrae "github.com/elder-plinius/go-glossopetrae/pkg/client"
)

func main() {
    client, err := glossopetrae.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `PhonemeInventory`
- `Lexicon`
- `WordEntry`
- `ConlangConfig`
- `GeneratedLanguage`
- `GrammarRules`
- `TranslationResult`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GenerateLanguage` | `cfg ConlangConfig` | Generate a complete conlang |
| `GeneratePhonemes` | `cfg ConlangConfig` | Generate phoneme inventory |
| `GenerateLexicon` | `phonemes PhonemeInventory, count int` | Generate lexicon from phonemes |
| `Translate` | `lang GeneratedLanguage, text string` | Translate text to conlang |
| `BackTranslate` | `lang GeneratedLanguage, text string` | Translate from conlang back |
| `EmbedSteganography` | `lang GeneratedLanguage, message string` | Embed hidden message in conlang text |
| `ExtractSteganography` | `lang GeneratedLanguage, text string` | Extract hidden message from conlang |
| `GetAvailablePhonemes` | `ctx context.Context` | Get available phoneme pools |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GLOSSOPETRAE_ADDRESS` | `localhost` | Service address |
| `GLOSSOPETRAE_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
