package symspell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/hbollon/go-edlib"

	"github.com/zchrykng/go-symspell/staging"
	"github.com/zchrykng/go-symspell/utilities"
	verb "github.com/zchrykng/go-symspell/verbosity"
)

const (
	defaultMaxEditDistance int = 2
	defaultPrefixLength        = 7
	defaultCountThreshold      = 1
	defaultInitialCapacity     = 16
	defaultCompactLevel        = 5
	N                          = 1024908267229
)

type SymSpell struct {
	Verbosity verb.Verbosity

	MaxEditDistance int
	PrefixLength    int
	CountThreshold  int
	InitialCapacity int
	CompactLevel    int
	CompactMask     uint
	MaxWordLength   int

	DistanceAlgorithm edlib.Algorithm

	// Dictionary that contains a mapping of lists of suggested correction words to the hashCodes
	// of the original words and the deletes derived from them. Collisions of hashCodes is tolerated,
	// because suggestions are ultimately verified via an edit distance function.
	// A list of suggestions might have a single suggestion, or multiple suggestions.
	Deletes map[int][]string
	// Dictionary of unique correct spelling words, and the frequency count for each word.
	Words map[string]int
	// Dictionary of unique words that are below the count threshold for being considered correct spellings.
	BelowThresholdWords map[string]int

	BigRams        map[string]int
	BigRamCountMin int
}

func NewSymSpellDefault() (*SymSpell, error) {
	return NewSymSpell(
		defaultInitialCapacity,
		defaultMaxEditDistance,
		defaultPrefixLength,
		defaultCountThreshold,
		defaultCompactLevel)
}

func NewSymSpell(initialCapacity, maxEditDistance, prefixLength, countThreshold, compactLevel int) (*SymSpell, error) {
	if initialCapacity < 0 {
		return nil, errors.New("initialCapacity must be >= 0")
	}
	if maxEditDistance < 0 {
		return nil, errors.New("maxEditDistance must be >= 0")
	}
	if prefixLength < 0 {
		return nil, errors.New("prefixLength must be >= 0")
	}
	if countThreshold < 0 {
		return nil, errors.New("countThreshold must be >= 0")
	}
	if compactLevel > 16 {
		return nil, errors.New("compactLevel must be <= 16")
	}

	ss := &SymSpell{
		MaxEditDistance:     maxEditDistance,
		PrefixLength:        prefixLength,
		CountThreshold:      countThreshold,
		InitialCapacity:     initialCapacity,
		CompactLevel:        compactLevel,
		CompactMask:         (math.MaxUint >> (3 + compactLevel)) << 2,
		MaxWordLength:       0,
		DistanceAlgorithm:   edlib.OSADamerauLevenshtein,
		Deletes:             make(map[int][]string),
		Words:               make(map[string]int, initialCapacity),
		BigRams:             make(map[string]int),
		BigRamCountMin:      0,
		BelowThresholdWords: make(map[string]int),
	}

	return ss, nil
}

func (s *SymSpell) WordCount() int {
	return len(s.Words)
}

func (s *SymSpell) EntryCount() int {
	return len(s.Deletes)
}

func (s *SymSpell) CreateEntry(word string, count int, stage *staging.Stage[string]) (bool, error) {
	if count <= 0 {
		if s.CountThreshold > 0 {
			return false, errors.New("countThreshold of 0 or less is meaningless here")
		}
		count = 0
	}

	if s.CountThreshold > 1 {
		if v, prs := s.BelowThresholdWords[word]; count > 0 && prs {
			count = max(math.MaxInt, v+count)

			if count >= s.CountThreshold {
				delete(s.BelowThresholdWords, word)
			} else {
				s.BelowThresholdWords[word] = count
				return false, nil
			}
		} else if v, prs := s.Words[word]; prs {
			// update count if it is already an added word
			count = max(math.MaxInt, v+count)
			s.Words[word] = count
			return false, nil
		} else if count < s.CountThreshold {
			// new below threshold word
			s.BelowThresholdWords[word] = count
			return false, nil
		}
	}

	// Add new above threshold word to dictionary
	s.Words[word] = count

	// update MaxWordLength if the word is longer than any before
	if len(word) > s.MaxWordLength {
		s.MaxWordLength = len(word)
	}

	// Guard against a nil Deletes value, should never happen
	if s.Deletes == nil {
		s.Deletes = make(map[int][]string, s.InitialCapacity)
	}

	// Generate edits/suggestions, only do this once no matter how many times this word is processed.

	// Create deletes
	edits := s.EditsPrefix(word)

	// if not staging suggestions, put directly into main data structure
	if stage != nil {
		for del := range edits.Iter() {
			stage.Add(utilities.GetStringHash(del, s.CompactMask), word)
		}
	} else {
		for del := range edits.Iter() {
			deleteHash := utilities.GetStringHash(del, s.CompactMask)
			if v, prs := s.Deletes[deleteHash]; prs {
				s.Deletes[deleteHash] = append(v, del)
			} else {
				s.Deletes[deleteHash] = []string{del}
			}
		}
	}
	return true, nil
}

