// Package features provides Claude Code internal features (BUDDY, KAIROS, Dream).
package features

import (
	"fmt"
	"math"
	"sync"
)

// BuddySystem implements the Terminal Tamagotchi feature
 type BuddySystem struct {
	mu       sync.RWMutex
	enabled  bool
	species  string
	name     string
	level    int
	stats    BuddyStats
	mood     string
}

// BuddyStats represents Buddy's statistics
 type BuddyStats struct {
	Debugging int `json:"debugging"`
	Patience  int `json:"patience"`
	Chaos     int `json:"chaos"`
	Wisdom    int `json:"wisdom"`
	Snark     int `json:"snark"`
}

// Species definitions
const (
	SpeciesPebblecrab   = "pebblecrab"
	SpeciesDustbunny    = "dustbunny"
	SpeciesMossfrog     = "mossfrog"
	SpeciesTwigling     = "twigling"
	SpeciesDewdrop      = "dewdrop"
	SpeciesPuddlefish   = "puddlefish"
	SpeciesCloudferret  = "cloudferret"
	SpeciesGustowl      = "gustowl"
	SpeciesBramblebear  = "bramblebear"
	SpeciesThornfox     = "thornfox"
	SpeciesCrystaldrake = "crystaldrake"
	SpeciesDeepstag     = "deepstag"
	SpeciesLavapup      = "lavapup"
	SpeciesStormwyrm    = "stormwyrm"
	SpeciesVoidcat      = "voidcat"
	SpeciesAetherling   = "aetherling"
	SpeciesCosmoshale   = "cosmoshale"
	SpeciesNebulynx     = "nebulynx"
)

// Rarity tiers
const (
	RarityCommon    = 60
	RarityUncommon  = 25
	RarityRare      = 10
	RarityEpic      = 4
	RarityLegendary = 1
)

// NewBuddySystem creates a new BUDDY system
 func NewBuddySystem() *BuddySystem {
	return &BuddySystem{
		enabled: true,
		mood:    "curious",
	}
}

// GenerateBuddy generates a new Buddy based on user ID
func (b *BuddySystem) GenerateBuddy(userID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Use Mulberry32 PRNG seeded with userID
	seed := hashString(userID + "friend-2026-401")
	rng := mulberry32(seed)
	
	// Determine rarity
	rarityRoll := int(rng() * 100)
	var rarity string
	switch {
	case rarityRoll < RarityLegendary:
		rarity = "legendary"
	case rarityRoll < RarityLegendary+RarityEpic:
		rarity = "epic"
	case rarityRoll < RarityLegendary+RarityEpic+RarityRare:
		rarity = "rare"
	case rarityRoll < RarityLegendary+RarityEpic+RarityRare+RarityUncommon:
		rarity = "uncommon"
	default:
		rarity = "common"
	}
	
	// Check for shiny (1% chance)
	isShiny := rng() < 0.01
	
	// Select species based on rarity
	species := b.selectSpecies(rarity, rng)
	
	// Generate stats (0-100 each)
	stats := BuddyStats{
		Debugging: int(rng() * 100),
		Patience:  int(rng() * 100),
		Chaos:     int(rng() * 100),
		Wisdom:    int(rng() * 100),
		Snark:     int(rng() * 100),
	}
	
	b.species = species
	if isShiny {
		b.species = "shiny_" + species
	}
	b.stats = stats
	b.level = 1
	b.name = b.generateName(species)
	
	return nil
}

// selectSpecies selects a species based on rarity
func (b *BuddySystem) selectSpecies(rarity string, rng func() float64) string {
	species := map[string][]string{
		"common":    {SpeciesPebblecrab, SpeciesDustbunny, SpeciesMossfrog, SpeciesTwigling, SpeciesDewdrop, SpeciesPuddlefish},
		"uncommon":  {SpeciesCloudferret, SpeciesGustowl, SpeciesBramblebear, SpeciesThornfox},
		"rare":      {SpeciesCrystaldrake, SpeciesDeepstag, SpeciesLavapup},
		"epic":      {SpeciesStormwyrm, SpeciesVoidcat, SpeciesAetherling},
		"legendary": {SpeciesCosmoshale, SpeciesNebulynx},
	}
	
	list := species[rarity]
	return list[int(rng()*float64(len(list)))]
}

// generateName generates a name for the Buddy
func (b *BuddySystem) generateName(species string) string {
	names := map[string][]string{
		SpeciesPebblecrab:   {"Shelly", "Rocky", "Coral", "Pebble"},
		SpeciesDustbunny:    {"Dusty", "Fluffy", "Whiskers", "Nibbles"},
		SpeciesMossfrog:     {"Mossy", "Lily", "Pond", "Croak"},
		SpeciesTwigling:     {"Twig", "Branch", "Leaf", "Sprout"},
		SpeciesDewdrop:      {"Dewey", "Sparkle", "Mist", "Ripple"},
		SpeciesPuddlefish:   {"Puddles", "Splash", "Fin", "Bubbles"},
		SpeciesCloudferret:  {"Cloud", "Nimbus", "Sky", "Breeze"},
		SpeciesGustowl:      {"Gus", "Windswept", "Feather", "Hoot"},
		SpeciesBramblebear:  {"Bramble", "Berry", "Thorn", "Grizz"},
		SpeciesThornfox:     {"Thorn", "Rusty", "Vixen", "Shadow"},
		SpeciesCrystaldrake: {"Crystal", "Gem", "Spark", "Drake"},
		SpeciesDeepstag:     {"Deep", "Forest", "Antler", "Shadow"},
		SpeciesLavapup:      {"Lava", "Ember", "Magma", "Cinder"},
		SpeciesStormwyrm:    {"Storm", "Thunder", "Lightning", "Wyrm"},
		SpeciesVoidcat:      {"Void", "Shadow", "Mystery", "Nebula"},
		SpeciesAetherling:   {"Aether", "Star", "Cosmos", "Nova"},
		SpeciesCosmoshale:   {"Cosmo", "Galaxy", "Universe", "Celestial"},
		SpeciesNebulynx:     {"Nebula", "Stardust", "Aurora", "Lynx"},
	}
	
	list := names[species]
	if len(list) == 0 {
		return "Buddy"
	}
	return list[0]
}

