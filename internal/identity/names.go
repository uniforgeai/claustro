// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package identity

import "math/rand"

var adjectives = []string{
	"amber", "bold", "bright", "calm", "cold", "crisp", "dark", "deep", "dense", "dry",
	"dusk", "epic", "faint", "firm", "fleet", "free", "fresh", "frosted", "gilded", "glad",
	"grand", "grey", "grim", "hazy", "icy", "jade", "keen", "lush", "misty", "noble",
	"pale", "prime", "proud", "quick", "rare", "rich", "royal", "sharp", "silent", "slim",
	"slow", "smoky", "soft", "stark", "still", "stoic", "sunny", "swift", "teal", "warm",
	"wild", "wise",
}

var nouns = []string{
	"bear", "brook", "canyon", "cedar", "cliff", "cloud", "coast", "coral", "crane", "creek",
	"dune", "eagle", "elm", "falcon", "fern", "fjord", "fox", "gale", "glacier", "hawk",
	"heron", "iris", "isle", "kelp", "lake", "lark", "lynx", "maple", "marsh", "mesa",
	"mist", "moon", "moss", "otter", "owl", "panda", "peak", "pine", "raven", "reed",
	"ridge", "river", "robin", "sage", "seal", "shore", "sparrow", "storm", "swan", "tide",
	"vale", "wave", "wren",
}

// RandomName returns a random adjective_noun name suitable for a sandbox.
func RandomName() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	return adj + "_" + noun
}