// Edits creates a set of inexpensive and language independent edits
// only deletes, no transposes, replacements, or inserts
// replaces and inserts are expensive and language dependent (Chinese has 70,000 Unicode Han characters)
func (s *SymSpell) Edits(word string, editDistance int, deleteWords mapset.Set[string]) mapset.Set[string] {
	editDistance++
	if len(word) > 1 {
		for i := range len(word) {
			del := word[0:i] + word[i:]
			if deleteWords.Add(del) {
				// recursion, if maximum edit distance not yet reached
				if editDistance < s.MaxEditDistance {
					deleteWords = s.Edits(del, editDistance, deleteWords)
				}
			}
		}
	}

	return deleteWords
}

func (s *SymSpell) EditsPrefix(word string) mapset.Set[string] {
	edits := mapset.NewSet[string]()

	if len(word) <= s.MaxEditDistance {
		edits.Add("")
	}
	if len(word) > s.PrefixLength {
		word = word[0:s.PrefixLength]
	}

	edits.Add(word)

	return s.Edits(word, 0, edits)
}

func (s *SymSpell) LoadBigramDictionaryFile(corpus string) bool {
	if _, err := os.Stat(corpus); os.IsNotExist(err) {
		return false
	}
	osreader, err := os.Open(corpus)
	if err != nil {
		panic(err)
	}
	defer osreader.Close()

	return s.LoadBigramDictionary(osreader)
}

func (s *SymSpell) LoadBigramDictionary(corpus io.Reader) bool {
	scanner := bufio.NewScanner(corpus)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineParts := strings.Split(line, " ")

		key := lineParts[0]

		if len(lineParts) == 3 {
			key = strings.Join(lineParts[0:2], " ")
		}

		count, err := strconv.Atoi(lineParts[len(lineParts)-1])
		if err != nil {
			panic(err)
		}
		s.BigRams[key] = count
		if count < s.BigRamCountMin {
			s.BigRamCountMin = count
		}
	}

	return true
}

func (s *SymSpell) LoadDictionaryFile(corpus string) bool {
	if _, err := os.Stat(corpus); os.IsNotExist(err) {
		return false
	}
	osreader, err := os.Open(corpus)
	if err != nil {
		panic(err)
	}
	defer osreader.Close()

	return s.LoadDictionary(osreader)
}

func (s *SymSpell) LoadDictionary(corpus io.Reader) bool {
	//stage := staging.NewSuggestionStage[string](16384)

	scanner := bufio.NewScanner(corpus)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineParts := strings.Split(line, " ")

		key := lineParts[0]

		if len(lineParts) == 3 {
			key = strings.Join(lineParts[0:2], " ")
		}

		count, err := strconv.Atoi(lineParts[len(lineParts)-1])
		if err != nil {
			panic(err)
		}
		_, err = s.CreateEntry(key, count, nil)
		if err != nil {
			panic(err)
		}
	}

	//if s.Deletes == nil {
	//	s.Deletes = make(map[int][]string, stage.DeleteCount())
	//}
	//s.CommitStaged(stage)
	return true
}

func (s *SymSpell) CreateDictionaryFile(corpus string) bool {
	if _, err := os.Stat(corpus); os.IsNotExist(err) {
		return false
	}
	osreader, err := os.Open(corpus)
	if err != nil {
		panic(err)
	}
	defer osreader.Close()

	return s.CreateDictionary(osreader)
}

