package main

import (
	"fmt"
	symspell "github.com/zchrykng/go-symspell"
	"github.com/zchrykng/go-symspell/verbosity"
)

func main() {
	s, err := symspell.NewSymSpellDefault()
	if err != nil {
		panic(err)
	}

	if s.LoadDictionaryFile("frequency_en.txt") {
		if s.LoadBigramDictionaryFile("frequency_bigram_en.txt") {

			fmt.Println(s.MaxWordLength)
			fmt.Println(len(s.Deletes))
			fmt.Println(len(s.Words))

			suggs, err := s.LookupDefault("test", verbosity.All)
			if err != nil {
				panic(err)
			}

			for _, sugg := range suggs {
				fmt.Println("--------------")
				fmt.Printf("     term: %s\n", sugg.Term)
				fmt.Printf(" distance: %d\n", sugg.Distance)
				fmt.Printf("frequency: %d\n", sugg.Frequency)
			}
		} else {
			fmt.Println("Failed to load Bigram dictionary")
		}
	} else {
		fmt.Println("Failed to load dictionary")
	}

}
