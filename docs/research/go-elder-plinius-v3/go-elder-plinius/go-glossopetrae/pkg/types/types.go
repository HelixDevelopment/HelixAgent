// Package types defines Go types for the GLOSSOPETRAE library.
// Go implementation of the GLOSSOPETRAE linguistic engine for AI. Generates constructed languages (conlangs) with phoneme selection, syllable structure, morphology, lexicon building, translation engine, and steganographic capabilities.
package types

import (
	"fmt"
	"strings"
)

// PhonemeInventory represents phonemeinventory data.
type PhonemeInventory struct {
	Vowels []string
	SyllableStructure string
	Clusters []string
	Consonants []string
	Tones []string
}

// Lexicon represents lexicon data.
type Lexicon struct {
	Words []WordEntry
	RootCount int
	DerivationRules []string
}

// WordEntry represents wordentry data.
type WordEntry struct {
	IPA string
	Word string
	Gloss string
	POS string
	Root string
	Derivation string
}

// ConlangConfig represents conlangconfig data.
type ConlangConfig struct {
	MorphologyType string
	WordCount int
	Seed int64
	PhonemeCount int
	Difficulty string
	EnableSteganography bool
	Name string
}

// Validate checks that the ConlangConfig is valid.
func (o *ConlangConfig) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// GeneratedLanguage represents generatedlanguage data.
type GeneratedLanguage struct {
	SampleText string
	Grammar GrammarRules
	Lexicon Lexicon
	Dictionary map[string]string
	Phonemes PhonemeInventory
	Name string
}

// Validate checks that the GeneratedLanguage is valid.
func (o *GeneratedLanguage) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// GrammarRules represents grammarrules data.
type GrammarRules struct {
	Aspects []string
	Cases []string
	Tenses []string
	WordOrder string
	Evidentiality []string
}

// TranslationResult represents translationresult data.
type TranslationResult struct {
	IPA string
	WordBreakdown []WordEntry
	Original string
	Gloss string
	Translated string
}