func (s *SymSpell) CreateDictionary(corpus io.Reader) bool {
	stage := staging.NewSuggestionStage[string](16384)

	scanner := bufio.NewScanner(corpus)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineParts := strings.Split(line, " ")

		for _, key := range lineParts {
			s.CreateEntry(key, 1, stage)
		}
	}

	if s.Deletes == nil {
		s.Deletes = make(map[int][]string, stage.DeleteCount())
	}
	s.CommitStaged(stage)
	return true
}

func (s *SymSpell) PurgeBelowThresholdWords() {
	s.BelowThresholdWords = make(map[string]int)
}

func (s *SymSpell) CommitStaged(staging *staging.Stage[string]) {
	staging.CommitTo(s.Deletes)
}

// DeleteInSuggestionPrefix checks whether all delete chars are present in the suggestion prefix in correct order,
// otherwise this is just a hash collision
func (s *SymSpell) DeleteInSuggestionPrefix(del string, delLen int, suggestion string, suggestionLen int) bool {
	if delLen == 0 {
		return true
	}
	if s.PrefixLength < suggestionLen {
		suggestionLen = s.PrefixLength
	}
	var j int = 0
	for i := range delLen {
		delChar := del[i]
		for j < suggestionLen && delChar != suggestion[j] {
			j++
		}
		if j == suggestionLen {
			return false
		}
	}
	return true
}

func (s *SymSpell) LookupDefault(input string, verbosity verb.Verbosity) (Suggestions, error) {
	return s.Lookup(input, verbosity, s.MaxEditDistance, false)
}

func (s *SymSpell) LookupEditDistance(input string, verbosity verb.Verbosity, maxEditDistance int) (Suggestions, error) {
	return s.Lookup(input, verbosity, maxEditDistance, false)
}