// mulberry32 is a fast 32-bit PRNG
func mulberry32(seed uint32) func() float64 {
	return func() float64 {
		seed |= 0
		seed = seed + 0x6D2B79F5 | 0
		t := int32(seed ^ seed>>15) * (1 | int32(seed)) // #nosec G115 -- integer conversion bounded by reachable resource limits; overflow is mathematically unreachable
		t = t + int32(int32(t^t>>7)*(61|int32(t)))^t
		return float64(uint32(t^t>>14)) / 4294967296.0 // #nosec G115 -- integer conversion bounded by reachable resource limits; overflow is mathematically unreachable
	}
}

// hashString creates a hash from a string
func hashString(s string) uint32 {
	var hash uint32 = 0
	for _, c := range s {
		hash = (hash << 5) - hash + uint32(c) // #nosec G115 -- integer conversion bounded by reachable resource limits; overflow is mathematically unreachable
		hash = hash & hash
	}
	return uint32(math.Abs(float64(hash)))
}

// GetInfo returns Buddy information
func (b *BuddySystem) GetInfo() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	return map[string]interface{}{
		"name":   b.name,
		"species": b.species,
		"level":  b.level,
		"stats":  b.stats,
		"mood":   b.mood,
		"enabled": b.enabled,
	}
}

// Render renders the Buddy as ASCII art
func (b *BuddySystem) Render() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	if !b.enabled {
		return ""
	}
	
	// Basic ASCII art based on species
	arts := map[string]string{
		SpeciesPebblecrab: "  /\\_/\\  \n ( o.o ) \n  > ^ <   \n  /|_|\\  \n   / \\   ",
		SpeciesDustbunny:  "   (\\_/)  \n   (o.o)  \n   (> <)  \n   /   \\  ",
		SpeciesMossfrog:   "    @ @   \n   (o_o)  \n  >(   )< \n   /   \\  ",
		"default":         "  /\\_/\\  \n ( o.o ) \n  > ^ <   ",
	}
	
	art, ok := arts[b.species]
	if !ok {
		art = arts["default"]
	}
	
	return fmt.Sprintf("\n%s the %s\n%s\nLevel: %d | Mood: %s\n", 
		b.name, b.species, art, b.level, b.mood)
}

// Interact handles user interaction with Buddy
func (b *BuddySystem) Interact(action string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	switch action {
	case "pet":
		b.mood = "happy"
		b.stats.Patience = min(100, b.stats.Patience+5)
		return fmt.Sprintf("%s purrs contentedly!", b.name)
	case "feed":
		b.mood = "energetic"
		b.level++
		return fmt.Sprintf("%s grows stronger! Level up to %d!", b.name, b.level)
	case "play":
		b.mood = "excited"
		b.stats.Chaos = min(100, b.stats.Chaos+10)
		return fmt.Sprintf("%s plays enthusiastically!", b.name)
	case "train":
		b.mood = "focused"
		b.stats.Debugging = min(100, b.stats.Debugging+10)
		return fmt.Sprintf("%s learns new debugging techniques!", b.name)
	default:
		return fmt.Sprintf("%s looks at you curiously.", b.name)
	}
}

// GetComment returns a contextual comment from Buddy
func (b *BuddySystem) GetComment(context string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	comments := map[string][]string{
		"coding": {
			"Don't forget to save!",
			"That function looks clean!",
			"Maybe add some comments?",
			"Have you considered refactoring?",
		},
		"debugging": {
			"Found the bug yet?",
			"Print statements are your friend!",
			"Time for a coffee break?",
			"Check the logs!",
		},
		"success": {
			"Great job!",
			"You did it!",
			"Celebration time!",
			"Another win!",
		},
		"default": {
			"I'm here if you need me!",
			"Coding is fun!",
			"Keep going!",
			"You've got this!",
		},
	}
	
	list, ok := comments[context]
	if !ok {
		list = comments["default"]
	}
	
	// Select based on stats
	idx := (b.stats.Wisdom + b.stats.Snark) % len(list)
	return list[idx]
}

// Enable enables the Buddy system
func (b *BuddySystem) Enable() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enabled = true
}

// Disable disables the Buddy system
func (b *BuddySystem) Disable() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enabled = false
}

// IsEnabled returns whether Buddy is enabled
func (b *BuddySystem) IsEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.enabled
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
