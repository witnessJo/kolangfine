package main

import (
	"fmt"
	kiwi "github.com/codingpot/kiwigo"
)

func main() {
	// Your code here
	kb := kiwi.NewBuilder("./base", 1, kiwi.KIWI_BUILD_INTEGRATE_ALLOMORPH)

	kb.AddWord("코딩냄비", kiwi.POS_NNP, 0)
	kb.AddWord("투자신탁", kiwi.POS_NNP, 10)

	k := kb.Build()
	defer k.Close()

	result, err := k.Analyze("투자신탁은 좋다.", 1, kiwi.KIWI_MATCH_ALL)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	for _, token := range result {
		fmt.Printf("%+v\n", token)
	}
	fmt.Printf("kolangfine was done.\n")
}
