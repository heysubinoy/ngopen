package server

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

var hostnameSuffix = ".n.sbn.lol"

func init() {
	envSuffix := os.Getenv("NGOPEN_HOSTNAME_SUFFIX")
	if envSuffix != "" {
		hostnameSuffix = envSuffix
	}
}

func GenerateHostname() string {
	rand.Seed(time.Now().UnixNano())

	adjectives := []string{
		"red", "blue", "green", "yellow", "purple", "orange", "gold", "silver", "white", "black",
		"brave", "calm", "charming", "clever", "cool", "crazy", "curious", "dark", "daring", "dazzling",
		"eager", "fancy", "fast", "fierce", "funny", "gentle", "glorious", "graceful", "happy", "harsh",
		"honest", "jolly", "kind", "lazy", "light", "lively", "lucky", "luminous", "magical", "mighty",
		"mild", "moody", "mysterious", "noble", "noisy", "odd", "painful", "peaceful", "playful", "polite",
		"proud", "quick", "quiet", "rare", "restless", "rough", "royal", "shiny", "shy", "silent",
		"smart", "smooth", "soft", "sparkling", "speedy", "spicy", "spiky", "stable", "stealthy", "stern",
		"strong", "sturdy", "sunny", "sweet", "swift", "tame", "tender", "tough", "tranquil", "unique",
		"vague", "vast", "vibrant", "wild", "witty", "warm", "young", "zesty", "zealous", "bold",
		"ancient", "bright", "elegant", "radiant", "epic", "fearless", "gritty", "hardy", "keen", "nimble",
	}

	nouns := []string{
		"lion", "tiger", "bear", "wolf", "fox", "deer", "hawk", "eagle", "owl", "cat",
		"dog", "rabbit", "panda", "koala", "whale", "shark", "dolphin", "horse", "zebra", "rhino",
		"giraffe", "monkey", "chimp", "otter", "boar", "crocodile", "elephant", "falcon", "goose", "goat",
		"iguana", "jaguar", "kangaroo", "lemming", "leopard", "llama", "lynx", "moose", "mule", "narwhal",
		"ocelot", "octopus", "ostrich", "panther", "parrot", "penguin", "porcupine", "puma", "quokka", "raccoon",
		"rat", "salamander", "seal", "sheep", "skunk", "sloth", "squid", "swan", "tapir", "termite",
		"toad", "turkey", "turtle", "vulture", "walrus", "weasel", "yak", "aardvark", "antelope", "badger",
		"bat", "beetle", "bison", "canary", "cheetah", "crab", "donkey", "duck", "ferret", "flamingo",
		"gecko", "gopher", "hamster", "hedgehog", "heron", "hyena", "maggot", "meerkat", "mole", "monarch",
		"mongoose", "moth", "newt", "orca", "platypus", "puppy", "quail", "reindeer", "rooster", "sparrow",
	}

	return fmt.Sprintf("%s-%s-%d%s",
		adjectives[rand.Intn(len(adjectives))],
		nouns[rand.Intn(len(nouns))],
		rand.Intn(10000),
		hostnameSuffix)
}
