package main

import (
	"fmt"
	"math/rand"
	"time"
)


func GenerateHostname() string {
	rand.Seed(time.Now().UnixNano())
	adjectives := []string{"red", "blue", "happy", "swift", "clever", "brave", "kind", "wise", "calm", "bold"}
	nouns := []string{"fox", "bear", "eagle", "wolf", "tiger", "lion", "hawk", "deer", "snake", "panda"}
	return fmt.Sprintf("%s-%s-%d.n.sbn.lol", adjectives[rand.Intn(len(adjectives))], nouns[rand.Intn(len(nouns))], rand.Intn(1000))
}