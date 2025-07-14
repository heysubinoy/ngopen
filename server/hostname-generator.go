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
	adjectives := []string{"red", "blue", "happy", "swift", "clever", "brave", "kind", "wise", "calm", "bold"}
	nouns := []string{"fox", "bear", "eagle", "wolf", "tiger", "lion", "hawk", "deer", "snake", "panda"}
	return fmt.Sprintf("%s-%s-%d%s", adjectives[rand.Intn(len(adjectives))], nouns[rand.Intn(len(nouns))], rand.Intn(1000), hostnameSuffix)
}