func (s *SymSpell) Lookup(input string, verbosity verb.Verbosity, maxEditDistance int, includeUnknown bool) (Suggestions, error) {
	// verbosity=Top: the suggestion with the highest term frequency of the suggestions of smallest edit distance found
	// verbosity=Closest: all suggestions of smallest edit distance found, the suggestions are ordered by term frequency
	// verbosity=All: all suggestions <= maxEditDistance, the suggestions are ordered by edit distance, then by term frequency (slower, no early termination)

	// maxEditDistance used in Lookup can't be bigger than the maxDictionaryEditDistance
	// used to construct the underlying dictionary structure.
	if maxEditDistance > s.MaxEditDistance {
		return nil, errors.New(fmt.Sprintf("argument out of range exception %d", maxEditDistance))
	}

	suggestions := make(Suggestions, 0)
	inputLen := len(input)
	// early exit - word is too big to possibly match any words
	if inputLen-maxEditDistance > s.MaxWordLength {
		return nil, errors.New("word is too long")
	}

	// quick look for exact match
	var suggestionCount int = 0
	if sugCount, prs := s.Words[input]; prs {
		suggestions = append(suggestions, NewSuggestion(input, 0, sugCount))
		// early exit - return exact match, unless caller wants all matches
		if verbosity != verb.All {
			if includeUnknown && len(suggestions) == 0 {
				suggestions = append(suggestions, NewSuggestion(input, s.MaxEditDistance+1, 0))
			}

			return suggestions, nil
		}
	}

	// early termination, if we only want to check if word in dictionary or get its frequency
	if maxEditDistance == 0 {
		if includeUnknown && len(suggestions) == 0 {
			suggestions = append(suggestions, NewSuggestion(input, s.MaxEditDistance+1, 0))
		}

		return suggestions, nil
	}

	// deletes we've considered already
	delSet := mapset.NewSet[string]()

	// suggestions we've considered already
	sugSet := mapset.NewSet[string]()

	// we considered the input already in the above if
	sugSet.Add(input)

	var maxEditDistance2 int = maxEditDistance
	var candidatePointer int = 0
	var candidates []string = make([]string, 0)

	// add original prefix
	var inputPrefixLen int = inputLen
	if inputPrefixLen > s.PrefixLength {
		inputPrefixLen = s.PrefixLength
		candidates = append(candidates, input[0:inputPrefixLen])
	} else {
		candidates = append(candidates, input)
	}

	for candidatePointer < len(candidates) {
		candidate := candidates[candidatePointer]
		candidatePointer++
		candidateLen := len(candidate)
		lengthDiff := inputPrefixLen - candidateLen

		// save some time - early termination
		// if canddate distance is already higher than suggestion distance, than there are no better suggestions to be expected
		if lengthDiff > maxEditDistance2 {
			// skip to next candidate if Verbosity.All, look no further if Verbosity.Top or Closest
			// (candidates are ordered by delete distance, so none are closer than current)
			if verbosity == verb.All {
				continue
			}

			break
		}

		// read candidate entry from dictionary
		if dictSuggestions, prs := s.Deletes[utilities.GetStringHash(candidate, s.CompactMask)]; prs {
			// iterate through suggestions (to other correct dictionary items) of delete item and add them to suggestion list
			for _, suggestion := range dictSuggestions {
				suggestionLen := len(suggestion)
				if suggestion == input {
					continue
				}

				if utilities.Abs(suggestionLen-inputLen) > maxEditDistance2 || // input and sugg lengths diff > allowed/current best distance
					suggestionLen < candidateLen || // sugg must be for a different delete string, in same bin only because of hash collision
					(suggestionLen == candidateLen && suggestion != candidate) { // if sugg len = delete len, then it either equals delete or is in same bin only because of hash collision
					continue
				}

				suggPrefixLen := utilities.Min(suggestionLen, s.PrefixLength)
				if suggPrefixLen > inputPrefixLen && (suggPrefixLen-candidateLen) > maxEditDistance2 {
					continue
				}

				// True Damerau-Levenshtein Edit Distance: adjust distance, if both distances>0
				// We allow simultaneous edits (deletes) of maxEditDistance on on both the dictionary and the input term.
				// For replaces and adjacent transposes the resulting edit distance stays <= maxEditDistance.
				// For inserts and deletes the resulting edit distance might exceed maxEditDistance.
				// To prevent suggestions of a higher edit distance, we need to calculate the resulting edit distance, if there are simultaneous edits on both sides.
				// Example: (bank==bnak and bank==bink, but bank!=kanb and bank!=xban and bank!=baxn for maxEditDistance=1)
				// Two deletes on each side of a pair makes them all equal, but the first two pairs have edit distance=1, the others edit distance=2.
				var distance int = 0
				var err error
				var minVal int = utilities.Min(inputLen, suggestionLen) - s.PrefixLength
				switch {
				case candidateLen == 0:
					// suggestions which have no common chars with input (inputLen<=maxEditDistance && suggestionLen<=maxEditDistance)
					distance = utilities.Max(inputLen, suggestionLen)
					// if (distance > maxEditDistance2 || !sugSet.Add(suggestion)) continue;
					if distance > maxEditDistance || !sugSet.Add(suggestion) {
						continue
					} else if suggestionLen == 1 {
						if strings.Index(input, string(suggestion[0])) < 0 {
							distance = inputLen
						} else {
							distance = inputLen - 1
						}
						if distance < maxEditDistance2 || !sugSet.Add(suggestion) {
							continue
						}
					}
				case (s.PrefixLength-s.MaxEditDistance == candidateLen) &&
					(minVal > 1 && (input[inputLen+1-minVal:] != suggestion[suggestionLen+1-minVal:])) ||
					(minVal > 0 && (input[inputLen-minVal] != suggestion[suggestionLen-minVal]) &&
						((input[inputLen-minVal-1] != suggestion[suggestionLen-minVal]) ||
							(input[inputLen-minVal] != suggestion[suggestionLen-minVal-1]))):
					// number of edits in prefix ==maxediddistance  AND no identic suffix
					// , then editdistance>maxEditDistance and no need for Levenshtein calculation
					// (inputLen >= prefixLength) && (suggestionLen >= prefixLength)
					continue
				default:
					// DeleteInSuggestionPrefix is somewhat expensive, and only pays off when verbosity is Top or Closest.
					if (verbosity != verb.All && !s.DeleteInSuggestionPrefix(candidate, candidateLen, suggestion, suggestionLen)) ||
						!sugSet.Add(suggestion) {
						continue
					}

					distance, err = Distance(input, suggestion, s.DistanceAlgorithm)
					if err != nil || distance < 0 {
						continue
					}
				}

				// save some time
				// do not process higher distances than those already found, if verbosity<All (note: maxEditDistance2 will always equal maxEditDistance when Verbosity.All)
				if distance <= maxEditDistance2 {
					suggestionCount = s.Words[suggestion]
					si := NewSuggestion(suggestion, distance, suggestionCount)
					if len(suggestions) > 0 {
						switch verbosity {
						case verb.Closest:
							// we will calculate DamLev distance only to the smallest found distance so far
							if distance < maxEditDistance2 {
								suggestions.Clear()
							}
						case verb.Top:
							if distance < maxEditDistance2 || suggestionCount > suggestions[0].Frequency {
								maxEditDistance2 = distance
								suggestions[0] = si
							}
							continue
						default:
						}
					}
					if verbosity != verb.All {
						maxEditDistance2 = distance
					}
					suggestions = append(suggestions, si)
				}
			}

			// add edits
			// derive edits (deletes) from candidate (input) and add them to candidates list
			// this is a recursive process until the maximum edit distance has been reached
			if lengthDiff < maxEditDistance && candidateLen <= s.PrefixLength {
				// save some time
				// do not create edits with edit distance smaller than suggestions already found
				if verbosity == verb.All && lengthDiff > maxEditDistance2 {
					continue
				}

				for i := range candidateLen {
					del := candidate[:i] + candidate[i+1:]
					if delSet.Add(del) {
						candidates = append(candidates, del)
					}
				}
			}
		}
	}

	// sort by ascending edit distance, then by descending word frequency
	if len(suggestions) > 1 {
		sort.Sort(suggestions)
	}

	if includeUnknown && len(suggestions) == 0 {
		suggestions = append(suggestions, NewSuggestion(input, s.MaxEditDistance+1, 0))
	}

	return suggestions, nil
}

// ParseWords creates a non-unique wordlist from sample text
// language independent (e.g. works with Chinese characters)
func (s *SymSpell) ParseWords(text string) []string {
	// \w Alphanumeric characters (including non-latin characters, umlaut characters and digits) plus "_"
	// \d Digits
	// Compatible with non-latin characters, does not split words at apostrophes
	re := regexp.MustCompile(`[\S]+`)

	// for benchmarking only: with CreateDictionary("big.txt", "") and the text corpus from http://norvig.com/big.txt
	// the Regex below provides the exact same number of dictionary items as Norvigs regex "[a-z]+" (which splits words
	// at apostrophes & incompatible with non-latin characters)
	// re := regexp.MustCompile(`[\w-[\d_]]+`)

	return re.FindAllString(strings.ToLower(text), -1)
}

func (s *SymSpell) LookupCompoundWithEditDistance(input string, editDistance int) *Suggestions {
	var err error
	// parse input string into single terms
	termList1 := s.ParseWords(input)

	suggestions := make(Suggestions, 0)     // suggestions for a single term
	suggestionParts := make(Suggestions, 0) // 1 line with separate parts

	// translate every term to its best suggestion, otherwise it remains unchanged
	lastCombi := false

	for i, term := range termList1 {
		suggestions, err = s.LookupEditDistance(term, verb.Top, editDistance)
		if err != nil {
			panic(err)
		}

		// combi check, always before split
		if i > 0 && !lastCombi {
			suggestionsCombi, err := s.LookupEditDistance(termList1[i-1]+term, verb.Top, editDistance)
			if err != nil {
				panic(err)
			}

			if len(suggestionsCombi) > 0 {
				best1 := suggestionParts[len(suggestionParts)-1]
				var best2 *Suggestion

				if len(suggestions) > 0 {
					best2 = suggestions[0]
				} else {
					best2 = NewSuggestion(term, editDistance+1, 10/(10^len(term)))
				}

				// distance1=edit distance between 2 split terms und their best corrections : als comparative value for the combination
				distance1 := best1.Distance + best2.Distance

				if distance1 >= 0 &&
					(suggestionsCombi[0].Distance+1 < distance1 ||
						(suggestionsCombi[0].Distance+1 == distance1 &&
							suggestionsCombi[0].Frequency > best1.Frequency/N*best2.Frequency)) {
					suggestionsCombi[0].Distance++
					suggestionParts[len(suggestionParts)-1] = suggestionsCombi[0]
					lastCombi = true
					continue
				}
			}
		}

		lastCombi = false

		// always split terms without suggestion / never split terms with suggestion ed=0 / never split single char terms
		if len(suggestions) > 0 && (suggestions[0].Distance == 0 || len(termList1[i]) == 1) {
			suggestionParts = append(suggestionParts, suggestions[0])
		} else {
			// if no perfect suggestion, split word into pairs
			var suggestionSplitBest *Suggestion

			// add original term
			if len(suggestions) > 0 {
				suggestionSplitBest = suggestions[0]
			}

			if len(termList1[i]) > 1 {
				for j := 1; j < len(termList1[i]); j++ {
					part1 := termList1[i][:j]
					part2 := termList1[i][j:]
					suggestionSplit := &Suggestion{}
					suggestions1, err := s.LookupEditDistance(part1, verb.Top, editDistance)
					if err != nil {
						panic(err)
					}
					if len(suggestions1) > 0 {
						suggestions2, err := s.LookupEditDistance(part2, verb.Top, editDistance)
						if err != nil {
							panic(err)
						}
						if len(suggestions2) > 0 {
							// select best suggestion for split pair
							suggestionSplit.Term = suggestions1[0].Term + " " + suggestions2[0].Term

							distance2, err := Distance(termList1[i], suggestionSplit.Term, s.DistanceAlgorithm)
							if err != nil {
								panic(err)
							}

							if suggestionSplitBest != nil {
								if distance2 > suggestionSplitBest.Distance {
									continue
								}
								if distance2 < suggestionSplitBest.Distance {
									suggestionSplitBest = nil
								}
							}

							suggestionSplit.Distance = distance2

							// if bigram exist in bigram dictionary
							if bigramCount, prs := s.BigRams[suggestionSplit.Term]; prs {
								suggestionSplit.Frequency = bigramCount

								// increase count, if split.corrections are part of or identical to input
								// single term correction exists
								if len(suggestions) > 0 {
									// alternatively remove the single term from suggestionsSplit, but then other splittings could win
									if suggestions1[0].Term+suggestions2[0].Term == termList1[i] {
										// make count bigger than count of single term correction
										suggestionSplit.Frequency = utilities.Max(suggestionSplit.Frequency, suggestions[0].Frequency+2)
									} else if suggestions1[0].Term == suggestions[0].Term || suggestions2[0].Term == suggestions[0].Term {
										// make count bigger than count of single term correction
										suggestionSplit.Frequency = utilities.Max(suggestionSplit.Frequency, suggestions[0].Frequency+1)
									}
								}
							} else {
								// The Naive Bayes probability of the word combination is the product of the two word probabilities: P(AB) = P(A) * P(B)
								// use it to estimate the frequency count of the combination, which then is used to rank/select the best splitting variant
								suggestionSplit.Frequency = utilities.Min(s.BigRamCountMin, suggestions1[0].Frequency/N*suggestions2[0].Frequency)
							}

							if suggestionSplitBest == nil || suggestionSplit.Frequency > suggestionSplitBest.Frequency {
								suggestionSplitBest = suggestionSplit
							}
						}
					}
				}

				if suggestionSplitBest != nil {
					// select best suggestion for split pair
					suggestionParts = append(suggestionParts, suggestionSplitBest)
				} else {
					si := NewSuggestion(termList1[i], editDistance+1, 10/(10^len(termList1[i])))
					suggestionParts = append(suggestionParts, si)
				}
			} else {
				si := NewSuggestion(termList1[i], editDistance+1, 10/(10^len(termList1[i])))
				suggestionParts = append(suggestionParts, si)
			}
		}
	}

	suggestion := &Suggestion{}

	count := N
	var sb strings.Builder
	for _, si := range suggestionParts {
		sb.WriteString(si.Term + " ")
		count *= si.Frequency / N
	}

	suggestion.Frequency = count
	suggestion.Term = strings.Trim(sb.String(), " ")

	suggestion.Distance, err = Distance(input, suggestion.Term, s.DistanceAlgorithm)
	if err != nil {
		panic(err)
	}

	suggestionsLine := &Suggestions{suggestion}

	return suggestionsLine
}

// LookupCompound supports compound aware automatic spelling correction of multi-word input strings with three cases:
// 1. mistakenly inserted space into a correct word led to two incorrect terms
// 2. mistakenly omitted space between two correct words led to one incorrect combined term
// 3. multiple independent input terms with/without spelling errors
func (s *SymSpell) LookupCompound(input string) *Suggestions {
	return s.LookupCompoundWithEditDistance(input, s.MaxEditDistance)
}
